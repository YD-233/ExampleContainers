package agentfunctions

import (
	"encoding/json"
	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"strings"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "ls",
		HelpString:          "ls [path]",
		Description:         "列出目录内容",
		Version:             1,
		Author:              "@zhujiayi",
		MitreAttackMappings: []string{"T1083"},
		SupportedUIFeatures: []string{"file_browser:list"},
		CommandParameters: []agentstructs.CommandParameter{
			{
				Name:             "path",
				ModalDisplayName: "Path",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:      "要列出的目录路径",
				DefaultValue:     ".",
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
					pathValue = strings.TrimSpace(pathValue)
					if pathValue == "" {
						pathValue = "."
					}
					args.AddArg(agentstructs.CommandParameter{
						Name:          "path",
						ParameterType: agentstructs.COMMAND_PARAMETER_TYPE_STRING,
						DefaultValue:  pathValue,
					})
					return nil
				}
				return args.LoadArgsFromJSONString(trimmed)
			}
			if trimmed == "" {
				trimmed = "."
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
			path, err := taskData.Args.GetStringArg("path")
			if err != nil || path == "" {
				path = "."
			}
			response.DisplayParams = &path
			return response
		},
	})
}
