package agentfunctions

import (
	"encoding/json"
	"fmt"
	"strings"

	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"github.com/MythicMeta/MythicContainer/mythicrpc"
)

func onNewCallback(input agentstructs.PTOnNewCallbackAllData) agentstructs.PTOnNewCallbackResponse {
	response := agentstructs.PTOnNewCallbackResponse{
		AgentCallbackID: input.Callback.AgentCallbackID,
		Success:         true,
	}

	// 只有 session callback 才会在 process_name 里带这个标记。
	sessionLinkID := extractSessionLinkID(input.Callback.ProcessName)
	if sessionLinkID == "" {
		return response
	}

	searchResp, err := mythicrpc.SendMythicRPCAgentStorageSearch(mythicrpc.MythicRPCAgentstorageSearchMessage{
		SearchUniqueID: "session_link:" + sessionLinkID,
	})
	if err != nil {
		response.Success = false
		response.Error = fmt.Sprintf("查询 session Agent Storage 失败: %v", err)
		return response
	}
	if !searchResp.Success {
		response.Success = false
		response.Error = searchResp.Error
		return response
	}
	if len(searchResp.AgentStorageMessages) == 0 {
		return response
	}

	record := sessionLinkRecord{}
	if err := json.Unmarshal(searchResp.AgentStorageMessages[0].Data, &record); err != nil {
		response.Success = false
		response.Error = fmt.Sprintf("解析 session Agent Storage 失败: %v", err)
		return response
	}

	// 把子 callback 描述更新成“来自哪个父 beacon”，这样在 UI 里更容易识别关系。
	description := fmt.Sprintf("session of %d [session:%s]", record.ParentCallbackDisplayID, record.SessionLinkID)
	callbackUpdateResp, err := mythicrpc.SendMythicRPCCallbackUpdate(mythicrpc.MythicRPCCallbackUpdateMessage{
		CallbackID:  &input.Callback.ID,
		Description: &description,
	})
	if err != nil {
		response.Success = false
		response.Error = fmt.Sprintf("更新 session callback 描述失败: %v", err)
		return response
	}
	if !callbackUpdateResp.Success {
		response.Success = false
		response.Error = callbackUpdateResp.Error
		return response
	}

	notifyResp, err := mythicrpc.SendMythicRPCResponseCreate(mythicrpc.MythicRPCResponseCreateMessage{
		TaskID:   record.ParentTaskID,
		Response: []byte(fmt.Sprintf("session callback ready: %d (uuid=%s)", input.Callback.DisplayID, input.Callback.AgentCallbackID)),
	})
	if err != nil {
		response.Success = false
		response.Error = fmt.Sprintf("回写 session callback 响应失败: %v", err)
		return response
	}
	if !notifyResp.Success {
		response.Success = false
		response.Error = notifyResp.Error
		return response
	}

	_, _ = mythicrpc.SendMythicRPCAgentStorageRemove(mythicrpc.MythicRPCAgentstorageRemoveMessage{
		UniqueID: "session_link:" + sessionLinkID,
	})
	return response
}

func extractSessionLinkID(processName string) string {
	startMarker := "[session:"
	start := strings.Index(processName, startMarker)
	if start < 0 {
		return ""
	}
	start += len(startMarker)
	end := strings.Index(processName[start:], "]")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(processName[start : start+end])
}
