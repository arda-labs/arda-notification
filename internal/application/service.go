package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"vn.io.arda/notification/internal/domain"
)

// Service holds all notification use-cases.
type Service struct {
	repo     domain.Repository
	hub      SSEHub
	resolver IAMResolver
}

// SSEHub is the interface for broadcasting to connected SSE clients.
// Implementation lives in transport/http/sse_hub.go.
type SSEHub interface {
	Broadcast(tenantKey, userID string, notification *domain.Notification)
}

// NewService creates a new application Service.
func NewService(repo domain.Repository, hub SSEHub, resolver IAMResolver) *Service {
	return &Service{repo: repo, hub: hub, resolver: resolver}
}

// Create processes a single notification (from direct API calls or USER-scoped Kafka events),
// persists it, and broadcasts via SSE if the user is connected.
func (s *Service) Create(ctx context.Context, input domain.CreateNotificationInput) (*domain.Notification, error) {
	n, err := s.repo.Create(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create notification: %w", err)
	}
	if n == nil {
		// Duplicate source_event_id — idempotent, not an error.
		return nil, nil
	}

	// Non-blocking SSE broadcast
	go s.hub.Broadcast(n.TenantKey, n.UserID, n)

	log.Info().
		Str("id", n.ID.String()).
		Str("tenant", n.TenantKey).
		Str("user", n.UserID).
		Str("type", string(n.Type)).
		Msg("notification created and broadcast")

	return n, nil
}

// Fanout resolves a FanoutInput to concrete user IDs based on TargetScope,
// then batch-inserts one notification row per user (fan-out on write).
// This is the primary entry point for Kafka-driven notifications.
func (s *Service) Fanout(ctx context.Context, input domain.FanoutInput) error {
	// Resolve target scope to (tenantKey → []userID) map.
	usersByTenant, err := s.resolveTargets(ctx, input)
	if err != nil {
		return fmt.Errorf("resolve fan-out targets: %w", err)
	}

	// Build one CreateNotificationInput per user.
	var batch []domain.CreateNotificationInput
	for tenantKey, userIDs := range usersByTenant {
		for _, uid := range userIDs {
			batch = append(batch, domain.CreateNotificationInput{
				TenantKey:     tenantKey,
				UserID:        uid,
				Type:          input.Type,
				Title:         input.Title,
				Body:          input.Body,
				Metadata:      input.Metadata,
				SourceEventID: input.SourceEventID,
			})
		}
	}

	if len(batch) == 0 {
		log.Warn().
			Str("scope", string(input.TargetScope)).
			Str("target_id", input.TargetID).
			Msg("fan-out resolved to zero users, skipping")
		return nil
	}

	insertedResults, err := s.repo.BatchCreate(ctx, batch)
	if err != nil {
		return fmt.Errorf("batch create notifications: %w", err)
	}

	for _, n := range insertedResults {
		// Non-blocking SSE broadcast
		go s.hub.Broadcast(n.TenantKey, n.UserID, n)
	}

	log.Info().
		Str("scope", string(input.TargetScope)).
		Str("target_id", input.TargetID).
		Int("batch_size", len(batch)).
		Int("inserted", len(insertedResults)).
		Msg("fan-out notifications created and broadcasted")

	return nil
}

// resolveTargets maps a FanoutInput to (tenantKey → []userID) using the IAMResolver.
func (s *Service) resolveTargets(ctx context.Context, input domain.FanoutInput) (map[string][]string, error) {
	result := make(map[string][]string)

	switch input.TargetScope {
	case domain.ScopeUser:
		// Direct single-user delivery — no IAM call needed.
		result[input.TenantKey] = []string{input.TargetID}

	case domain.ScopeTenant:
		userIDs, err := s.resolver.UsersByTenant(ctx, input.TenantKey)
		if err != nil {
			return nil, fmt.Errorf("UsersByTenant(%s): %w", input.TenantKey, err)
		}
		result[input.TenantKey] = userIDs

	case domain.ScopeRole:
		userIDs, err := s.resolver.UsersByRole(ctx, input.TenantKey, input.TargetID)
		if err != nil {
			return nil, fmt.Errorf("UsersByRole(%s, %s): %w", input.TenantKey, input.TargetID, err)
		}
		result[input.TenantKey] = userIDs

	case domain.ScopePlatform:
		all, err := s.resolver.AllActiveUsers(ctx)
		if err != nil {
			return nil, fmt.Errorf("AllActiveUsers: %w", err)
		}
		for tk, uids := range all {
			result[tk] = uids
		}

	default:
		return nil, fmt.Errorf("unknown target scope: %q", input.TargetScope)
	}

	// Post-resolution: Always include the performer (OriginUserID) in the fan-out
	// if they are not already in the target set.
	if input.OriginUserID != "" {
		found := false
		for _, uids := range result {
			for _, uid := range uids {
				if uid == input.OriginUserID {
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			log.Debug().Str("user", input.OriginUserID).Msg("adding origin user to fan-out targets")
			// Add to the action's tenant or "master" by default for admins.
			tenant := input.TenantKey
			if tenant == "" {
				tenant = "master"
			}
			result[tenant] = append(result[tenant], input.OriginUserID)
		}
	}

	return result, nil
}

// List returns paginated notifications for a user.
func (s *Service) List(ctx context.Context, filter domain.NotificationFilter) ([]*domain.Notification, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 20
	}
	return s.repo.List(ctx, filter)
}

// CountUnread returns the unread badge count for a user.
func (s *Service) CountUnread(ctx context.Context, tenantKey, userID string) (int64, error) {
	return s.repo.CountUnread(ctx, tenantKey, userID)
}

// MarkRead marks a single notification as read.
func (s *Service) MarkRead(ctx context.Context, idStr, tenantKey, userID string) error {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return fmt.Errorf("invalid notification id: %w", err)
	}
	return s.repo.MarkRead(ctx, id, tenantKey, userID)
}

// MarkAllRead marks all notifications for a user as read.
func (s *Service) MarkAllRead(ctx context.Context, tenantKey, userID string) (int64, error) {
	return s.repo.MarkAllRead(ctx, tenantKey, userID)
}

// Delete removes a notification (must belong to the requesting user).
func (s *Service) Delete(ctx context.Context, idStr, tenantKey, userID string) error {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return fmt.Errorf("invalid notification id: %w", err)
	}
	return s.repo.Delete(ctx, id, tenantKey, userID)
}

// PurgeTTL deletes old notifications. Called by a background scheduler.
func (s *Service) PurgeTTL(ctx context.Context, days int) {
	count, err := s.repo.PurgeOlderThan(ctx, days)
	if err != nil {
		log.Error().Err(err).Msg("notification TTL purge failed")
		return
	}
	log.Info().Int64("deleted", count).Int("older_than_days", days).Msg("notification TTL purge completed")
}
