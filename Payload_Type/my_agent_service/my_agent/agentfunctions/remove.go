package agentfunctions

import (
	"encoding/json"
	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"strings"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "remove",
		HelpString:          "remove <path>",
		Description:         "删除目标主机上的文件或目录",
		Version:             1,
		Author:              "@zhujiayi",
		MitreAttackMappings: []string{"T1070.004"},
		SupportedUIFeatures: []string{"file_browser:remove"},
		CommandParameters: []agentstructs.CommandParameter{
			{
				Name:             "path",
				ModalDisplayName: "Path",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:      "要删除的路径",
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
				var fileBrowserTask struct {
					Host     string `json:"host"`
					Path     string `json:"path"`
					File     string `json:"file"`
					FullPath string `json:"full_path"`
				}
				if err := json.Unmarshal([]byte(trimmed), &fileBrowserTask); err == nil && (fileBrowserTask.FullPath != "" || fileBrowserTask.File != "" || fileBrowserTask.Path != "" || fileBrowserTask.Host != "") {
					pathValue := strings.TrimSpace(fileBrowserTask.FullPath)
					if pathValue == "" {
						switch {
						case fileBrowserTask.Path == "":
							pathValue = fileBrowserTask.File
						case fileBrowserTask.File == "":
							pathValue = fileBrowserTask.Path
						case strings.HasSuffix(fileBrowserTask.Path, "/") || strings.HasSuffix(fileBrowserTask.Path, "\\"):
							pathValue = fileBrowserTask.Path + fileBrowserTask.File
						default:
							pathValue = fileBrowserTask.Path + "/" + fileBrowserTask.File
						}
					}
					args.AddArg(agentstructs.CommandParameter{
						Name:          "path",
						ParameterType: agentstructs.COMMAND_PARAMETER_TYPE_STRING,
						DefaultValue:  strings.TrimSpace(pathValue),
					})
					return nil
				}
				return args.LoadArgsFromJSONString(trimmed)
			}
			args.AddArg(agentstructs.CommandParameter{
				Name:          "path",
				ParameterType: agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				DefaultValue:  trimmed,
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
			pathValue, err := taskData.Args.GetStringArg("path")
			if err != nil || strings.TrimSpace(pathValue) == "" {
				response.Success = false
				response.Error = "path 不能为空"
				return response
			}
			pathValue = strings.TrimSpace(pathValue)
			response.DisplayParams = &pathValue
			return response
		},
	})
}
