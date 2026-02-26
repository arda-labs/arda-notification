package handlers

import (
	"encoding/json"

	"vn.io.arda/notification/internal/domain"
	"vn.io.arda/notification/internal/messages"
)

func init() {
	Register("tenant-events", "TENANT_CREATED", handleTenantCreated)
	Register("tenant-events", "TENANT_UPDATED", handleTenantUpdated)
	Register("tenant-events", "TENANT_STATUS_UPDATED", handleTenantStatusUpdated)
	Register("tenant-events", "TENANT_DELETED", handleTenantDeleted)
}

type tenantEnv struct {
	EventType   string `json:"eventType"`
	EventID     string `json:"eventId"`
	TenantKey   string `json:"tenantKey"`
	DisplayName string `json:"displayName"`
	DbType      string `json:"dbType"`
	Status      string `json:"status"`
	CreatedBy   string `json:"createdBy"`
}

func parseTenantEnv(data []byte) (*tenantEnv, bool) {
	var env tenantEnv
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, false
	}
	return &env, true
}

func tenantFanout(env *tenantEnv, title, body string) *domain.FanoutInput {
	return &domain.FanoutInput{
		TargetScope:   domain.ScopeRole,
		TargetID:      "PLATFORM_ADMIN",
		TenantKey:     "master",
		Type:          domain.TypeSystem,
		Title:         title,
		Body:          body,
		Metadata:      map[string]any{"eventType": env.EventType, "tenantKey": env.TenantKey},
		SourceEventID: env.EventID,
		OriginUserID:  env.CreatedBy,
	}
}

func handleTenantCreated(data []byte) *domain.FanoutInput {
	env, ok := parseTenantEnv(data)
	if !ok {
		return nil
	}
	displayName := env.DisplayName
	if displayName == "" {
		displayName = env.TenantKey
	}
	title, body := messages.TenantCreated(displayName, env.DbType)
	return tenantFanout(env, title, body)
}

func handleTenantUpdated(data []byte) *domain.FanoutInput {
	env, ok := parseTenantEnv(data)
	if !ok {
		return nil
	}
	displayName := env.DisplayName
	if displayName == "" {
		displayName = env.TenantKey
	}
	title, body := messages.TenantUpdated(displayName)
	return tenantFanout(env, title, body)
}

func handleTenantStatusUpdated(data []byte) *domain.FanoutInput {
	env, ok := parseTenantEnv(data)
	if !ok {
		return nil
	}
	title, body := messages.TenantStatusUpdated(env.TenantKey, env.Status)
	return tenantFanout(env, title, body)
}

func handleTenantDeleted(data []byte) *domain.FanoutInput {
	env, ok := parseTenantEnv(data)
	if !ok {
		return nil
	}
	title, body := messages.TenantDeleted(env.TenantKey)
	return tenantFanout(env, title, body)
}
