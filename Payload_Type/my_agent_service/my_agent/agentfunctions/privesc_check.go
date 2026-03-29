package agentfunctions

import (
	"fmt"
	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"strings"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "privesc_check",
		HelpString:          "privesc_check [keyword]",
		Description:         "执行基础权限提升环境探测，输出当前权限状态与常见提权线索",
		Version:             1,
		Author:              "@zhujiayi",
		MitreAttackMappings: []string{"T1068", "T1548"},
		CommandParameters: []agentstructs.CommandParameter{
			{
				Name:             "keyword",
				ModalDisplayName: "Keyword",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:      "可选，仅返回包含关键字的探测结果",
				DefaultValue:     "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{
					{
						ParameterIsRequired: false,
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
			display := "full"
			if strings.TrimSpace(keyword) != "" {
				display = fmt.Sprintf("keyword=%s", strings.TrimSpace(keyword))
			}
			response.DisplayParams = &display
			return response
		},
	})
}
