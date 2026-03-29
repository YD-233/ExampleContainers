package main

import (
	"runtime"
	"strings"
)

var BuildInteractiveShell = ""
var BuildWebsocketCallbackHost = ""
var BuildWebsocketCallbackPort = ""
var BuildWebsocketAESPSK = ""

type agentRuntimeConfig struct {
	InteractiveEnabled bool   `json:"interactive_enabled"`
	SocksEnabled       bool   `json:"socks_enabled"`
	RpfwdEnabled       bool   `json:"rpfwd_enabled"`
	SysinfoEnabled     bool   `json:"sysinfo_enabled"`
	ProcessListEnabled bool   `json:"process_list_enabled"`
	AVScanEnabled      bool   `json:"avscan_enabled"`
	InteractiveShell   string `json:"interactive_shell"`
}

var runtimeConfig = agentRuntimeConfig{}

func defaultLegacyCapabilities() agentRuntimeConfig {
	config := agentRuntimeConfig{
		InteractiveEnabled: true,
		SocksEnabled:       false,
		RpfwdEnabled:       false,
		SysinfoEnabled:     true,
		ProcessListEnabled: true,
		AVScanEnabled:      runtime.GOOS == "windows",
		InteractiveShell:   strings.TrimSpace(BuildInteractiveShell),
	}
	if config.InteractiveShell == "" {
		config.InteractiveShell = defaultInteractiveShell()
	}
	return config
}

func defaultInteractiveShell() string {
	if runtime.GOOS == "windows" {
		return "cmd.exe"
	}
	return "/bin/sh"
}

func parseBoolWithDefault(value string, defaultValue bool) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return defaultValue
	}
}
