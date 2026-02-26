package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"vn.io.arda/notification/internal/domain"
)

// Repository is the PostgreSQL implementation of domain.Repository.
type Repository struct {
	pool *pgxpool.Pool
}

// New creates a new postgres Repository.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create inserts a new notification record.
func (r *Repository) Create(ctx context.Context, input domain.CreateNotificationInput) (*domain.Notification, error) {
	metaJSON, _ := json.Marshal(input.Metadata)

	var sourceEventID *string
	if input.SourceEventID != "" {
		sourceEventID = &input.SourceEventID
	}

	var n domain.Notification
	err := r.pool.QueryRow(ctx, `
		INSERT INTO notifications (tenant_key, user_id, type, title, body, metadata, source_event_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (source_event_id) WHERE source_event_id IS NOT NULL DO NOTHING
		RETURNING id, tenant_key, user_id, type, title, body, metadata, is_read, read_at, created_at, source_event_id
	`, input.TenantKey, input.UserID, string(input.Type), input.Title, input.Body, metaJSON, sourceEventID).
		Scan(&n.ID, &n.TenantKey, &n.UserID, &n.Type, &n.Title, &n.Body,
			&metaJSON, &n.IsRead, &n.ReadAt, &n.CreatedAt, &sourceEventID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Duplicate source_event_id, idempotent — not an error
			return nil, nil
		}
		return nil, fmt.Errorf("insert notification: %w", err)
	}

	if sourceEventID != nil {
		n.SourceEventID = *sourceEventID
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &n.Metadata)
	}

	return &n, nil
}

func (r *Repository) BatchCreate(ctx context.Context, inputs []domain.CreateNotificationInput) ([]*domain.Notification, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	// Build VALUES list: ($1,$2,...), ($9,$10,...) etc.
	// Each row has 7 params: tenant_key, user_id, type, title, body, metadata, source_event_id
	const paramsPerRow = 7
	args := make([]any, 0, len(inputs)*paramsPerRow)
	valuesClauses := make([]string, 0, len(inputs))

	for i, input := range inputs {
		base := i * paramsPerRow
		metaJSON, _ := json.Marshal(input.Metadata)
		var sourceEventID *string
		if input.SourceEventID != "" {
			sourceEventID = &input.SourceEventID
		}

		valuesClauses = append(valuesClauses, fmt.Sprintf(
			"($%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7,
		))
		args = append(args,
			input.TenantKey, input.UserID, string(input.Type),
			input.Title, input.Body, metaJSON, sourceEventID,
		)
	}

	// Join all value tuples into a single INSERT statement.
	query := "INSERT INTO notifications (tenant_key, user_id, type, title, body, metadata, source_event_id) VALUES " +
		joinStrings(valuesClauses, ",") +
		" ON CONFLICT (source_event_id) WHERE source_event_id IS NOT NULL DO NOTHING " +
		"RETURNING id, tenant_key, user_id, type, title, body, metadata, is_read, read_at, created_at, source_event_id"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch insert notifications query failed: %w", err)
	}
	defer rows.Close()

	var insertedResults []*domain.Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		insertedResults = append(insertedResults, n)
	}

	return insertedResults, nil
}

// joinStrings joins a slice of strings with a separator (avoids importing strings package).
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}



// List fetches paginated notifications for a user.
func (r *Repository) List(ctx context.Context, f domain.NotificationFilter) ([]*domain.Notification, error) {
	query := `
		SELECT id, tenant_key, user_id, type, title, body, metadata, is_read, read_at, created_at, source_event_id
		FROM notifications
		WHERE tenant_key = $1 AND user_id = $2
	`
	args := []any{f.TenantKey, f.UserID}
	paramIdx := 3

	if f.IsRead != nil {
		query += fmt.Sprintf(" AND is_read = $%d", paramIdx)
		args = append(args, *f.IsRead)
		paramIdx++
	}
	if f.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", paramIdx)
		args = append(args, string(f.Type))
		paramIdx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", paramIdx, paramIdx+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var results []*domain.Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, n)
	}
	return results, nil
}

// GetByID fetches a single notification.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_key, user_id, type, title, body, metadata, is_read, read_at, created_at, source_event_id
		FROM notifications WHERE id = $1
	`, id)
	return scanNotification(row)
}

// MarkRead marks a single notification as read.
func (r *Repository) MarkRead(ctx context.Context, id uuid.UUID, tenantKey, userID string) error {
	now := time.Now()
	tag, err := r.pool.Exec(ctx, `
		UPDATE notifications SET is_read = TRUE, read_at = $1
		WHERE id = $2 AND tenant_key = $3 AND user_id = $4 AND is_read = FALSE
	`, now, id, tenantKey, userID)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("notification not found or already read")
	}
	return nil
}

// MarkAllRead marks all unread notifications for a user as read.
func (r *Repository) MarkAllRead(ctx context.Context, tenantKey, userID string) (int64, error) {
	now := time.Now()
	tag, err := r.pool.Exec(ctx, `
		UPDATE notifications SET is_read = TRUE, read_at = $1
		WHERE tenant_key = $2 AND user_id = $3 AND is_read = FALSE
	`, now, tenantKey, userID)
	if err != nil {
		return 0, fmt.Errorf("mark all read: %w", err)
	}
	return tag.RowsAffected(), nil
}

// Delete removes a notification belonging to the user.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID, tenantKey, userID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM notifications WHERE id = $1 AND tenant_key = $2 AND user_id = $3
	`, id, tenantKey, userID)
	if err != nil {
		return fmt.Errorf("delete notification: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

// CountUnread returns the count of unread notifications for a user.
func (r *Repository) CountUnread(ctx context.Context, tenantKey, userID string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE tenant_key = $1 AND user_id = $2 AND is_read = FALSE`,
		tenantKey, userID,
	).Scan(&count)
	return count, err
}

// PurgeOlderThan deletes notifications older than the given number of days.
func (r *Repository) PurgeOlderThan(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM notifications WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge notifications: %w", err)
	}
	return tag.RowsAffected(), nil
}

// scanNotification is a helper to scan a row into a Notification struct.
type scannable interface {
	Scan(dest ...any) error
}

func scanNotification(row scannable) (*domain.Notification, error) {
	var n domain.Notification
	var metaJSON []byte
	var sourceEventID *string

	err := row.Scan(
		&n.ID, &n.TenantKey, &n.UserID, &n.Type, &n.Title, &n.Body,
		&metaJSON, &n.IsRead, &n.ReadAt, &n.CreatedAt, &sourceEventID,
	)
	if err != nil {
		return nil, fmt.Errorf("scan notification: %w", err)
	}
	if sourceEventID != nil {
		n.SourceEventID = *sourceEventID
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &n.Metadata)
	}
	return &n, nil
}
