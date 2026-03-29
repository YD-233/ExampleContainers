package agentfunctions

import (
	"fmt"
	"strconv"
	"strings"

	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"github.com/MythicMeta/MythicContainer/mythicrpc"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "rpfwd_stop",
		HelpString:          "rpfwd_stop 8080",
		Description:         "停止目标主机上的反向端口转发监听",
		Version:             1,
		Author:              "@zhujiayi",
		MitreAttackMappings: []string{"T1090"},
		CommandParameters: []agentstructs.CommandParameter{
			{
				Name:                      "local_port",
				ModalDisplayName:          "Local Port",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_NUMBER,
				Description:               "目标主机监听端口",
				DefaultValue:              8080,
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: true, UIModalPosition: 1}},
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
			localPort, err := strconv.Atoi(trimmed)
			if err != nil {
				return fmt.Errorf("local_port 必须是数字")
			}
			return args.SetArgValue("local_port", localPort)
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
			proxyResp, err := mythicrpc.SendMythicRPCProxyStop(mythicrpc.MythicRPCProxyStopMessage{
				TaskID:   taskData.Task.ID,
				Port:     int(localPort),
				PortType: callbackPortTypeRpfwd,
			})
			if err != nil {
				response.Success = false
				response.Error = fmt.Sprintf("停止 RPFWD 失败: %v", err)
				return response
			}
			if !proxyResp.Success {
				response.Success = false
				response.Error = proxyResp.Error
				return response
			}
			display := fmt.Sprintf("stop %d", int(localPort))
			response.DisplayParams = &display
			return response
		},
	})
}
