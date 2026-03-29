package agentfunctions

type sessionLinkRecord struct {
	SessionLinkID           string `json:"session_link_id"`
	ParentCallbackID        int    `json:"parent_callback_id"`
	ParentCallbackDisplayID int    `json:"parent_callback_display_id"`
	ParentTaskID            int    `json:"parent_task_id"`
	CreatedAt               string `json:"created_at"`
}
