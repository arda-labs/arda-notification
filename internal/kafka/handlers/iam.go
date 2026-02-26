package handlers

import (
	"encoding/json"

	"vn.io.arda/notification/internal/domain"
	"vn.io.arda/notification/internal/messages"
)

func init() {
	Register("iam-events", "LOGIN_NEW_DEVICE", handleLoginNewDevice)
	Register("iam-events", "PASSWORD_CHANGED", handlePasswordChanged)
}

type iamEnv struct {
	EventType string `json:"eventType"`
	EventID   string `json:"eventId"`
	TenantKey string `json:"tenantKey"`
	Payload   struct {
		UserID string `json:"userId"`
		IP     string `json:"ip"`
		Detail string `json:"detail"`
	} `json:"payload"`
}

func parseIAMEnv(data []byte) (*iamEnv, bool) {
	var env iamEnv
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, false
	}
	return &env, true
}

func handleLoginNewDevice(data []byte) *domain.FanoutInput {
	env, ok := parseIAMEnv(data)
	if !ok {
		return nil
	}
	title, body := messages.LoginNewDevice(env.Payload.IP)
	return &domain.FanoutInput{
		TargetScope:   domain.ScopeUser,
		TargetID:      env.Payload.UserID,
		TenantKey:     env.TenantKey,
		Type:          domain.TypeIAM,
		Title:         title,
		Body:          body,
		Metadata:      map[string]any{"ip": env.Payload.IP, "detail": env.Payload.Detail},
		SourceEventID: env.EventID,
	}
}

func handlePasswordChanged(data []byte) *domain.FanoutInput {
	env, ok := parseIAMEnv(data)
	if !ok {
		return nil
	}
	title, body := messages.PasswordChanged()
	return &domain.FanoutInput{
		TargetScope:   domain.ScopeUser,
		TargetID:      env.Payload.UserID,
		TenantKey:     env.TenantKey,
		Type:          domain.TypeIAM,
		Title:         title,
		Body:          body,
		Metadata:      map[string]any{"ip": env.Payload.IP, "detail": env.Payload.Detail},
		SourceEventID: env.EventID,
	}
}
