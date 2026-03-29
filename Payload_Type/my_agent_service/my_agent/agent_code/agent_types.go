package main

type CheckinMessage struct {
	Action         string   `json:"action"`
	UUID           string   `json:"uuid"`
	IPs            []string `json:"ips"`
	OS             string   `json:"os"`
	User           string   `json:"user"`
	Host           string   `json:"host"`
	PID            int      `json:"pid"`
	Architecture   string   `json:"architecture"`
	Domain         string   `json:"domain"`
	IntegrityLevel int      `json:"integrity_level"`
	ProcessName    string   `json:"process_name"`
}

type Task struct {
	Command    string  `json:"command"`
	Parameters string  `json:"parameters"`
	ID         string  `json:"id"`
	Timestamp  float64 `json:"timestamp"`
}

type TaskResponse struct {
	TaskID          string `json:"task_id"`
	UserOutput      string `json:"user_output"`
	Completed       bool   `json:"completed"`
	Status          string `json:"status,omitempty"`
	FileBrowser     any    `json:"file_browser,omitempty"`
	RemovedFiles    any    `json:"removed_files,omitempty"`
	Artifacts       any    `json:"artifacts,omitempty"`
	Credentials     any    `json:"credentials,omitempty"`
	ProcessResponse any    `json:"process_response,omitempty"`
}

type ReportArtifact struct {
	BaseArtifact string `json:"base_artifact"`
	Artifact     string `json:"artifact"`
	NeedsCleanup bool   `json:"needs_cleanup,omitempty"`
	Resolved     bool   `json:"resolved,omitempty"`
}

type ReportCredential struct {
	CredentialType string `json:"credential_type"`
	Realm          string `json:"realm"`
	Account        string `json:"account"`
	Credential     string `json:"credential"`
	Comment        string `json:"comment,omitempty"`
	Metadata       string `json:"metadata,omitempty"`
}

type ProcessResponseEnvelope struct {
	Kind        string             `json:"kind"`
	Summary     string             `json:"summary,omitempty"`
	Command     string             `json:"command,omitempty"`
	RawOutput   string             `json:"raw_output,omitempty"`
	Artifacts   []ReportArtifact   `json:"artifacts,omitempty"`
	Credentials []ReportCredential `json:"credentials,omitempty"`
}

type DownloadTransfer struct {
	TotalChunks  *int    `json:"total_chunks,omitempty"`
	ChunkSize    *int    `json:"chunk_size,omitempty"`
	ChunkData    *string `json:"chunk_data,omitempty"`
	ChunkNum     *int    `json:"chunk_num,omitempty"`
	FullPath     *string `json:"full_path,omitempty"`
	FileName     *string `json:"filename,omitempty"`
	FileID       *string `json:"file_id,omitempty"`
	Host         *string `json:"host,omitempty"`
	IsScreenshot *bool   `json:"is_screenshot,omitempty"`
}

type DownloadResponseMessage struct {
	TaskID   string            `json:"task_id"`
	Download *DownloadTransfer `json:"download,omitempty"`
}

type UploadTransfer struct {
	ChunkSize *int    `json:"chunk_size,omitempty"`
	ChunkNum  int     `json:"chunk_num"`
	FullPath  *string `json:"full_path,omitempty"`
	FileID    *string `json:"file_id,omitempty"`
	Host      *string `json:"host,omitempty"`
}

type UploadResponseMessage struct {
	TaskID string          `json:"task_id"`
	Upload *UploadTransfer `json:"upload,omitempty"`
}
