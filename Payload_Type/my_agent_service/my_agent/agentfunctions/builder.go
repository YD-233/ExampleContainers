package agentfunctions

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"github.com/MythicMeta/MythicContainer/mythicrpc"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

var payloadDefinition = agentstructs.PayloadType{
	Name:                                   "my_agent",
	FileExtension:                          "bin",
	Author:                                 "@zhujiayi",
	SupportedOS:                            []string{agentstructs.SUPPORTED_OS_LINUX, agentstructs.SUPPORTED_OS_MACOS, agentstructs.SUPPORTED_OS_WINDOWS},
	Wrapper:                                false,
	CanBeWrappedByTheFollowingPayloadTypes: []string{},
	SupportsDynamicLoading:                 true,
	Description:                            "最小可跑的 Go Agent 模板（用于 Mythic 二次开发）",
	SupportedC2Profiles:                    []string{"masked_https"},
	MythicEncryptsData:                     true,
	MessageFormat:                          agentstructs.MessageFormatJSON,
	BuildParameters: []agentstructs.BuildParameter{
		{
			Name:          "architecture",
			Description:   "选择目标架构",
			Required:      false,
			DefaultValue:  "amd64",
			Choices:       []string{"amd64", "arm64"},
			ParameterType: agentstructs.BUILD_PARAMETER_TYPE_CHOOSE_ONE,
		},
		{
			Name:          "interactive_shell",
			Description:   "交互式 shell 默认路径，Windows 可用 cmd.exe 或 powershell.exe",
			Required:      false,
			DefaultValue:  "",
			ParameterType: agentstructs.BUILD_PARAMETER_TYPE_STRING,
		},
		{
			Name:          "insecure_skip_verify",
			Description:   "是否跳过 HTTPS 证书校验，仅建议本地调试使用",
			Required:      false,
			DefaultValue:  false,
			ParameterType: agentstructs.BUILD_PARAMETER_TYPE_BOOLEAN,
		},
	},
	BuildSteps: []agentstructs.BuildStep{
		{
			Name:        "Configure",
			Description: "整理构建参数并生成编译命令",
		},
		{
			Name:        "Compile",
			Description: "编译 my_agent/agent_code",
		},
		{
			Name:        "Finalize",
			Description: "读取构建产物并回传给 Mythic",
		},
	},
}

