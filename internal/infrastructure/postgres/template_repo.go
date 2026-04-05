package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"vn.io.arda/notification/internal/domain"
)

// TemplateRepo implements domain.TemplateRepository.
type TemplateRepo struct {
	pool *pgxpool.Pool
}

// NewTemplateRepo creates a new TemplateRepo.
func NewTemplateRepo(pool *pgxpool.Pool) *TemplateRepo {
	return &TemplateRepo{pool: pool}
}

func (r *TemplateRepo) Get(ctx context.Context, key, locale string) (*domain.Template, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT template_key, locale, title_template, body_template, created_at
		FROM notification_templates
		WHERE template_key = $1 AND locale = $2
	`, key, locale)
	return scanTemplate(row)
}

func (r *TemplateRepo) List(ctx context.Context, locale string) ([]domain.Template, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT template_key, locale, title_template, body_template, created_at
		FROM notification_templates
		WHERE locale = $1
		ORDER BY template_key
	`, locale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *t)
	}
	return results, nil
}

func (r *TemplateRepo) Upsert(ctx context.Context, t domain.Template) (*domain.Template, error) {
	now := time.Now()

	row := r.pool.QueryRow(ctx, `
		INSERT INTO notification_templates (template_key, locale, title_template, body_template, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (template_key, locale) DO UPDATE SET
			title_template = EXCLUDED.title_template,
			body_template  = EXCLUDED.body_template
		RETURNING template_key, locale, title_template, body_template, created_at
	`, t.TemplateKey, t.Locale, t.TitleTemplate, t.BodyTemplate, now)
	return scanTemplate(row)
}

func (r *TemplateRepo) Delete(ctx context.Context, key, locale string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM notification_templates WHERE template_key = $1 AND locale = $2`, key, locale)
	return err
}

func scanTemplate(row scannable) (*domain.Template, error) {
	var t domain.Template
	err := row.Scan(&t.TemplateKey, &t.Locale, &t.TitleTemplate, &t.BodyTemplate, &t.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// Suppress unused import warnings.
var _ = uuid.New
