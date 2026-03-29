package agentfunctions

import (
	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"strings"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "report",
		HelpString:          "report {json}",
		Description:         "通过 process_response 向 Mythic 登记 artifact、credential 和附加说明",
		Version:             1,
		Author:              "@zhujiayi",
		MitreAttackMappings: []string{"T1552", "T1083"},
		CommandParameters: []agentstructs.CommandParameter{
			{
				Name:                      "summary",
				ModalDisplayName:          "Summary",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:               "显示给操作员的摘要信息",
				DefaultValue:              "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: false, UIModalPosition: 1}},
			},
			{
				Name:                      "artifact_type",
				ModalDisplayName:          "Artifact Type",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:               "如 Process Create、File Write",
				DefaultValue:              "Process Create",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: false, UIModalPosition: 2}},
			},
			{
				Name:                      "artifact_message",
				ModalDisplayName:          "Artifact",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:               "具体 artifact 内容",
				DefaultValue:              "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: false, UIModalPosition: 3}},
			},
			{
				Name:                      "credential_type",
				ModalDisplayName:          "Credential Type",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_CHOOSE_ONE,
				Description:               "凭据类型",
				DefaultValue:              "plaintext",
				Choices:                   []string{"plaintext", "certificate", "hash", "key", "ticket", "cookie"},
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: false, UIModalPosition: 4}},
			},
			{
				Name:                      "realm",
				ModalDisplayName:          "Realm",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:               "域或主机名",
				DefaultValue:              "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: false, UIModalPosition: 5}},
			},
			{
				Name:                      "account",
				ModalDisplayName:          "Account",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:               "用户名或账号名",
				DefaultValue:              "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: false, UIModalPosition: 6}},
			},
			{
				Name:                      "credential",
				ModalDisplayName:          "Credential",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:               "凭据值",
				DefaultValue:              "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: false, UIModalPosition: 7}},
			},
			{
				Name:                      "comment",
				ModalDisplayName:          "Comment",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:               "凭据备注",
				DefaultValue:              "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: false, UIModalPosition: 8}},
			},
			{
				Name:                      "metadata",
				ModalDisplayName:          "Metadata",
				ParameterType:             agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:               "凭据附加信息",
				DefaultValue:              "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{{ParameterIsRequired: false, UIModalPosition: 9}},
			},
		},
		TaskFunctionParseArgString: func(args *agentstructs.PTTaskMessageArgsData, input string) error {
			trimmed := strings.TrimSpace(input)
			if strings.HasPrefix(trimmed, "{") {
				return args.LoadArgsFromJSONString(trimmed)
			}
			return args.SetArgValue("summary", trimmed)
		},
		TaskFunctionParseArgDictionary: func(args *agentstructs.PTTaskMessageArgsData, input map[string]interface{}) error {
			return args.LoadArgsFromDictionary(input)
		},
		TaskFunctionCreateTasking: func(taskData *agentstructs.PTTaskMessageAllData) agentstructs.PTTaskCreateTaskingMessageResponse {
			response := agentstructs.PTTaskCreateTaskingMessageResponse{
				Success: true,
				TaskID:  taskData.Task.ID,
			}
			summary, _ := taskData.Args.GetStringArg("summary")
			artifactMessage, _ := taskData.Args.GetStringArg("artifact_message")
			account, _ := taskData.Args.GetStringArg("account")
			credential, _ := taskData.Args.GetStringArg("credential")
			if strings.TrimSpace(summary) == "" && strings.TrimSpace(artifactMessage) == "" && strings.TrimSpace(credential) == "" {
				response.Success = false
				response.Error = "至少提供 summary、artifact_message 或 credential"
				return response
			}
			display := firstNonEmptyString(strings.TrimSpace(summary), strings.TrimSpace(artifactMessage), strings.TrimSpace(account))
			response.DisplayParams = &display
			return response
		},
		TaskFunctionProcessResponse: handleStructuredProcessResponse,
	})
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "report"
}
