package agentfunctions

import (
	"fmt"
	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"strings"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "ps",
		HelpString:          "ps [keyword]",
		Description:         "列出进程列表，可按关键字过滤",
		Version:             1,
		Author:              "@zhujiayi",
		MitreAttackMappings: []string{"T1057"},
		CommandParameters: []agentstructs.CommandParameter{
			{
				Name:             "keyword",
				ModalDisplayName: "Keyword",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:      "可选，按进程名或命令行关键字过滤",
				DefaultValue:     "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{
					{
						ParameterIsRequired: false,
						UIModalPosition:     1,
					},
				},
			},
			{
				Name:             "limit",
				ModalDisplayName: "Limit",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_NUMBER,
				Description:      "最多返回多少条结果",
				DefaultValue:     60,
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{
					{
						ParameterIsRequired: false,
						UIModalPosition:     2,
					},
				},
			},
		},
		TaskFunctionParseArgString: func(args *agentstructs.PTTaskMessageArgsData, input string) error {
			trimmed := strings.TrimSpace(input)
			if strings.HasPrefix(trimmed, "{") {
				return args.LoadArgsFromJSONString(trimmed)
			}
			if trimmed != "" {
				args.AddArg(agentstructs.CommandParameter{
					Name:          "keyword",
					ParameterType: agentstructs.COMMAND_PARAMETER_TYPE_STRING,
					DefaultValue:  trimmed,
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
			keyword, _ := taskData.Args.GetStringArg("keyword")
			limit, _ := taskData.Args.GetNumberArg("limit")
			display := "all"
			if strings.TrimSpace(keyword) != "" {
				display = keyword
			}
			if limit > 0 {
				display = display + " limit=" + fmt.Sprintf("%.0f", limit)
			}
			response.DisplayParams = &display
			return response
		},
	})
}
