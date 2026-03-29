package agentfunctions

import (
	"fmt"
	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"strings"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "privesc_tool",
		HelpString:          "privesc_tool {path,args,timeout_seconds}",
		Description:         "调用外部提权辅助工具或脚本，并回传输出结果",
		Version:             1,
		Author:              "@zhujiayi",
		MitreAttackMappings: []string{"T1068", "T1548"},
		CommandParameters: []agentstructs.CommandParameter{
			{
				Name:             "path",
				ModalDisplayName: "Tool Path",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:      "工具或脚本的本地路径",
				DefaultValue:     "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{
					{
						ParameterIsRequired: true,
						UIModalPosition:     1,
					},
				},
			},
			{
				Name:             "args",
				ModalDisplayName: "Arguments",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:      "可选，传给工具的额外参数",
				DefaultValue:     "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{
					{
						ParameterIsRequired: false,
						UIModalPosition:     2,
					},
				},
			},
			{
				Name:             "timeout_seconds",
				ModalDisplayName: "Timeout",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_NUMBER,
				Description:      "超时时间，默认 120 秒",
				DefaultValue:     120,
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{
					{
						ParameterIsRequired: false,
						UIModalPosition:     3,
					},
				},
			},
		},
		TaskFunctionParseArgString: func(args *agentstructs.PTTaskMessageArgsData, input string) error {
			trimmed := strings.TrimSpace(input)
			if strings.HasPrefix(trimmed, "{") {
				return args.LoadArgsFromJSONString(trimmed)
			}
			if trimmed == "" {
				return nil
			}
			parts := strings.Fields(trimmed)
			args.AddArg(agentstructs.CommandParameter{
				Name:          "path",
				ParameterType: agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				DefaultValue:  parts[0],
			})
			if len(parts) > 1 {
				args.AddArg(agentstructs.CommandParameter{
					Name:          "args",
					ParameterType: agentstructs.COMMAND_PARAMETER_TYPE_STRING,
					DefaultValue:  strings.Join(parts[1:], " "),
				})
			}
			return nil
		},
		TaskFunctionParseArgDictionary: func(args *agentstructs.PTTaskMessageArgsData, input map[string]interface{}) error {
			return args.LoadArgsFromDictionary(input)
		},
		TaskFunctionCreateTasking: func(taskData *agentstructs.PTTaskMessageAllData) agentstructs.PTTaskCreateTaskingMessageResponse {
			response := agentstructs.PTTaskCreateTaskingMessageResponse{
				Success: true,
				TaskID:  taskData.Task.ID,
			}
			path, err := taskData.Args.GetStringArg("path")
			if err != nil || strings.TrimSpace(path) == "" {
				response.Success = false
				response.Error = "path 不能为空"
				return response
			}
			argsValue, _ := taskData.Args.GetStringArg("args")
			display := strings.TrimSpace(path)
			if strings.TrimSpace(argsValue) != "" {
				display = fmt.Sprintf("%s %s", display, strings.TrimSpace(argsValue))
			}
			response.DisplayParams = &display
			return response
		},
	})
}
