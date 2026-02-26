package handlers

import (
	"encoding/json"

	"vn.io.arda/notification/internal/domain"
	"vn.io.arda/notification/internal/messages"
)

func init() {
	Register("bpm-events", "TASK_ASSIGNED", handleTaskAssigned)
	Register("bpm-events", "TASK_COMPLETED", handleTaskCompleted)
	Register("bpm-events", "APPROVAL_REQUIRED", handleApprovalRequired)
}

type bpmEnv struct {
	EventType string `json:"eventType"`
	EventID   string `json:"eventId"`
	TenantKey string `json:"tenantKey"`
	Payload   struct {
		TaskID      string `json:"taskId"`
		TaskName    string `json:"taskName"`
		AssigneeID  string `json:"assigneeId"`
		ProcessName string `json:"processName"`
	} `json:"payload"`
}

func parseBPMEnv(data []byte) (*bpmEnv, bool) {
	var env bpmEnv
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, false
	}
	if env.Payload.AssigneeID == "" {
		return nil, false
	}
	return &env, true
}

func handleTaskAssigned(data []byte) *domain.FanoutInput {
	env, ok := parseBPMEnv(data)
	if !ok {
		return nil
	}
	title, body := messages.TaskAssigned(env.Payload.TaskName, env.Payload.ProcessName)
	return &domain.FanoutInput{
		TargetScope:   domain.ScopeUser,
		TargetID:      env.Payload.AssigneeID,
		TenantKey:     env.TenantKey,
		Type:          domain.TypeWorkflow,
		Title:         title,
		Body:          body,
		Metadata:      map[string]any{"taskId": env.Payload.TaskID, "processName": env.Payload.ProcessName},
		SourceEventID: env.EventID,
	}
}

func handleTaskCompleted(data []byte) *domain.FanoutInput {
	env, ok := parseBPMEnv(data)
	if !ok {
		return nil
	}
	title, body := messages.TaskCompleted(env.Payload.TaskName)
	return &domain.FanoutInput{
		TargetScope:   domain.ScopeUser,
		TargetID:      env.Payload.AssigneeID,
		TenantKey:     env.TenantKey,
		Type:          domain.TypeWorkflow,
		Title:         title,
		Body:          body,
		Metadata:      map[string]any{"taskId": env.Payload.TaskID, "processName": env.Payload.ProcessName},
		SourceEventID: env.EventID,
	}
}

func handleApprovalRequired(data []byte) *domain.FanoutInput {
	env, ok := parseBPMEnv(data)
	if !ok {
		return nil
	}
	title, body := messages.ApprovalRequired(env.Payload.TaskName, env.Payload.ProcessName)
	return &domain.FanoutInput{
		TargetScope:   domain.ScopeUser,
		TargetID:      env.Payload.AssigneeID,
		TenantKey:     env.TenantKey,
		Type:          domain.TypeWorkflow,
		Title:         title,
		Body:          body,
		Metadata:      map[string]any{"taskId": env.Payload.TaskID, "processName": env.Payload.ProcessName},
		SourceEventID: env.EventID,
	}
}
