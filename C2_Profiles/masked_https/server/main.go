package main

import (
	"MaskedHTTPS/common"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type websocketEnvelope struct {
	Client bool   `json:"client"`
	Data   string `json:"data"`
	Tag    string `json:"tag,omitempty"`
}

func main() {
	baseDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("获取当前目录失败: %v", err)
	}
	configPath := common.ConfigPath(baseDir)
	config, err := common.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("读取配置失败: %v", err)
	}
	config, err = common.EnsureTLSFiles(baseDir, config)
	if err != nil {
		log.Fatalf("准备 TLS 证书失败: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(config.PostPath, func(w http.ResponseWriter, r *http.Request) {
		handleBeacon(w, r, config)
	})
	mux.HandleFunc(config.WebsocketPath, func(w http.ResponseWriter, r *http.Request) {
		handleWebsocket(w, r, config)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeDecoyResponse(w, config)
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", config.BindPort),
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
	}

	log.Printf("masked_https_server 启动: bind=:%d ssl=%t post_path=%s websocket_path=%s", config.BindPort, config.UseSSL, config.PostPath, config.WebsocketPath)
	if config.UseSSL {
		log.Fatal(server.ListenAndServeTLS(config.CertFile, config.KeyFile))
	}
	log.Fatal(server.ListenAndServe())
}

func handleBeacon(w http.ResponseWriter, r *http.Request, config common.RuntimeConfig) {
	if r.Method != http.MethodPost {
		writeDecoyResponse(w, config)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	responseBody, statusCode, contentType, err := forwardToMythic(body)
	if err != nil {
		http.Error(w, "failed to forward", http.StatusBadGateway)
		return
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "text/plain")
	}
	w.WriteHeader(statusCode)
	_, _ = w.Write(responseBody)
}

func handleWebsocket(w http.ResponseWriter, r *http.Request, config common.RuntimeConfig) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		var envelope websocketEnvelope
		if err := conn.ReadJSON(&envelope); err != nil {
			return
		}
		if strings.TrimSpace(envelope.Data) == "" {
			_ = conn.WriteJSON(websocketEnvelope{Client: false, Data: ""})
			continue
		}
		responseBody, _, _, err := forwardToMythic([]byte(envelope.Data))
		if err != nil {
			return
		}
		if err := conn.WriteJSON(websocketEnvelope{Client: false, Data: string(responseBody), Tag: envelope.Tag}); err != nil {
			return
		}
	}
}

func writeDecoyResponse(w http.ResponseWriter, config common.RuntimeConfig) {
	w.Header().Set("Content-Type", config.DecoyContentType)
	w.WriteHeader(config.DecoyStatusCode)
	_, _ = io.WriteString(w, config.DecoyBody)
}

func forwardToMythic(body []byte) ([]byte, int, string, error) {
	targetURL, err := mythicAgentMessageURL()
	if err != nil {
		return nil, 0, "", err
	}
	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Mythic", "masked_https")

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, "", err
	}
	return responseBody, resp.StatusCode, resp.Header.Get("Content-Type"), nil
}

func mythicAgentMessageURL() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("MYTHIC_ADDRESS")); explicit != "" {
		return appendAgentMessagePath(explicit)
	}
	host := strings.TrimSpace(os.Getenv("MYTHIC_SERVER_HOST"))
	port := strings.TrimSpace(os.Getenv("MYTHIC_SERVER_PORT"))
	scheme := strings.TrimSpace(os.Getenv("MYTHIC_SERVER_SCHEME"))
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "17443"
	}
	if scheme == "" {
		// Mythic 的 17443 在本地开发和容器内部默认都是明文 HTTP，
		// 只有显式指定时才切到 HTTPS，避免这里把上游协议写死导致 502。
		scheme = "http"
	}
	return appendAgentMessagePath(fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(host, port)))
}

func appendAgentMessagePath(base string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/agent_message"
	}
	if !strings.HasSuffix(parsed.Path, "/agent_message") {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/agent_message"
	}
	return parsed.String(), nil
}

func init() {
	// 确保以 server 目录为工作目录时能找到本地文件。
	if exePath, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exePath))
	}
}
