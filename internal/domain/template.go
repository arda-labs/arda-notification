package domain

import (
	"context"
	"time"
)

// Template stores a localised notification message template.
type Template struct {
	ID             string    `json:"id"`
	TemplateKey    string    `json:"template_key"`
	Locale         string    `json:"locale"`
	TitleTemplate  string    `json:"title_template"`
	BodyTemplate   string    `json:"body_template"`
	CreatedAt      time.Time `json:"created_at"`
}

// TemplateRepository defines the port for template persistence.
type TemplateRepository interface {
	// Get returns the template for a key and locale.
	// Returns nil when no row exists.
	Get(ctx context.Context, key, locale string) (*Template, error)

	// List returns all templates for a locale.
	List(ctx context.Context, locale string) ([]Template, error)

	// Upsert inserts or updates a template.
	Upsert(ctx context.Context, t Template) (*Template, error)

	// Delete removes a template.
	Delete(ctx context.Context, key, locale string) error
}
