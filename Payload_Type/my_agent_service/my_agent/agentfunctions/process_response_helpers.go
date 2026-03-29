package agentfunctions

import (
	"encoding/json"
	"fmt"
	"strings"

	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"github.com/MythicMeta/MythicContainer/mythicrpc"
)

type processResponsePayload struct {
	Kind        string                      `json:"kind"`
	Summary     string                      `json:"summary,omitempty"`
	Command     string                      `json:"command,omitempty"`
	RawOutput   string                      `json:"raw_output,omitempty"`
	Artifacts   []processResponseArtifact   `json:"artifacts,omitempty"`
	Credentials []processResponseCredential `json:"credentials,omitempty"`
}

type processResponseArtifact struct {
	BaseArtifact string `json:"base_artifact"`
	Artifact     string `json:"artifact"`
}

type processResponseCredential struct {
	CredentialType string `json:"credential_type"`
	Realm          string `json:"realm"`
	Account        string `json:"account"`
	Credential     string `json:"credential"`
	Comment        string `json:"comment,omitempty"`
	Metadata       string `json:"metadata,omitempty"`
}

func handleStructuredProcessResponse(input agentstructs.PtTaskProcessResponseMessage) agentstructs.PTTaskProcessResponseMessageResponse {
	response := agentstructs.PTTaskProcessResponseMessageResponse{
		TaskID:  input.TaskData.Task.ID,
		Success: true,
	}

	payload, err := decodeProcessResponsePayload(input.Response)
	if err != nil {
		response.Success = false
		response.Error = err.Error()
		return response
	}

	if len(payload.Artifacts) > 0 {
		for _, artifact := range payload.Artifacts {
			if strings.TrimSpace(artifact.BaseArtifact) == "" || strings.TrimSpace(artifact.Artifact) == "" {
				continue
			}
			_, err := mythicrpc.SendMythicRPCArtifactCreate(mythicrpc.MythicRPCArtifactCreateMessage{
				TaskID:           input.TaskData.Task.ID,
				BaseArtifactType: strings.TrimSpace(artifact.BaseArtifact),
				ArtifactMessage:  strings.TrimSpace(artifact.Artifact),
			})
			if err != nil {
				response.Success = false
				response.Error = fmt.Sprintf("登记 artifact 失败: %v", err)
				return response
			}
		}
	}

	normalizedCreds := normalizeCredentials(payload.Credentials)
	if len(normalizedCreds) > 0 {
		_, err := mythicrpc.SendMythicRPCCredentialCreate(mythicrpc.MythicRPCCredentialCreateMessage{
			TaskID:      input.TaskData.Task.ID,
			Credentials: normalizedCreds,
		})
		if err != nil {
			response.Success = false
			response.Error = fmt.Sprintf("登记 credential 失败: %v", err)
			return response
		}
	}

	if summary := buildStructuredSummary(payload, len(normalizedCreds)); summary != "" {
		_, err := mythicrpc.SendMythicRPCResponseCreate(mythicrpc.MythicRPCResponseCreateMessage{
			TaskID:   input.TaskData.Task.ID,
			Response: []byte(summary),
		})
		if err != nil {
			response.Success = false
			response.Error = fmt.Sprintf("创建响应失败: %v", err)
			return response
		}
	}

	return response
}

func decodeProcessResponsePayload(raw interface{}) (processResponsePayload, error) {
	payload := processResponsePayload{}
	if raw == nil {
		return payload, fmt.Errorf("process_response 为空")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return payload, fmt.Errorf("序列化 process_response 失败: %w", err)
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return payload, fmt.Errorf("解析 process_response 失败: %w", err)
	}
	return payload, nil
}

func normalizeCredentials(input []processResponseCredential) []mythicrpc.MythicRPCCredentialCreateCredentialData {
	results := make([]mythicrpc.MythicRPCCredentialCreateCredentialData, 0, len(input))
	for _, credential := range input {
		if strings.TrimSpace(credential.Credential) == "" {
			continue
		}
		credentialType := strings.ToLower(strings.TrimSpace(credential.CredentialType))
		if credentialType == "" {
			credentialType = "plaintext"
		}
		results = append(results, mythicrpc.MythicRPCCredentialCreateCredentialData{
			CredentialType: credentialType,
			Realm:          strings.TrimSpace(credential.Realm),
			Account:        strings.TrimSpace(credential.Account),
			Credential:     strings.TrimSpace(credential.Credential),
			Comment:        strings.TrimSpace(credential.Comment),
			ExtraData:      strings.TrimSpace(credential.Metadata),
		})
	}
	return results
}

func buildStructuredSummary(payload processResponsePayload, credentialCount int) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(payload.Summary) != "" {
		parts = append(parts, strings.TrimSpace(payload.Summary))
	}
	if len(payload.Artifacts) > 0 {
		parts = append(parts, fmt.Sprintf("登记 artifact %d 条", len(payload.Artifacts)))
	}
	if credentialCount > 0 {
		parts = append(parts, fmt.Sprintf("登记 credential %d 条", credentialCount))
	}
	return strings.Join(parts, "\n")
}
