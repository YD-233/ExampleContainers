package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	c2structs "github.com/MythicMeta/MythicContainer/c2_structs"
)

// RuntimeConfig 同时承载服务端运行配置和与 Agent 共享的伪装参数。
// config check 会用 payload 的 C2 参数覆盖共享字段，从而保证服务端与 Agent 保持一致。
type RuntimeConfig struct {
	BindPort         int               `json:"bind_port"`
	UseSSL           bool              `json:"use_ssl"`
	CertFile         string            `json:"cert_file"`
	KeyFile          string            `json:"key_file"`
	DecoyStatusCode  int               `json:"decoy_status_code"`
	DecoyContentType string            `json:"decoy_content_type"`
	DecoyBody        string            `json:"decoy_body"`
	CallbackHost     string            `json:"callback_host"`
	CallbackPort     string            `json:"callback_port"`
	PostPath         string            `json:"post_path"`
	WebsocketPath    string            `json:"websocket_path"`
	UserAgent        string            `json:"user_agent"`
	HostHeader       string            `json:"host_header"`
	ExtraHeaders     map[string]string `json:"extra_headers"`
	QueryString      string            `json:"query_string"`
	AESPSK           string            `json:"aes_psk,omitempty"`
}

func DefaultConfig() RuntimeConfig {
	return normalizeConfig(RuntimeConfig{
		BindPort:         8443,
		UseSSL:           true,
		CertFile:         "",
		KeyFile:          "",
		DecoyStatusCode:  200,
		DecoyContentType: "text/html; charset=utf-8",
		DecoyBody:        "<html><head><title>masked_https</title></head><body><h1>It works</h1></body></html>",
		CallbackHost:     "https://127.0.0.1",
		CallbackPort:     "8443",
		PostPath:         "/cdn-cgi/submit",
		WebsocketPath:    "/cdn-cgi/ws",
		UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		HostHeader:       "",
		ExtraHeaders: map[string]string{
			"Accept":          "*/*",
			"Accept-Language": "zh-CN,zh;q=0.9",
		},
		QueryString: "",
		AESPSK:      "",
	})
}

func LoadConfig(path string) (RuntimeConfig, error) {
	config := DefaultConfig()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return config, err
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return config, err
	}
	return normalizeConfig(config), nil
}

func SaveConfig(path string, config RuntimeConfig) error {
	config = normalizeConfig(config)
	raw, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0644)
}

func ApplyC2Parameters(config RuntimeConfig, params c2structs.C2Parameters) (RuntimeConfig, error) {
	config = normalizeConfig(config)

	callbackHost, err := params.GetStringArg("callback_host")
	if err == nil && strings.TrimSpace(callbackHost) != "" {
		config.CallbackHost = strings.TrimSpace(callbackHost)
	}

	if callbackPort, err := params.GetArg("callback_port"); err == nil {
		text := strings.TrimSpace(fmt.Sprintf("%v", callbackPort))
		if text != "" && text != "<nil>" {
			config.CallbackPort = text
		}
	}

	if postPath, err := params.GetStringArg("post_path"); err == nil && strings.TrimSpace(postPath) != "" {
		config.PostPath = strings.TrimSpace(postPath)
	}
	if websocketPath, err := params.GetStringArg("websocket_path"); err == nil && strings.TrimSpace(websocketPath) != "" {
		config.WebsocketPath = strings.TrimSpace(websocketPath)
	}
	if userAgent, err := params.GetStringArg("user_agent"); err == nil {
		config.UserAgent = strings.TrimSpace(userAgent)
	}
	if hostHeader, err := params.GetStringArg("host_header"); err == nil {
		config.HostHeader = strings.TrimSpace(hostHeader)
	}
	if queryString, err := params.GetStringArg("query_string"); err == nil {
		config.QueryString = strings.TrimSpace(strings.TrimPrefix(queryString, "?"))
	}
	if aesArg, err := params.GetArg("AESPSK"); err == nil {
		config.AESPSK = extractAESPSKValue(aesArg)
	}
	if extraHeaders, err := params.GetDictionaryArg("extra_headers"); err == nil && len(extraHeaders) > 0 {
		config.ExtraHeaders = extraHeaders
	}

	return normalizeConfig(config), nil
}

func ConfigPath(baseDir string) string {
	return filepath.Join(baseDir, "config.json")
}

func normalizeConfig(config RuntimeConfig) RuntimeConfig {
	if config.BindPort <= 0 {
		config.BindPort = 8443
	}
	if config.DecoyStatusCode <= 0 {
		config.DecoyStatusCode = 200
	}
	if strings.TrimSpace(config.DecoyContentType) == "" {
		config.DecoyContentType = "text/html; charset=utf-8"
	}
	if strings.TrimSpace(config.DecoyBody) == "" {
		config.DecoyBody = "<html><body><h1>It works</h1></body></html>"
	}
	if strings.TrimSpace(config.CallbackHost) == "" {
		config.CallbackHost = "https://127.0.0.1"
	}
	if strings.TrimSpace(config.CallbackPort) == "" {
		config.CallbackPort = fmt.Sprintf("%d", config.BindPort)
	}
	config.PostPath = normalizePath(config.PostPath, "/cdn-cgi/submit")
	config.WebsocketPath = normalizePath(config.WebsocketPath, "/cdn-cgi/ws")
	config.QueryString = strings.TrimSpace(strings.TrimPrefix(config.QueryString, "?"))
	config.UserAgent = strings.TrimSpace(config.UserAgent)
	if config.ExtraHeaders == nil {
		config.ExtraHeaders = map[string]string{
			"Accept":          "*/*",
			"Accept-Language": "zh-CN,zh;q=0.9",
		}
	}
	config.HostHeader = strings.TrimSpace(config.HostHeader)
	config.CertFile = strings.TrimSpace(config.CertFile)
	config.KeyFile = strings.TrimSpace(config.KeyFile)
	return config
}

func normalizePath(path string, fallback string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fallback
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}
	return trimmed
}

func extractAESPSKValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if strings.EqualFold(trimmed, "none") {
			return ""
		}
		return trimmed
	case map[string]interface{}:
		if encKey, ok := typed["enc_key"].(string); ok {
			return strings.TrimSpace(encKey)
		}
	}
	return ""
}