// build 负责把 Mythic UI 里的构建参数收敛成一个统一的嵌入配置，
// 再通过 linker flags 打进 Agent，避免运行时依赖一长串散落变量。
func build(payloadBuildMsg agentstructs.PayloadBuildMessage) agentstructs.PayloadBuildResponse {
	commandList := append([]string(nil), payloadBuildMsg.CommandList...)
	response := agentstructs.PayloadBuildResponse{
		PayloadUUID:        payloadBuildMsg.PayloadUUID,
		Success:            true,
		UpdatedCommandList: &commandList,
	}

	if len(payloadBuildMsg.C2Profiles) == 0 {
		response.Success = false
		response.BuildStdErr = "必须选择 masked_https C2 Profile"
		return response
	}

	goos := "linux"
	switch strings.ToLower(payloadBuildMsg.SelectedOS) {
	case "macos":
		goos = "darwin"
	case "windows":
		goos = "windows"
	}

	goarch, err := payloadBuildMsg.BuildParameters.GetStringArg("architecture")
	if err != nil {
		response.Success = false
		response.BuildStdErr = err.Error()
		return response
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	interactiveShell, err := payloadBuildMsg.BuildParameters.GetStringArg("interactive_shell")
	if err != nil {
		interactiveShell = ""
	}
	insecureSkipVerify, err := payloadBuildMsg.BuildParameters.GetBooleanArg("insecure_skip_verify")
	if err != nil {
		insecureSkipVerify = false
	}
	commandList = filterCommandList(payloadBuildMsg.CommandList, goos)
	capabilities := deriveCapabilitiesFromCommandList(commandList, goos)
	enableSessionMode := capabilities.sessionModeEnabled
	response.UpdatedCommandList = &commandList

	maskedProfile, exists := findC2Profile(payloadBuildMsg.C2Profiles, "masked_https")
	if !exists {
		response.Success = false
		response.BuildStdErr = "必须选择 masked_https C2 Profile"
		return response
	}
	httpEmbedded, websocketEmbedded, err := buildMaskedHTTPSProfiles(maskedProfile)
	if err != nil {
		response.Success = false
		response.BuildStdErr = err.Error()
		return response
	}
	if enableSessionMode && strings.TrimSpace(websocketEmbedded.Parameters["websocket_path"].(string)) == "" {
		response.Success = false
		response.BuildStdErr = "启用 session mode 时 websocket_path 不能为空"
		return response
	}

	embeddedConfigB64, err := buildEmbeddedConfigB64(embeddedBuildConfig{
		PayloadUUID:        payloadBuildMsg.PayloadUUID,
		InsecureSkipVerify: insecureSkipVerify,
		EnableSessionMode:  enableSessionMode,
		PollInterval:       5,
		Capabilities: embeddedBuildCapabilities{
			InteractiveEnabled: capabilities.interactiveEnabled,
			SocksEnabled:       capabilities.socksEnabled,
			RpfwdEnabled:       capabilities.rpfwdEnabled,
			SysinfoEnabled:     capabilities.sysinfoEnabled,
			ProcessListEnabled: capabilities.processListEnabled,
			AVScanEnabled:      capabilities.avscanEnabled,
			InteractiveShell:   interactiveShell,
		},
		Profiles: map[string]embeddedBuildProfile{
			"http":      httpEmbedded,
			"websocket": websocketEmbedded,
		},
	})
	if err != nil {
		response.Success = false
		response.BuildStdErr = "生成嵌入配置失败: " + err.Error()
		return response
	}

	outputName := fmt.Sprintf("%s-%s-%s.bin", payloadBuildMsg.PayloadUUID, goos, goarch)
	outputPath := filepath.Join(os.TempDir(), outputName)
	defer os.Remove(outputPath)
	ldflags := buildLdflags(map[string]string{
		"main.BuildPayloadUUID":           payloadBuildMsg.PayloadUUID,
		"main.BuildCallbackHost":          httpEmbedded.CallbackHost,
		"main.BuildCallbackPort":          httpEmbedded.CallbackPort,
		"main.BuildAESPSK":                httpEmbedded.AESPSK,
		"main.BuildEnableSessionMode":     fmt.Sprintf("%t", enableSessionMode),
		"main.BuildInteractiveShell":      interactiveShell,
		"main.BuildInsecureSkipVerify":    fmt.Sprintf("%t", insecureSkipVerify),
		"main.BuildWebsocketCallbackHost": websocketEmbedded.CallbackHost,
		"main.BuildWebsocketCallbackPort": websocketEmbedded.CallbackPort,
		"main.BuildWebsocketAESPSK":       websocketEmbedded.AESPSK,
		"main.BuildEmbeddedConfigB64":     embeddedConfigB64,
	})
	args := []string{
		"build",
		"-trimpath",
		"-ldflags",
		ldflags,
		"-o",
		outputPath,
		"./my_agent/agent_code",
	}

	mythicrpc.SendMythicRPCPayloadUpdateBuildStep(mythicrpc.MythicRPCPayloadUpdateBuildStepMessage{
		PayloadUUID: payloadBuildMsg.PayloadUUID,
		StepName:    "Configure",
		StepSuccess: true,
		StepStdout:  fmt.Sprintf("GOOS=%s GOARCH=%s http=%s:%s websocket=%s:%s output=%s session_mode=%t interactive=%t socks=%t rpfwd=%t sysinfo=%t ps=%t avscan=%t shell=%s insecure_skip_verify=%t commands=%v", goos, goarch, httpEmbedded.CallbackHost, httpEmbedded.CallbackPort, websocketEmbedded.CallbackHost, websocketEmbedded.CallbackPort, outputPath, enableSessionMode, capabilities.interactiveEnabled, capabilities.socksEnabled, capabilities.rpfwdEnabled, capabilities.sysinfoEnabled, capabilities.processListEnabled, capabilities.avscanEnabled, strings.TrimSpace(interactiveShell), insecureSkipVerify, commandList),
	})

	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		response.Success = false
		response.BuildMessage = "编译失败"
		response.BuildStdOut = stdout.String()
		response.BuildStdErr = stderr.String() + "\n" + err.Error()
		mythicrpc.SendMythicRPCPayloadUpdateBuildStep(mythicrpc.MythicRPCPayloadUpdateBuildStepMessage{
			PayloadUUID: payloadBuildMsg.PayloadUUID,
			StepName:    "Compile",
			StepSuccess: false,
			StepStdout:  response.BuildStdErr,
		})
		return response
	}

	mythicrpc.SendMythicRPCPayloadUpdateBuildStep(mythicrpc.MythicRPCPayloadUpdateBuildStepMessage{
		PayloadUUID: payloadBuildMsg.PayloadUUID,
		StepName:    "Compile",
		StepSuccess: true,
		StepStdout:  stdout.String(),
	})

	payloadBytes, err := os.ReadFile(outputPath)
	if err != nil {
		response.Success = false
		response.BuildMessage = "构建成功但读取产物失败"
		response.BuildStdOut = stdout.String()
		response.BuildStdErr = err.Error()
		mythicrpc.SendMythicRPCPayloadUpdateBuildStep(mythicrpc.MythicRPCPayloadUpdateBuildStepMessage{
			PayloadUUID: payloadBuildMsg.PayloadUUID,
			StepName:    "Finalize",
			StepSuccess: false,
			StepStdout:  response.BuildStdErr,
		})
		return response
	}

	response.Payload = &payloadBytes
	response.BuildStdOut = stdout.String()
	response.BuildStdErr = stderr.String()
	response.BuildMessage = "构建成功"
	response.Success = true
	mythicrpc.SendMythicRPCPayloadUpdateBuildStep(mythicrpc.MythicRPCPayloadUpdateBuildStepMessage{
		PayloadUUID: payloadBuildMsg.PayloadUUID,
		StepName:    "Finalize",
		StepSuccess: true,
		StepStdout:  fmt.Sprintf("payload bytes: %d", len(payloadBytes)),
	})
	return response
}

func extractAESPSKValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		if strings.EqualFold(strings.TrimSpace(typed), "none") {
			return ""
		}
		return strings.TrimSpace(typed)
	case map[string]interface{}:
		if encKey, ok := typed["enc_key"].(string); ok {
			return strings.TrimSpace(encKey)
		}
	}
	return ""
}

func filterCommandList(input []string, goos string) []string {
	filtered := make([]string, 0, len(input))
	for _, command := range input {
		switch command {
		case "avscan":
			if goos == "windows" {
				filtered = append(filtered, command)
			}
		default:
			filtered = append(filtered, command)
		}
	}
	return filtered
}

type derivedCapabilities struct {
	sessionModeEnabled bool
	interactiveEnabled bool
	socksEnabled       bool
	rpfwdEnabled       bool
	sysinfoEnabled     bool
	processListEnabled bool
	avscanEnabled      bool
}

// 根据 UI 中 “Select Commands to Include in the Payload” 的结果推导能力。
// 这样代理、信息收集等模块是否编入 payload，完全由命令选择决定。
func deriveCapabilitiesFromCommandList(commandList []string, goos string) derivedCapabilities {
	commandSet := make(map[string]struct{}, len(commandList))
	for _, command := range commandList {
		commandSet[command] = struct{}{}
	}
	capabilities := derivedCapabilities{
		interactiveEnabled: hasAnyCommand(commandSet, "pty"),
		socksEnabled:       hasAnyCommand(commandSet, "socks", "socks_stop"),
		rpfwdEnabled:       hasAnyCommand(commandSet, "rpfwd", "rpfwd_stop"),
		sysinfoEnabled:     hasAnyCommand(commandSet, "sysinfo"),
		processListEnabled: hasAnyCommand(commandSet, "ps"),
		avscanEnabled:      goos == "windows" && hasAnyCommand(commandSet, "avscan"),
	}
	capabilities.sessionModeEnabled = hasAnyCommand(commandSet,
		"session_start", "session_stop", "session_status",
		"pty", "socks", "socks_stop", "rpfwd", "rpfwd_stop",
	)
	return capabilities
}

