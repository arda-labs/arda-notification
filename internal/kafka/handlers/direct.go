package handlers

import (
	"encoding/json"

	"vn.io.arda/notification/internal/domain"
)

func init() {
	RegisterDirect("notification-commands", handleDirectCommand)
}

func handleDirectCommand(data []byte) *domain.FanoutInput {
	var cmd struct {
		CommandID   string         `json:"commandId"`
		TenantKey   string         `json:"tenantKey"`
		TargetScope string         `json:"targetScope"`
		TargetID    string         `json:"targetId"`
		Type        string         `json:"type"`
		Title       string         `json:"title"`
		Body        string         `json:"body"`
		Metadata    map[string]any `json:"metadata"`
	}

	if err := json.Unmarshal(data, &cmd); err != nil {
		return nil
	}

	notifType := domain.NotificationType(cmd.Type)
	switch notifType {
	case domain.TypeSystem, domain.TypeWorkflow, domain.TypeCRM, domain.TypeIAM, domain.TypeCustom:
	default:
		notifType = domain.TypeCustom
	}

	scope := domain.TargetScope(cmd.TargetScope)
	switch scope {
	case domain.ScopeUser, domain.ScopeTenant, domain.ScopePlatform, domain.ScopeRole:
	default:
		if cmd.TargetID != "" {
			scope = domain.ScopeUser
		} else {
			return nil
		}
	}

	return &domain.FanoutInput{
		TargetScope:   scope,
		TargetID:      cmd.TargetID,
		TenantKey:     cmd.TenantKey,
		Type:          notifType,
		Title:         cmd.Title,
		Body:          cmd.Body,
		Metadata:      cmd.Metadata,
		SourceEventID: cmd.CommandID,
	}
}
