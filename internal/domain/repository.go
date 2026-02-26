package domain

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the port for notification persistence.
// Implementations live in infrastructure/postgres.
type Repository interface {
	// Create stores a new notification and returns the saved entity.
	Create(ctx context.Context, input CreateNotificationInput) (*Notification, error)

	// BatchCreate inserts multiple notifications in a single operation (used by fan-out).
	// Returns the successfully inserted notifications.
	BatchCreate(ctx context.Context, inputs []CreateNotificationInput) ([]*Notification, error)

	// List fetches notifications matching the given filter.
	List(ctx context.Context, filter NotificationFilter) ([]*Notification, error)

	// GetByID fetches a single notification by its ID.
	GetByID(ctx context.Context, id uuid.UUID) (*Notification, error)

	// MarkRead marks a single notification as read.
	MarkRead(ctx context.Context, id uuid.UUID, tenantKey, userID string) error

	// MarkAllRead marks all unread notifications for a user as read.
	MarkAllRead(ctx context.Context, tenantKey, userID string) (int64, error)

	// Delete removes a notification (soft or hard delete).
	Delete(ctx context.Context, id uuid.UUID, tenantKey, userID string) error

	// CountUnread returns the number of unread notifications for a user.
	CountUnread(ctx context.Context, tenantKey, userID string) (int64, error)

	// PurgeOlderThan deletes notifications older than the specified duration (TTL cleanup).
	PurgeOlderThan(ctx context.Context, days int) (int64, error)
}
