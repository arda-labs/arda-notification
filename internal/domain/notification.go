package domain

import (
	"time"

	"github.com/google/uuid"
)

// NotificationType represents the origin domain of the notification.
type NotificationType string

const (
	TypeSystem   NotificationType = "SYSTEM"
	TypeWorkflow NotificationType = "WORKFLOW"
	TypeCRM      NotificationType = "CRM"
	TypeIAM      NotificationType = "IAM"
	TypeCustom   NotificationType = "CUSTOM"
)

// TargetScope defines who should receive the notification (before fan-out).
type TargetScope string

const (
	// ScopeUser targets a single Keycloak user ID directly.
	ScopeUser TargetScope = "USER"
	// ScopeTenant fans-out to all active users within a tenant realm.
	ScopeTenant TargetScope = "TENANT"
	// ScopePlatform fans-out to all active users across all tenants.
	ScopePlatform TargetScope = "PLATFORM"
	// ScopeRole fans-out to all users holding a given role within a tenant.
	ScopeRole TargetScope = "ROLE"
)

// Notification is the core domain entity.
type Notification struct {
	ID            uuid.UUID        `json:"id"`
	TenantKey     string           `json:"tenant_key"`
	UserID        string           `json:"user_id"`
	Type          NotificationType `json:"type"`
	Title         string           `json:"title"`
	Body          string           `json:"body"`
	Metadata      map[string]any   `json:"metadata,omitempty"`
	IsRead        bool             `json:"is_read"`
	ReadAt        *time.Time       `json:"read_at,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	SourceEventID string           `json:"source_event_id,omitempty"`
}

// NotificationFilter holds query parameters for listing notifications.
type NotificationFilter struct {
	TenantKey string
	UserID    string
	IsRead    *bool
	Type      NotificationType
	Limit     int
	Offset    int
}

// CreateNotificationInput is the post-fan-out DTO — always has a concrete user_id.
// Used by Repository.Create / Repository.BatchCreate.
type CreateNotificationInput struct {
	TenantKey     string
	UserID        string
	Type          NotificationType
	Title         string
	Body          string
	Metadata      map[string]any
	SourceEventID string
}

// FanoutInput is the pre-fan-out DTO produced by Kafka handlers.
// The application Service resolves TargetScope → concrete user IDs,
// then batch-inserts CreateNotificationInput rows.
type FanoutInput struct {
	// TargetScope determines the resolution strategy.
	TargetScope TargetScope
	// TargetID is the userID (USER), tenantKey (TENANT), or roleName (ROLE).
	// Empty for PLATFORM scope.
	TargetID      string
	TenantKey     string
	Type          NotificationType
	Title         string
	Body          string
	Metadata      map[string]any
	SourceEventID string
	// OriginUserID is the ID of the user who performed the action.
	// We use this to ensure the performer also receives the notification.
	OriginUserID string
}
