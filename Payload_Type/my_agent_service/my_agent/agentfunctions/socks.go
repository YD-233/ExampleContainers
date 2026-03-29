package agentfunctions

import (
	"fmt"
	"strconv"
	"strings"

	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"github.com/MythicMeta/MythicContainer/mythicrpc"
)

const (
	callbackPortTypeSocks = "socks"
	callbackPortTypeRpfwd = "rpfwd"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "socks",
		HelpString:          "socks 1080",
		Description:         "启动 Mythic SOCKS 代理端口并在 agent 侧启用 SOCKS 通道",
		Version:             1,
		Author:              "@zhujiayi",
		MitreAttackMappings: []string{"T1090"},
		CommandParameters: []agentstructs.CommandParameter{
			{
				Name:                      "local_port",
				ModalDisplayName:          "Local Port",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_NUMBER,
				Description:               "在 Mythic 侧打开的 SOCKS 端口",
				DefaultValue:              1080,
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: true, UIModalPosition: 1}},
			},
			{
				Name:                      "username",
				ModalDisplayName:          "Username",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:               "可选 SOCKS 连接用户名",
				DefaultValue:              "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: false, UIModalPosition: 2}},
			},
			{
				Name:                      "password",
				ModalDisplayName:          "Password",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:               "可选 SOCKS 连接密码",
				DefaultValue:              "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: false, UIModalPosition: 3}},
			},
		},
		TaskFunctionParseArgString: func(args *agentstructs.PTTaskMessageArgsData, input string) error {
			trimmed := strings.TrimSpace(input)
			if trimmed == "" {
				return nil
			}
			if strings.HasPrefix(trimmed, "{") {
				return args.LoadArgsFromJSONString(trimmed)
			}
			port, err := strconv.Atoi(trimmed)
			if err != nil {
				return fmt.Errorf("local_port 必须是数字")
			}
			return args.SetArgValue("local_port", port)
		},
		TaskFunctionParseArgDictionary: func(args *agentstructs.PTTaskMessageArgsData, input map[string]interface{}) error {
			return args.LoadArgsFromDictionary(input)
		},
		TaskFunctionCreateTasking: func(taskData *agentstructs.PTTaskMessageAllData) agentstructs.PTTaskCreateTaskingMessageResponse {
			response := agentstructs.PTTaskCreateTaskingMessageResponse{Success: true, TaskID: taskData.Task.ID}
			localPort, err := taskData.Args.GetNumberArg("local_port")
			if err != nil {
				response.Success = false
				response.Error = err.Error()
				return response
			}
			username, _ := taskData.Args.GetStringArg("username")
			password, _ := taskData.Args.GetStringArg("password")
			proxyResp, err := mythicrpc.SendMythicRPCProxyStart(mythicrpc.MythicRPCProxyStartMessage{
				TaskID:    taskData.Task.ID,
				LocalPort: int(localPort),
				PortType:  callbackPortTypeSocks,
				Username:  strings.TrimSpace(username),
				Password:  strings.TrimSpace(password),
			})
			if err != nil {
				response.Success = false
				response.Error = fmt.Sprintf("启动 SOCKS 监听失败: %v", err)
				return response
			}
			if !proxyResp.Success {
				response.Success = false
				response.Error = proxyResp.Error
				return response
			}
			display := fmt.Sprintf("listen %d", proxyResp.LocalPort)
			response.DisplayParams = &display
			return response
		},
	})
}