func hasAnyCommand(commandSet map[string]struct{}, names ...string) bool {
	for _, name := range names {
		if _, ok := commandSet[name]; ok {
			return true
		}
	}
	return false
}

func buildLdflags(values map[string]string) string {
	flags := []string{"-s", "-w"}
	order := []string{
		"main.BuildEmbeddedConfigB64",
		"main.BuildPayloadUUID",
		"main.BuildCallbackHost",
		"main.BuildCallbackPort",
		"main.BuildAESPSK",
		"main.BuildEnableSessionMode",
		"main.BuildInteractiveShell",
		"main.BuildInsecureSkipVerify",
		"main.BuildWebsocketCallbackHost",
		"main.BuildWebsocketCallbackPort",
		"main.BuildWebsocketAESPSK",
	}
	for _, key := range order {
		flags = append(flags, fmt.Sprintf("-X %s", quoteLdflagAssignment(key, values[key])))
	}
	return strings.Join(flags, " ")
}

func quoteLdflagAssignment(name string, value string) string {
	escaped := strings.ReplaceAll(value, `'`, `'\''`)
	return fmt.Sprintf("'%s=%s'", name, escaped)
}

func Initialize() {
	agentstructs.AllPayloadData.Get("my_agent").AddPayloadDefinition(payloadDefinition)
	agentstructs.AllPayloadData.Get("my_agent").AddBuildFunction(build)
	agentstructs.AllPayloadData.Get("my_agent").AddOnNewCallbackFunction(onNewCallback)
	logRegisteredCommands()
}

// logRegisteredCommands 启动时打印运行时实际注册到容器里的命令列表，
// 方便定位“数据库里有命令，但容器运行时找不到命令”的问题。
func logRegisteredCommands() {
	commands := agentstructs.AllPayloadData.Get("my_agent").GetCommands()
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		names = append(names, command.Name)
	}
	sort.Strings(names)
	fmt.Printf("[my_agent] 运行时注册命令: %s\n", strings.Join(names, ", "))
}

type embeddedBuildConfig struct {
	PayloadUUID        string                          `json:"payload_uuid"`
	InsecureSkipVerify bool                            `json:"insecure_skip_verify"`
	EnableSessionMode  bool                            `json:"enable_session_mode"`
	PollInterval       int                             `json:"poll_interval_seconds"`
	Profiles           map[string]embeddedBuildProfile `json:"profiles"`
	Capabilities       embeddedBuildCapabilities       `json:"capabilities"`
}

type embeddedBuildCapabilities struct {
	InteractiveEnabled bool   `json:"interactive_enabled"`
	SocksEnabled       bool   `json:"socks_enabled"`
	RpfwdEnabled       bool   `json:"rpfwd_enabled"`
	SysinfoEnabled     bool   `json:"sysinfo_enabled"`
	ProcessListEnabled bool   `json:"process_list_enabled"`
	AVScanEnabled      bool   `json:"avscan_enabled"`
	InteractiveShell   string `json:"interactive_shell"`
}

