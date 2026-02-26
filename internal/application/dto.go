package application

import "vn.io.arda/notification/internal/domain"

// NotificationInput is the DTO used by Kafka handlers to create notifications.
// This is a type alias for domain.CreateNotificationInput for convenience.
type NotificationInput = domain.CreateNotificationInput
