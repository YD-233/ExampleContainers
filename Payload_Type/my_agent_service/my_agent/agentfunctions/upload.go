package agentfunctions

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"github.com/MythicMeta/MythicContainer/mythicrpc"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "upload",
		HelpString:          "upload <local_path>",
		Description:         "从 Mythic 上传文件到目标主机",
		Version:             1,
		Author:              "@zhujiayi",
		MitreAttackMappings: []string{"T1105"},
		SupportedUIFeatures: []string{"file_browser:upload"},
		CommandParameters: []agentstructs.CommandParameter{
			{
				Name:             "file",
				ModalDisplayName: "File",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_FILE,
				Description:      "要上传到目标主机的文件",
				DefaultValue:     "",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{
					{
						ParameterIsRequired: false,
						UIModalPosition:     1,
					},
				},
			},
			{
				Name:             "path",
				ModalDisplayName: "Path",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:      "目标路径，可以是目录或完整文件路径",
				DefaultValue:     ".",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{
					{
						ParameterIsRequired: true,
						UIModalPosition:     2,
					},
				},
			},
		},
		TaskFunctionParseArgString: func(args *agentstructs.PTTaskMessageArgsData, input string) error {
			trimmed := strings.TrimSpace(input)
			if trimmed == "" {
				return args.SetArgValue("path", ".")
			}
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
					if pathValue == "" {
						pathValue = "."
					}
					return args.SetArgValue("path", pathValue)
				}
				return args.LoadArgsFromJSONString(trimmed)
			}
			return args.SetArgValue("path", trimmed)
		},
		TaskFunctionParseArgDictionary: func(args *agentstructs.PTTaskMessageArgsData, input map[string]interface{}) error {
			return args.LoadArgsFromDictionary(input)
		},
		TaskFunctionCreateTasking: func(taskData *agentstructs.PTTaskMessageAllData) agentstructs.PTTaskCreateTaskingMessageResponse {
			response := agentstructs.PTTaskCreateTaskingMessageResponse{
				Success: true,
				TaskID:  taskData.Task.ID,
			}

			fileID, err := taskData.Args.GetStringArg("file")
			if err != nil || strings.TrimSpace(fileID) == "" {
				var rawParams map[string]interface{}
				if err := json.Unmarshal([]byte(taskData.Task.Params), &rawParams); err == nil {
					if rawFileID, ok := rawParams["file"].(string); ok && strings.TrimSpace(rawFileID) != "" {
						if addErr := taskData.Args.AddArg(agentstructs.CommandParameter{
							Name:          "file",
							ParameterType: agentstructs.COMMAND_PARAMETER_TYPE_FILE,
							DefaultValue:  strings.TrimSpace(rawFileID),
						}); addErr == nil {
							_ = taskData.Args.SetArgValue("file", strings.TrimSpace(rawFileID))
							fileID = strings.TrimSpace(rawFileID)
						}
					}
				}
			}
			if strings.TrimSpace(fileID) == "" {
				response.Success = false
				response.Error = "file 不能为空"
				return response
			}
			// 将 Mythic 侧生成的文件标识写回任务参数，方便 Agent 后续按分块拉取文件内容。
			if err := taskData.Args.AddArg(agentstructs.CommandParameter{
				Name:          "file_id",
				ParameterType: agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				DefaultValue:  fileID,
			}); err != nil {
				response.Success = false
				response.Error = err.Error()
				return response
			}
			if err := taskData.Args.SetArgValue("file_id", fileID); err != nil {
				response.Success = false
				response.Error = err.Error()
				return response
			}

			pathValue, err := taskData.Args.GetStringArg("path")
			if err != nil || strings.TrimSpace(pathValue) == "" {
				pathValue = "."
			}
			pathValue = strings.TrimSpace(pathValue)

			fileResp, err := mythicrpc.SendMythicRPCFileSearch(mythicrpc.MythicRPCFileSearchMessage{
				TaskID:      taskData.Task.ID,
				AgentFileID: fileID,
			})
			if err != nil {
				response.Success = false
				response.Error = fmt.Sprintf("查询上传文件元数据失败: %v", err)
				return response
			}
			if !fileResp.Success || len(fileResp.Files) == 0 {
				response.Success = false
				response.Error = fmt.Sprintf("查询上传文件元数据失败: %s", fileResp.Error)
				return response
			}

			filename := fileResp.Files[0].Filename
			if filename == "" {
				filename = filepath.Base(fileID)
			}
			if err := taskData.Args.AddArg(agentstructs.CommandParameter{
				Name:          "filename",
				ParameterType: agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				DefaultValue:  filename,
			}); err != nil {
				response.Success = false
				response.Error = err.Error()
				return response
			}
			if err := taskData.Args.SetArgValue("filename", filename); err != nil {
				response.Success = false
				response.Error = err.Error()
				return response
			}

			displayParams := fmt.Sprintf("%s -> %s", filename, pathValue)
			response.DisplayParams = &displayParams
			return response
		},
	})
}
