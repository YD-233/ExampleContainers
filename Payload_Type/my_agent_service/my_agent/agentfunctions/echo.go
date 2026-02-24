package agentfunctions

import (
	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"strings"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "echo",
		HelpString:          "echo {\"text\":\"hello\"}",
		Description:         "回显字符串，作为最小命令模板",
		Version:             1,
		Author:              "@zhujiayi",
		MitreAttackMappings: []string{"T1036"},
		CommandParameters: []agentstructs.CommandParameter{
			{
				Name:             "text",
				ModalDisplayName: "Text",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:      "要回显的内容",
				DefaultValue:     "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{
					{
						ParameterIsRequired: true,
						UIModalPosition:     1,
					},
				},
			},
		},
		TaskFunctionParseArgString: func(args *agentstructs.PTTaskMessageArgsData, input string) error {
			trimmed := strings.TrimSpace(input)
			if strings.HasPrefix(trimmed, "{") {
				return args.LoadArgsFromJSONString(trimmed)
			}
			args.AddArg(agentstructs.CommandParameter{
				Name:          "text",
				ParameterType: agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				DefaultValue:  input,
			})
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
			if text, err := taskData.Args.GetStringArg("text"); err != nil {
				response.Success = false
				response.Error = err.Error()
				return response
			} else {
				response.DisplayParams = &text
				return response
			}
		},
	})
}
