package agentfunctions

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"github.com/MythicMeta/MythicContainer/mythicrpc"
	"github.com/google/uuid"
)

func init() {
	agentstructs.AllPayloadData.Get("my_agent").AddCommand(agentstructs.Command{
		Name:                "session_start",
		HelpString:          "session_start",
		Description:         "从 beacon callback 派生一个 websocket session callback",
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
			// 这里提前生成 session_link_id，并写入 Agent Storage。
			// 后续 websocket 派生出来的新 callback 会在 OnNewCallback 阶段靠这个 ID 回溯父 beacon/task。
			sessionLinkID := uuid.NewString()
			record := sessionLinkRecord{
				SessionLinkID:           sessionLinkID,
				ParentCallbackID:        taskData.Callback.ID,
				ParentCallbackDisplayID: taskData.Callback.DisplayID,
				ParentTaskID:            taskData.Task.ID,
				CreatedAt:               time.Now().UTC().Format(time.RFC3339),
			}
			recordBytes, err := json.Marshal(record)
			if err != nil {
				response.Success = false
				response.Error = fmt.Sprintf("序列化 session 关联信息失败: %v", err)
				return response
			}
			storageResp, err := mythicrpc.SendMythicRPCAgentStorageCreate(mythicrpc.MythicRPCAgentstorageCreateMessage{
				UniqueID:    "session_link:" + sessionLinkID,
				DataToStore: recordBytes,
			})
			if err != nil {
				response.Success = false
				response.Error = fmt.Sprintf("写入 Agent Storage 失败: %v", err)
				return response
			}
			if !storageResp.Success {
				response.Success = false
				response.Error = storageResp.Error
				return response
			}
			manualArgs := fmt.Sprintf(`{"session_link_id":"%s"}`, sessionLinkID)
			taskData.Args.SetManualArgs(manualArgs)
			display := "start session " + sessionLinkID
			response.DisplayParams = &display
			return response
		},
	})
}
