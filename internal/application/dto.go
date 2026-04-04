package application

import "vn.io.arda/notification/internal/domain"

// NotificationInput is the DTO used by Kafka handlers to create notifications.
// This is a type alias for domain.CreateNotificationInput for convenience.
type NotificationInput = domain.CreateNotificationInput

// PreferenceUpdateInput is the DTO for batch upserting preferences.
type PreferenceUpdateInput struct {
	Type            string  `json:"type"`
	ChannelInApp    *bool   `json:"channel_in_app,omitempty"`
	ChannelEmail    *bool   `json:"channel_email,omitempty"`
	QuietHoursStart *string `json:"quiet_hours_start,omitempty"`
	QuietHoursEnd   *string `json:"quiet_hours_end,omitempty"`
}
