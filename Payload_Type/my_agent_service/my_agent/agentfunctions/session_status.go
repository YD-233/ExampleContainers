package agentfunctions

import (
	"strings"

	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "session_status",
		HelpString:          "session_status",
		Description:         "查询当前 beacon/session 的 session 状态",
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
			display := "session status"
			response.DisplayParams = &display
			return response
		},
	})
}
