package agentfunctions

import (
	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"strings"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "pty",
		HelpString:          "pty {\"shell\":\"/bin/sh\"}",
		Description:         "启动交互式 shell 会话",
		Version:             1,
		Author:              "@zhujiayi",
		MitreAttackMappings: []string{"T1059"},
		SupportedUIFeatures: []string{"task_response:interactive"},
		CommandParameters: []agentstructs.CommandParameter{
			{
				Name:             "shell",
				ModalDisplayName: "Shell",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:      "可选的 shell 路径",
				DefaultValue:     "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{
					ParameterIsRequired: false,
					UIModalPosition:     1,
				}},
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
			return args.SetArgValue("shell", trimmed)
		},
		TaskFunctionParseArgDictionary: func(args *agentstructs.PTTaskMessageArgsData, input map[string]interface{}) error {
			return args.LoadArgsFromDictionary(input)
		},
		TaskFunctionCreateTasking: func(taskData *agentstructs.PTTaskMessageAllData) agentstructs.PTTaskCreateTaskingMessageResponse {
			response := agentstructs.PTTaskCreateTaskingMessageResponse{
				Success: true,
				TaskID:  taskData.Task.ID,
			}
			if shell, err := taskData.Args.GetStringArg("shell"); err == nil && strings.TrimSpace(shell) != "" {
				shell = strings.TrimSpace(shell)
				response.DisplayParams = &shell
			} else {
				display := "interactive shell"
				response.DisplayParams = &display
			}
			return response
		},
	})
}
