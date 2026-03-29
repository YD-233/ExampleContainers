package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

var BuildEmbeddedConfigB64 = ""
var BuildEnableSessionMode = ""

type CallbackRole string

const (
	CallbackRoleBeacon  CallbackRole = "beacon"
	CallbackRoleSession CallbackRole = "session"
)

type EmbeddedConfig struct {
	PayloadUUID         string                     `json:"payload_uuid"`
	InsecureSkipVerify  bool                       `json:"insecure_skip_verify"`
	EnableSessionMode   bool                       `json:"enable_session_mode"`
	PollIntervalSeconds int                        `json:"poll_interval_seconds"`
	Profiles            map[string]EmbeddedProfile `json:"profiles"`
	Capabilities        agentRuntimeConfig         `json:"capabilities"`
}

// EmbeddedProfile 表示一个被 builder 固化进二进制的 C2 profile 视图。
// v2 以后如果要接更多 profile，继续往 Profiles map 中扩展即可。
type EmbeddedProfile struct {
	Name         string                 `json:"name"`
	CallbackHost string                 `json:"callback_host"`
	CallbackPort string                 `json:"callback_port"`
	AESPSK       string                 `json:"aes_psk"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
}

func loadEmbeddedConfig() (EmbeddedConfig, error) {
	if strings.TrimSpace(BuildEmbeddedConfigB64) != "" {
		// 新版本优先读取统一嵌入配置，避免大量散落的 linker 变量难以维护。
		return decodeEmbeddedConfig(BuildEmbeddedConfigB64)
	}
	// 向后兼容旧的本地调试环境变量。
	return buildEmbeddedConfigFromLegacyInputs(), nil
}

func decodeEmbeddedConfig(raw string) (EmbeddedConfig, error) {
	config := EmbeddedConfig{}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return config, fmt.Errorf("解码嵌入配置失败: %w", err)
	}
	if err := json.Unmarshal(decoded, &config); err != nil {
		return config, fmt.Errorf("解析嵌入配置失败: %w", err)
	}
	if config.Profiles == nil {
		config.Profiles = make(map[string]EmbeddedProfile)
	}
	return normalizeEmbeddedConfig(config), nil
}

func buildEmbeddedConfigFromLegacyInputs() EmbeddedConfig {
	config := EmbeddedConfig{
		PayloadUUID:         strings.TrimSpace(BuildPayloadUUID),
		InsecureSkipVerify:  parseBoolWithDefault(BuildInsecureSkipVerify, false),
		EnableSessionMode:   parseBoolWithDefault(BuildEnableSessionMode, true),
		PollIntervalSeconds: parsePollIntervalValue(taskPollWait),
		Profiles:            make(map[string]EmbeddedProfile),
		Capabilities:        defaultLegacyCapabilities(),
	}

	config.Profiles["http"] = EmbeddedProfile{
		Name:         "http",
		CallbackHost: strings.TrimSpace(BuildCallbackHost),
		CallbackPort: strings.TrimSpace(BuildCallbackPort),
		AESPSK:       strings.TrimSpace(BuildAESPSK),
		Parameters:   map[string]interface{}{},
	}

	sessionHost := strings.TrimSpace(BuildWebsocketCallbackHost)
	if sessionHost == "" {
		sessionHost = strings.TrimSpace(BuildCallbackHost)
	}
	sessionPort := strings.TrimSpace(BuildWebsocketCallbackPort)
	if sessionPort == "" {
		sessionPort = strings.TrimSpace(BuildCallbackPort)
	}
	sessionAES := strings.TrimSpace(BuildWebsocketAESPSK)
	if sessionAES == "" {
		sessionAES = strings.TrimSpace(BuildAESPSK)
	}
	config.Profiles["websocket"] = EmbeddedProfile{
		Name:         "websocket",
		CallbackHost: sessionHost,
		CallbackPort: sessionPort,
		AESPSK:       sessionAES,
		Parameters:   map[string]interface{}{},
	}

	return normalizeEmbeddedConfig(config)
}

func normalizeEmbeddedConfig(config EmbeddedConfig) EmbeddedConfig {
	if config.Profiles == nil {
		config.Profiles = make(map[string]EmbeddedProfile)
	}
	config.PayloadUUID = strings.TrimSpace(config.PayloadUUID)
	config.Capabilities.InteractiveShell = strings.TrimSpace(config.Capabilities.InteractiveShell)
	if config.Capabilities.InteractiveShell == "" {
		if defaultShell := defaultInteractiveShell(); defaultShell != "" {
			config.Capabilities.InteractiveShell = defaultShell
		}
	}
	if config.PollIntervalSeconds < 1 {
		config.PollIntervalSeconds = 5
	}

	for key, profile := range config.Profiles {
		profile.Name = strings.TrimSpace(profile.Name)
		if profile.Name == "" {
			profile.Name = key
		}
		profile.CallbackHost = strings.TrimSpace(profile.CallbackHost)
		profile.CallbackPort = strings.TrimSpace(profile.CallbackPort)
		profile.AESPSK = strings.TrimSpace(profile.AESPSK)
		if profile.Parameters == nil {
			profile.Parameters = make(map[string]interface{})
		}
		config.Profiles[key] = profile
	}
	return config
}

func parsePollIntervalValue(defaultInterval interface{}) int {
	switch value := defaultInterval.(type) {
	case int:
		if value > 0 {
			return value
		}
	case int64:
		if value > 0 {
			return int(value)
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 5
}
