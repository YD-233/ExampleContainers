package c2functions

import (
	"MaskedHTTPS/common"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	c2structs "github.com/MythicMeta/MythicContainer/c2_structs"
)

var c2Definition = c2structs.C2Profile{
	Name:             "masked_https",
	Description:      "基于 HTTPS/WSS 的基础流量伪装 C2 Profile",
	Author:           "@zhujiayi",
	IsP2p:            false,
	IsServerRouted:   false,
	ServerBinaryPath: "./server/masked_https_server",
	SemVer:           "0.0.1",
	ConfigCheckFunction: func(message c2structs.C2ConfigCheckMessage) c2structs.C2ConfigCheckMessageResponse {
		return configCheck(message)
	},
	OPSECCheckFunction: func(message c2structs.C2OPSECMessage) c2structs.C2OPSECMessageResponse {
		return opsecCheck(message)
	},
}

var c2Parameters = []c2structs.C2Parameter{
	{
		Name:          "callback_host",
		Description:   "Agent 连接的 HTTPS 地址，例如 https://example.com",
		DefaultValue:  "https://127.0.0.1",
		ParameterType: c2structs.C2_PARAMETER_TYPE_STRING,
		Required:      true,
		VerifierRegex: "^(https://).+",
		UiPosition:    1,
	},
	{
		Name:          "callback_port",
		Description:   "Agent 连接的端口",
		DefaultValue:  8443,
		ParameterType: c2structs.C2_PARAMETER_TYPE_NUMBER,
		Required:      true,
		VerifierRegex: "^[0-9]{1,5}$",
		UiPosition:    2,
	},
	{
		Name:          "post_path",
		Description:   "Beacon POST 路径",
		DefaultValue:  "/cdn-cgi/submit",
		ParameterType: c2structs.C2_PARAMETER_TYPE_STRING,
		Required:      true,
		UiPosition:    3,
	},
	{
		Name:          "websocket_path",
		Description:   "Session WSS 路径",
		DefaultValue:  "/cdn-cgi/ws",
		ParameterType: c2structs.C2_PARAMETER_TYPE_STRING,
		Required:      true,
		UiPosition:    4,
	},
	{
		Name:          "user_agent",
		Description:   "Agent 使用的 User-Agent",
		DefaultValue:  common.DefaultConfig().UserAgent,
		ParameterType: c2structs.C2_PARAMETER_TYPE_STRING,
		Required:      false,
		UiPosition:    5,
	},
	{
		Name:          "host_header",
		Description:   "可选，自定义 Host 头；留空则跟随 callback_host",
		DefaultValue:  "",
		ParameterType: c2structs.C2_PARAMETER_TYPE_STRING,
		Required:      false,
		UiPosition:    6,
	},
	{
		Name:              "extra_headers",
		Description:       "额外 Header 字典",
		ParameterType:     c2structs.C2_PARAMETER_TYPE_DICTIONARY,
		Required:          false,
		DictionaryChoices: []c2structs.C2ParameterDictionary{{Name: "Accept", DefaultValue: "*/*", DefaultShow: true}, {Name: "Accept-Language", DefaultValue: "zh-CN,zh;q=0.9", DefaultShow: true}},
		UiPosition:        7,
	},
	{
		Name:          "query_string",
		Description:   "可选，请求统一附带的查询字符串，不要带问号",
		DefaultValue:  "",
		ParameterType: c2structs.C2_PARAMETER_TYPE_STRING,
		Required:      false,
		UiPosition:    8,
	},
	{
		Name:          "AESPSK",
		Description:   "Encryption Type",
		DefaultValue:  "aes256_hmac",
		ParameterType: c2structs.C2_PARAMETER_TYPE_CHOOSE_ONE,
		Required:      false,
		IsCryptoType:  true,
		Choices:       []string{"aes256_hmac", "none"},
		UiPosition:    9,
	},
}

func Initialize() {
	c2structs.AllC2Data.Get(c2Definition.Name).AddC2Definition(c2Definition)
	c2structs.AllC2Data.Get(c2Definition.Name).AddParameters(c2Parameters)
}

func configCheck(message c2structs.C2ConfigCheckMessage) c2structs.C2ConfigCheckMessageResponse {
	response := c2structs.C2ConfigCheckMessageResponse{
		Success:               true,
		RestartInternalServer: true,
	}

	configPath := common.ConfigPath(serverFolderPath())
	config, err := common.LoadConfig(configPath)
	if err != nil {
		response.Success = false
		response.Error = "读取服务端配置失败: " + err.Error()
		return response
	}

	config, err = common.ApplyC2Parameters(config, message.C2Parameters)
	if err != nil {
		response.Success = false
		response.Error = "应用 C2 参数失败: " + err.Error()
		return response
	}

	if err := validateRuntimeConfig(config); err != nil {
		response.Success = false
		response.Error = err.Error()
		return response
	}
	if err := common.SaveConfig(configPath, config); err != nil {
		response.Success = false
		response.Error = "写入服务端配置失败: " + err.Error()
		return response
	}

	response.Message = fmt.Sprintf("已同步 masked_https 配置: bind_port=%d post_path=%s websocket_path=%s use_ssl=%t", config.BindPort, config.PostPath, config.WebsocketPath, config.UseSSL)
	return response
}

func opsecCheck(message c2structs.C2OPSECMessage) c2structs.C2OPSECMessageResponse {
	response := c2structs.C2OPSECMessageResponse{Success: true}
	callbackHost, _ := message.GetStringArg("callback_host")
	callbackPort, _ := message.GetArg("callback_port")
	postPath, _ := message.GetStringArg("post_path")
	websocketPath, _ := message.GetStringArg("websocket_path")

	portText := strings.TrimSpace(fmt.Sprintf("%v", callbackPort))
	notes := make([]string, 0)
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(callbackHost)), "https://") {
		notes = append(notes, "callback_host 当前不是 https://，不符合 masked_https 的命名预期")
	}
	if portText != "" && portText != "443" && portText != "8443" {
		notes = append(notes, fmt.Sprintf("当前 HTTPS 端口为 %s，属于非典型端口", portText))
	}
	if strings.TrimSpace(postPath) == strings.TrimSpace(websocketPath) {
		notes = append(notes, "post_path 与 websocket_path 相同，会降低流量区分度")
	}
	if len(notes) == 0 {
		response.Message = "HTTPS、路径与会话通道配置看起来合理"
		return response
	}
	response.Message = strings.Join(notes, "; ")
	return response
}

func validateRuntimeConfig(config common.RuntimeConfig) error {
	if strings.TrimSpace(config.CallbackHost) == "" {
		return fmt.Errorf("callback_host 不能为空")
	}
	parsed, err := url.Parse(strings.TrimSpace(config.CallbackHost))
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("callback_host 无法解析")
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(config.CallbackHost)), "https://") {
		return fmt.Errorf("callback_host 必须以 https:// 开头")
	}
	if strings.TrimSpace(config.PostPath) == "" {
		return fmt.Errorf("post_path 不能为空")
	}
	if strings.TrimSpace(config.WebsocketPath) == "" {
		return fmt.Errorf("websocket_path 不能为空")
	}
	return nil
}

func serverFolderPath() string {
	if abs, err := filepath.Abs("./server"); err == nil {
		return abs
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "server")
}
