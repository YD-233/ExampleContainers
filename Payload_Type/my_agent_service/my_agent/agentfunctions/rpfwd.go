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
		Name:                "rpfwd",
		HelpString:          "rpfwd {\"local_port\":8080,\"remote_ip\":\"127.0.0.1\",\"remote_port\":80}",
		Description:         "在目标主机上启动反向端口转发监听",
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
			{
				Name:                      "remote_ip",
				ModalDisplayName:          "Remote IP",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:               "Mythic 将连接的远端 IP",
				DefaultValue:              "127.0.0.1",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: true, UIModalPosition: 2}},
			},
			{
				Name:                      "remote_port",
				ModalDisplayName:          "Remote Port",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_NUMBER,
				Description:               "Mythic 将连接的远端端口",
				DefaultValue:              80,
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: true, UIModalPosition: 3}},
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
			parts := strings.Fields(trimmed)
			if len(parts) != 3 {
				return fmt.Errorf("格式应为: <local_port> <remote_ip> <remote_port>")
			}
			localPort, err := strconv.Atoi(parts[0])
			if err != nil {
				return fmt.Errorf("local_port 必须是数字")
			}
			remotePort, err := strconv.Atoi(parts[2])
			if err != nil {
				return fmt.Errorf("remote_port 必须是数字")
			}
			if err := args.SetArgValue("local_port", localPort); err != nil {
				return err
			}
			if err := args.SetArgValue("remote_ip", parts[1]); err != nil {
				return err
			}
			return args.SetArgValue("remote_port", remotePort)
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
			remotePort, err := taskData.Args.GetNumberArg("remote_port")
			if err != nil {
				response.Success = false
				response.Error = err.Error()
				return response
			}
			remoteIP, err := taskData.Args.GetStringArg("remote_ip")
			if err != nil {
				response.Success = false
				response.Error = err.Error()
				return response
			}
			proxyResp, err := mythicrpc.SendMythicRPCProxyStart(mythicrpc.MythicRPCProxyStartMessage{
				TaskID:     taskData.Task.ID,
				LocalPort:  int(localPort),
				RemotePort: int(remotePort),
				RemoteIP:   strings.TrimSpace(remoteIP),
				PortType:   callbackPortTypeRpfwd,
			})
			if err != nil {
				response.Success = false
				response.Error = fmt.Sprintf("启动 RPFWD 失败: %v", err)
				return response
			}
			if !proxyResp.Success {
				response.Success = false
				response.Error = proxyResp.Error
				return response
			}
			display := fmt.Sprintf("%d -> %s:%d", int(localPort), strings.TrimSpace(remoteIP), int(remotePort))
			response.DisplayParams = &display
			return response
		},
	})
}
