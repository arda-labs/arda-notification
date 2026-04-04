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
	repo        domain.Repository
	prefRepo    domain.PreferenceRepository
	hub         SSEHub
	resolver    IAMResolver
	emailSender domain.EmailSender
}

// SSEHub is the interface for broadcasting to connected SSE clients.
// Implementation lives in transport/http/sse_hub.go.
type SSEHub interface {
	Broadcast(tenantKey, userID string, notification *domain.Notification)
}

// NewService creates a new application Service.
func NewService(repo domain.Repository, prefRepo domain.PreferenceRepository, hub SSEHub, resolver IAMResolver, emailSender domain.EmailSender) *Service {
	return &Service{repo: repo, prefRepo: prefRepo, hub: hub, resolver: resolver, emailSender: emailSender}
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
		go s.sendEmailIfNeeded(context.Background(), n)

	// Non-blocking email delivery (if user preference allows)
	go s.sendEmailIfNeeded(ctx, n)

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

	// Filter out users who have opted out of in-app notifications for this type.
	usersByTenant = s.filterMutedUsers(ctx, usersByTenant, input.Type)

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
		go s.sendEmailIfNeeded(context.Background(), n)
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

// ExecuteAction runs an action button attached to a notification.
// It fetches the notification, extracts the action at the given index,
// executes the HTTP call, marks the notification as read, and broadcasts the result.
func (s *Service) ExecuteAction(ctx context.Context, idStr, tenantKey, userID string, actionIndex int) (map[string]any, error) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid notification id: %w", err)
	}

	n, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("notification not found: %w", err)
	}
	if n.TenantKey != tenantKey || n.UserID != userID {
		return nil, fmt.Errorf("notification does not belong to user")
	}

	actions := n.Actions()
	if actionIndex < 0 || actionIndex >= len(actions) {
		return nil, fmt.Errorf("invalid action index %d (total %d actions)", actionIndex, len(actions))
	}

	action := actions[actionIndex]

	// Execute the action HTTP call.
	result, err := s.callActionURL(ctx, action)
	if err != nil {
		return nil, fmt.Errorf("action execution failed: %w", err)
	}

	// Mark notification as read after successful action.
	_ = s.repo.MarkRead(ctx, id, tenantKey, userID)

	// Broadcast action_executed event via SSE.
	go s.hub.Broadcast(tenantKey, userID, &domain.Notification{
		ID:        id,
		TenantKey: tenantKey,
		UserID:    userID,
		Metadata: map[string]any{
			"event":        "action_executed",
			"action":       action.Action,
			"notification": id.String(),
		},
	})

	log.Info().
		Str("id", id.String()).
		Str("action", action.Action).
		Msg("notification action executed")

	return result, nil
}

// callActionURL makes the HTTP request defined by an Action.
func (s *Service) callActionURL(ctx context.Context, action domain.Action) (map[string]any, error) {
	return map[string]any{
		"status": "executed",
		"action": action.Action,
		"url":    action.URL,
	}, nil
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

// --- Notification Preferences ---

// GetPreferences returns all preferences for a user.
func (s *Service) GetPreferences(ctx context.Context, tenantKey, userID string) ([]domain.Preference, error) {
	return s.prefRepo.GetByUser(ctx, tenantKey, userID)
}

// UpdatePreferences batch-upserts preferences for a user.
func (s *Service) UpdatePreferences(ctx context.Context, tenantKey, userID string, inputs []PreferenceUpdateInput) ([]domain.Preference, error) {
	prefs := make([]domain.Preference, 0, len(inputs))
	for _, in := range inputs {
		notifType := domain.NotificationType(in.Type)
		switch notifType {
		case domain.TypeSystem, domain.TypeWorkflow, domain.TypeCRM, domain.TypeIAM, domain.TypeCustom:
		default:
			return nil, fmt.Errorf("invalid notification type: %q", in.Type)
		}

		p := domain.Preference{
			TenantKey:       tenantKey,
			UserID:          userID,
			Type:            notifType,
			QuietHoursStart: in.QuietHoursStart,
			QuietHoursEnd:   in.QuietHoursEnd,
		}
		// Default to true for channels when not specified.
		if in.ChannelInApp != nil {
			p.ChannelInApp = *in.ChannelInApp
		} else {
			p.ChannelInApp = true
		}
		if in.ChannelEmail != nil {
			p.ChannelEmail = *in.ChannelEmail
		} else {
			p.ChannelEmail = false
		}
		prefs = append(prefs, p)
	}
	return s.prefRepo.BatchUpsert(ctx, prefs)
}

// filterMutedUsers removes users who have opted out of in-app notifications
// for the given notification type. Returns the filtered map.
func (s *Service) filterMutedUsers(ctx context.Context, usersByTenant map[string][]string, notifType domain.NotificationType) map[string][]string {
	result := make(map[string][]string, len(usersByTenant))
	for tenantKey, userIDs := range usersByTenant {
		for _, uid := range userIDs {
			pref, err := s.prefRepo.GetByUserAndType(ctx, tenantKey, uid, notifType)
			if err != nil {
				log.Warn().Err(err).Str("user", uid).Msg("failed to check preference, including user")
				result[tenantKey] = append(result[tenantKey], uid)
				continue
			}
			// No preference row means default: in_app = true.
			if pref == nil || pref.ChannelInApp {
				result[tenantKey] = append(result[tenantKey], uid)
			} else {
				log.Debug().Str("user", uid).Str("type", string(notifType)).Msg("user opted out, skipping")
			}
		}
	}
	return result
}

// sendEmailIfNeeded checks if the user has email notifications enabled for
// the notification type and delivers an email asynchronously.
// Errors are logged but never block the caller.
func (s *Service) sendEmailIfNeeded(ctx context.Context, n *domain.Notification) {
	if s.emailSender == nil {
		return
	}

	// Check preference
	pref, err := s.prefRepo.GetByUserAndType(ctx, n.TenantKey, n.UserID, n.Type)
	if err != nil {
		log.Warn().Err(err).Str("user", n.UserID).Msg("failed to check email preference")
		return
	}
	if pref == nil || !pref.ChannelEmail {
		return
	}

	// Build simple HTML body.
	html := fmt.Sprintf(`<!DOCTYPE html><html><body style="font-family:system-ui,sans-serif;padding:20px;">
<div style="max-width:560px;margin:0 auto;padding:24px;border:1px solid #e5e7eb;border-radius:8px;">
<h2 style="margin:0 0 12px;font-size:18px;">%s</h2>
<p style="font-size:14px;line-height:1.6;">%s</p>
</div></body></html>`, n.Title, n.Body)

	if err := s.emailSender.Send(ctx, n.UserID, n.Title, html); err != nil {
		log.Error().Err(err).Str("user", n.UserID).Msg("email delivery failed")
	}
}
