package agentfunctions

import (
	"strings"

	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "session_stop",
		HelpString:          "session_stop",
		Description:         "停止当前 session callback",
		Version:             1,
		Author:              "@zhujiayi",
		MitreAttackMappings: []string{"T1071"},
		TaskFunctionParseArgString: func(args *agentstructs.PTTaskMessageArgsData, input string) error {
			if strings.TrimSpace(input) == "" {
				return nil
			}
			if strings.HasPrefix(strings.TrimSpace(input), "{") {
				return args.LoadArgsFromJSONString(input)
			}
			return nil
		},
		TaskFunctionParseArgDictionary: func(args *agentstructs.PTTaskMessageArgsData, input map[string]interface{}) error {
			return args.LoadArgsFromDictionary(input)
		},
		TaskFunctionCreateTasking: func(taskData *agentstructs.PTTaskMessageAllData) agentstructs.PTTaskCreateTaskingMessageResponse {
			response := agentstructs.PTTaskCreateTaskingMessageResponse{Success: true, TaskID: taskData.Task.ID}
			display := "stop session"
			response.DisplayParams = &display
			return response
		},
	})
}
