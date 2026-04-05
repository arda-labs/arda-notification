package application

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"
	"vn.io.arda/notification/internal/domain"
)

// TemplateEngine renders notification messages from database templates.
// Falls back to the provided defaults when no template is found.
type TemplateEngine struct {
	repo          domain.TemplateRepository
	defaultLocale string
}

// NewTemplateEngine creates a new TemplateEngine.
func NewTemplateEngine(repo domain.TemplateRepository, defaultLocale string) *TemplateEngine {
	return &TemplateEngine{repo: repo, defaultLocale: defaultLocale}
}

// Render resolves a template by key and locale, then substitutes variables.
func (e *TemplateEngine) Render(ctx context.Context, key, locale string, vars map[string]string, fallbackTitle, fallbackBody string) (string, string, error) {
	tmpl, err := e.repo.Get(ctx, key, locale)
	if err != nil {
		log.Warn().Err(err).Str("key", key).Msg("template lookup failed, using fallback")
		return e.sub(fallbackTitle, vars), e.sub(fallbackBody, vars), nil
	}
	if tmpl == nil && locale != e.defaultLocale {
		tmpl, err = e.repo.Get(ctx, key, e.defaultLocale)
		if err != nil {
			return e.sub(fallbackTitle, vars), e.sub(fallbackBody, vars), nil
		}
	}
	if tmpl == nil {
		return e.sub(fallbackTitle, vars), e.sub(fallbackBody, vars), nil
	}
	return e.sub(tmpl.TitleTemplate, vars), e.sub(tmpl.BodyTemplate, vars), nil
}

// substitute replaces {{variable}} placeholders with values.
func (e *TemplateEngine) sub(tmpl string, vars map[string]string) string {
	result := tmpl
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}

// GetTemplates returns all templates for a locale.
func (e *TemplateEngine) GetTemplates(ctx context.Context, locale string) ([]domain.Template, error) {
	return e.repo.List(ctx, locale)
}

// UpsertTemplate creates or updates a template.
func (e *TemplateEngine) UpsertTemplate(ctx context.Context, t domain.Template) (*domain.Template, error) {
	return e.repo.Upsert(ctx, t)
}

// DeleteTemplate removes a template.
func (e *TemplateEngine) DeleteTemplate(ctx context.Context, key, locale string) error {
	return e.repo.Delete(ctx, key, locale)
}
