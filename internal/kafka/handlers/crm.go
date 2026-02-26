package handlers

import (
	"encoding/json"

	"vn.io.arda/notification/internal/domain"
	"vn.io.arda/notification/internal/messages"
)

func init() {
	Register("crm-events", "LEAD_STATUS_CHANGED", handleLeadStatusChanged)
	Register("crm-events", "DEAL_UPDATED", handleDealUpdated)
}

type crmEnv struct {
	EventType string `json:"eventType"`
	EventID   string `json:"eventId"`
	TenantKey string `json:"tenantKey"`
	Payload   struct {
		EntityID   string `json:"entityId"`
		EntityName string `json:"entityName"`
		OwnerID    string `json:"ownerId"`
	} `json:"payload"`
}

func parseCRMEnv(data []byte) (*crmEnv, bool) {
	var env crmEnv
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, false
	}
	if env.Payload.OwnerID == "" {
		return nil, false
	}
	return &env, true
}

func handleLeadStatusChanged(data []byte) *domain.FanoutInput {
	env, ok := parseCRMEnv(data)
	if !ok {
		return nil
	}
	title, body := messages.LeadStatusChanged(env.Payload.EntityName)
	return &domain.FanoutInput{
		TargetScope:   domain.ScopeUser,
		TargetID:      env.Payload.OwnerID,
		TenantKey:     env.TenantKey,
		Type:          domain.TypeCRM,
		Title:         title,
		Body:          body,
		Metadata:      map[string]any{"entityId": env.Payload.EntityID},
		SourceEventID: env.EventID,
	}
}

func handleDealUpdated(data []byte) *domain.FanoutInput {
	env, ok := parseCRMEnv(data)
	if !ok {
		return nil
	}
	title, body := messages.DealUpdated(env.Payload.EntityName)
	return &domain.FanoutInput{
		TargetScope:   domain.ScopeUser,
		TargetID:      env.Payload.OwnerID,
		TenantKey:     env.TenantKey,
		Type:          domain.TypeCRM,
		Title:         title,
		Body:          body,
		Metadata:      map[string]any{"entityId": env.Payload.EntityID},
		SourceEventID: env.EventID,
	}
}
