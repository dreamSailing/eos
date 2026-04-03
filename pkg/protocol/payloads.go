package protocol

func ApprovalRequestPayload(v ApprovalRequest) map[string]any {
	payload := map[string]any{
		"approval_id": v.ApprovalID,
		"message":     v.Message,
	}
	if v.Title != "" {
		payload["title"] = v.Title
	}
	if v.RiskLevel != "" {
		payload["risk_level"] = v.RiskLevel
	}
	if len(v.Options) > 0 {
		payload["options"] = append([]string(nil), v.Options...)
	}
	return payload
}

func ApprovalResolutionPayload(v ApprovalResolution) map[string]any {
	payload := map[string]any{
		"approval_id": v.ApprovalID,
		"decision":    v.Decision,
	}
	if v.Reason != "" {
		payload["reason"] = v.Reason
	}
	return payload
}

func InquiryRequestPayload(v InquiryRequest) map[string]any {
	payload := map[string]any{
		"inquiry_id": v.InquiryID,
		"question":   v.Question,
	}
	if len(v.Options) > 0 {
		payload["options"] = append([]string(nil), v.Options...)
	}
	if v.AllowText {
		payload["allow_text"] = true
	}
	return payload
}

func InquiryResolutionPayload(v InquiryResolution) map[string]any {
	payload := map[string]any{
		"inquiry_id": v.InquiryID,
	}
	if v.Option != "" {
		payload["option"] = v.Option
	}
	if v.Text != "" {
		payload["text"] = v.Text
	}
	return payload
}

func SessionPayload(v SessionInfo) map[string]any {
	payload := map[string]any{
		"session_id": v.SessionID,
	}
	if v.ThreadID != "" {
		payload["thread_id"] = v.ThreadID
	}
	if v.Workspace != "" {
		payload["workspace"] = v.Workspace
	}
	if v.Title != "" {
		payload["title"] = v.Title
	}
	if v.Preview != "" {
		payload["preview"] = v.Preview
	}
	if v.Mode != "" {
		payload["mode"] = v.Mode
	}
	if v.Status != "" {
		payload["status"] = v.Status
	}
	if v.CurrentRequestID != "" {
		payload["current_request_id"] = v.CurrentRequestID
	}
	if len(v.PendingApprovals) > 0 {
		payload["pending_approvals"] = append([]string(nil), v.PendingApprovals...)
	}
	if len(v.PendingInquiries) > 0 {
		payload["pending_inquiries"] = append([]string(nil), v.PendingInquiries...)
	}
	if len(v.RunningTasks) > 0 {
		payload["running_tasks"] = append([]string(nil), v.RunningTasks...)
	}
	if !v.UpdatedAt.IsZero() {
		payload["updated_at"] = v.UpdatedAt
	}
	if len(v.Metadata) > 0 {
		payload["metadata"] = ClonePayload(v.Metadata)
	}
	return payload
}

func TaskPayload(v TaskInfo) map[string]any {
	payload := map[string]any{
		"task_id": v.TaskID,
	}
	if v.Kind != "" {
		payload["kind"] = v.Kind
	}
	if v.Status != "" {
		payload["status"] = v.Status
	}
	if v.Label != "" {
		payload["label"] = v.Label
	}
	if v.CanCancel {
		payload["can_cancel"] = true
	}
	if !v.StartedAt.IsZero() {
		payload["started_at"] = v.StartedAt
	}
	if !v.FinishedAt.IsZero() {
		payload["finished_at"] = v.FinishedAt
	}
	return payload
}

func ToolCallPayload(v ToolCall) map[string]any {
	payload := map[string]any{
		"tool_name": v.ToolName,
	}
	if len(v.Arguments) > 0 {
		payload["arguments"] = ClonePayload(v.Arguments)
	}
	if v.RiskLevel != "" {
		payload["risk_level"] = v.RiskLevel
	}
	return payload
}

func ToolResultPayload(v ToolResult) map[string]any {
	payload := map[string]any{
		"tool_name": v.ToolName,
	}
	if v.Status != "" {
		payload["status"] = v.Status
	}
	if v.Display != "" {
		payload["display"] = v.Display
	}
	if len(v.Data) > 0 {
		payload["data"] = ClonePayload(v.Data)
	}
	return payload
}

func TextPayloadMap(v TextPayload) map[string]any {
	payload := map[string]any{
		"text": v.Text,
	}
	if v.CollapsedDefault {
		payload["collapsed_default"] = true
	}
	return payload
}
