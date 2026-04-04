package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"vn.io.arda/notification/internal/domain"
)

// PreferenceRepo implements domain.PreferenceRepository.
type PreferenceRepo struct {
	pool *pgxpool.Pool
}

// NewPreferenceRepo creates a new PreferenceRepo.
func NewPreferenceRepo(pool *pgxpool.Pool) *PreferenceRepo {
	return &PreferenceRepo{pool: pool}
}

func (r *PreferenceRepo) GetByUser(ctx context.Context, tenantKey, userID string) ([]domain.Preference, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_key, user_id, type, channel_in_app, channel_email,
		       quiet_hours_start, quiet_hours_end, created_at, updated_at
		FROM notification_preferences
		WHERE tenant_key = $1 AND user_id = $2
		ORDER BY type
	`, tenantKey, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.Preference
	for rows.Next() {
		p, err := scanPreference(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *p)
	}
	return results, nil
}

func (r *PreferenceRepo) GetByUserAndType(ctx context.Context, tenantKey, userID string, notifType domain.NotificationType) (*domain.Preference, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_key, user_id, type, channel_in_app, channel_email,
		       quiet_hours_start, quiet_hours_end, created_at, updated_at
		FROM notification_preferences
		WHERE tenant_key = $1 AND user_id = $2 AND type = $3
	`, tenantKey, userID, string(notifType))

	p, err := scanPreference(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

func (r *PreferenceRepo) Upsert(ctx context.Context, p domain.Preference) (*domain.Preference, error) {
	now := time.Now()
	var idStr string
	if p.ID == "" {
		idStr = uuid.New().String()
	} else {
		idStr = p.ID
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO notification_preferences (id, tenant_key, user_id, type, channel_in_app, channel_email,
		                                      quiet_hours_start, quiet_hours_end, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (tenant_key, user_id, type) DO UPDATE SET
			channel_in_app = EXCLUDED.channel_in_app,
			channel_email  = EXCLUDED.channel_email,
			quiet_hours_start = EXCLUDED.quiet_hours_start,
			quiet_hours_end   = EXCLUDED.quiet_hours_end,
			updated_at = EXCLUDED.updated_at
		RETURNING id, tenant_key, user_id, type, channel_in_app, channel_email,
		          quiet_hours_start, quiet_hours_end, created_at, updated_at
	`, idStr, p.TenantKey, p.UserID, string(p.Type), p.ChannelInApp, p.ChannelEmail,
		p.QuietHoursStart, p.QuietHoursEnd, now, now)

	return scanPreference(row)
}

func (r *PreferenceRepo) BatchUpsert(ctx context.Context, prefs []domain.Preference) ([]domain.Preference, error) {
	if len(prefs) == 0 {
		return nil, nil
	}

	// Use a transaction for batch atomicity.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var results []domain.Preference
	for _, p := range prefs {
		now := time.Now()
		idStr := uuid.New().String()
		if p.ID != "" {
			idStr = p.ID
		}

		row := tx.QueryRow(ctx, `
			INSERT INTO notification_preferences (id, tenant_key, user_id, type, channel_in_app, channel_email,
			                                      quiet_hours_start, quiet_hours_end, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (tenant_key, user_id, type) DO UPDATE SET
				channel_in_app = EXCLUDED.channel_in_app,
				channel_email  = EXCLUDED.channel_email,
				quiet_hours_start = EXCLUDED.quiet_hours_start,
				quiet_hours_end   = EXCLUDED.quiet_hours_end,
				updated_at = EXCLUDED.updated_at
			RETURNING id, tenant_key, user_id, type, channel_in_app, channel_email,
			          quiet_hours_start, quiet_hours_end, created_at, updated_at
		`, idStr, p.TenantKey, p.UserID, string(p.Type), p.ChannelInApp, p.ChannelEmail,
			p.QuietHoursStart, p.QuietHoursEnd, now, now)

		saved, err := scanPreference(row)
		if err != nil {
			return nil, err
		}
		results = append(results, *saved)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return results, nil
}

// scanPreference scans a row into a Preference struct.
func scanPreference(row scannable) (*domain.Preference, error) {
	var p domain.Preference
	err := row.Scan(
		&p.ID, &p.TenantKey, &p.UserID, &p.Type,
		&p.ChannelInApp, &p.ChannelEmail,
		&p.QuietHoursStart, &p.QuietHoursEnd,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Suppress unused import warning — json is used by other files in this package.
var _ = json.Marshal
