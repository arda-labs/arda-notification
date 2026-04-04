package domain

import (
	"context"
	"fmt"
	"time"
)

// Preference stores a user's channel settings for one notification type.
type Preference struct {
	ID              string    `json:"id"`
	TenantKey       string    `json:"tenant_key"`
	UserID          string    `json:"user_id"`
	Type            NotificationType `json:"type"`
	ChannelInApp    bool      `json:"channel_in_app"`
	ChannelEmail    bool      `json:"channel_email"`
	QuietHoursStart *string   `json:"quiet_hours_start,omitempty"` // "22:00"
	QuietHoursEnd   *string   `json:"quiet_hours_end,omitempty"`   // "08:00"
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// PreferenceInput is the DTO for upsert operations.
type PreferenceInput struct {
	Type            NotificationType `json:"type"`
	ChannelInApp    *bool            `json:"channel_in_app,omitempty"`
	ChannelEmail    *bool            `json:"channel_email,omitempty"`
	QuietHoursStart *string          `json:"quiet_hours_start,omitempty"`
	QuietHoursEnd   *string          `json:"quiet_hours_end,omitempty"`
}

// IsQuiet checks whether the given time falls within the quiet hours window.
// Returns false if quiet hours are not configured.
func (p *Preference) IsQuiet(t time.Time) bool {
	if p.QuietHoursStart == nil || p.QuietHoursEnd == nil {
		return false
	}
	// Parse HH:MM strings and compare only the clock portion.
	start, errS := parseTimeOfDay(*p.QuietHoursStart)
	end, errE := parseTimeOfDay(*p.QuietHoursEnd)
	if errS != nil || errE != nil {
		return false
	}
	hour, min, _ := t.Clock()
	now := hour*60 + min
	if start <= end {
		return now >= start && now <= end
	}
	// Overnight window, e.g. 22:00 – 06:00
	return now >= start || now <= end
}

func parseTimeOfDay(s string) (int, error) {
	// Expects "HH:MM" format.
	var h, m int
	_, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil {
		return 0, err
	}
	return h*60 + m, nil
}

// PreferenceRepository defines the port for preference persistence.
type PreferenceRepository interface {
	// GetByUser returns all preferences for a user within a tenant.
	GetByUser(ctx context.Context, tenantKey, userID string) ([]Preference, error)

	// GetByUserAndType returns the preference for a specific notification type.
	// Returns nil (not error) when no row exists.
	GetByUserAndType(ctx context.Context, tenantKey, userID string, notifType NotificationType) (*Preference, error)

	// Upsert inserts or updates a preference. Returns the saved entity.
	Upsert(ctx context.Context, p Preference) (*Preference, error)

	// BatchUpsert inserts or updates multiple preferences in one operation.
	BatchUpsert(ctx context.Context, prefs []Preference) ([]Preference, error)
}