type embeddedBuildProfile struct {
	Name         string                 `json:"name"`
	CallbackHost string                 `json:"callback_host"`
	CallbackPort string                 `json:"callback_port"`
	AESPSK       string                 `json:"aes_psk"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
}

func findC2Profile(profiles []agentstructs.PayloadBuildC2Profile, name string) (agentstructs.PayloadBuildC2Profile, bool) {
	for _, profile := range profiles {
		if strings.EqualFold(strings.TrimSpace(profile.Name), name) {
			return profile, true
		}
	}
	return agentstructs.PayloadBuildC2Profile{}, false
}

func buildEmbeddedProfile(name string, profile agentstructs.PayloadBuildC2Profile) (embeddedBuildProfile, error) {
	callbackHost, _ := profile.GetStringArg("callback_host")
	callbackPortValue, _ := profile.GetArg("callback_port")
	aespskValue, _ := profile.GetArg("AESPSK")
	return embeddedBuildProfile{
		Name:         name,
		CallbackHost: strings.TrimSpace(callbackHost),
		CallbackPort: stringifyOptionalValue(callbackPortValue),
		AESPSK:       extractAESPSKValue(aespskValue),
		Parameters:   profile.Parameters,
	}, nil
}

func buildMaskedHTTPSProfiles(profile agentstructs.PayloadBuildC2Profile) (embeddedBuildProfile, embeddedBuildProfile, error) {
	callbackHost, _ := profile.GetStringArg("callback_host")
	callbackPortValue, _ := profile.GetArg("callback_port")
	aespskValue, _ := profile.GetArg("AESPSK")
	postPath, _ := profile.GetStringArg("post_path")
	websocketPath, _ := profile.GetStringArg("websocket_path")
	userAgent, _ := profile.GetStringArg("user_agent")
	hostHeader, _ := profile.GetStringArg("host_header")
	queryString, _ := profile.GetStringArg("query_string")
	extraHeaders, _ := extractDictionaryParameters(profile, "extra_headers")

	callbackPort := stringifyOptionalValue(callbackPortValue)
	callbackHost = strings.TrimSpace(callbackHost)
	if callbackHost == "" {
		return embeddedBuildProfile{}, embeddedBuildProfile{}, fmt.Errorf("masked_https callback_host 不能为空")
	}
	if strings.TrimSpace(postPath) == "" {
		return embeddedBuildProfile{}, embeddedBuildProfile{}, fmt.Errorf("masked_https post_path 不能为空")
	}
	if strings.TrimSpace(websocketPath) == "" {
		return embeddedBuildProfile{}, embeddedBuildProfile{}, fmt.Errorf("masked_https websocket_path 不能为空")
	}

	shared := map[string]interface{}{
		"user_agent":    strings.TrimSpace(userAgent),
		"host_header":   strings.TrimSpace(hostHeader),
		"query_string":  strings.TrimSpace(strings.TrimPrefix(queryString, "?")),
		"extra_headers": extraHeaders,
	}
	httpParams := cloneParameterMap(shared)
	httpParams["post_path"] = strings.TrimSpace(postPath)
	websocketParams := cloneParameterMap(shared)
	websocketParams["websocket_path"] = strings.TrimSpace(websocketPath)

	httpProfile := embeddedBuildProfile{
		Name:         "http",
		CallbackHost: callbackHost,
		CallbackPort: callbackPort,
		AESPSK:       extractAESPSKValue(aespskValue),
		Parameters:   httpParams,
	}
	websocketProfile := embeddedBuildProfile{
		Name:         "websocket",
		CallbackHost: callbackHost,
		CallbackPort: callbackPort,
		AESPSK:       extractAESPSKValue(aespskValue),
		Parameters:   websocketParams,
	}
	return httpProfile, websocketProfile, nil
}

func buildEmbeddedConfigB64(config embeddedBuildConfig) (string, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func stringifyOptionalValue(value interface{}) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprintf("%v", value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func extractDictionaryParameters(profile agentstructs.PayloadBuildC2Profile, name string) (map[string]string, error) {
	raw, err := profile.GetArg(name)
	if err != nil || raw == nil {
		return map[string]string{}, err
	}
	result := make(map[string]string)
	switch typed := raw.(type) {
	case map[string]interface{}:
		for key, value := range typed {
			text := strings.TrimSpace(fmt.Sprintf("%v", value))
			if text == "" || text == "<nil>" {
				continue
			}
			result[key] = text
		}
	case map[string]string:
		for key, value := range typed {
			if strings.TrimSpace(value) == "" {
				continue
			}
			result[key] = strings.TrimSpace(value)
		}
	}
	return result, nil
}

func cloneParameterMap(input map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}
