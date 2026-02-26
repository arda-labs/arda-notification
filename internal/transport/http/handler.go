package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"vn.io.arda/notification/internal/application"
	"vn.io.arda/notification/internal/domain"
)

// Handler holds all HTTP handler methods.
type Handler struct {
	svc *application.Service
	hub *Hub
}

// NewHandler creates a new Handler.
func NewHandler(svc *application.Service, hub *Hub) *Handler {
	return &Handler{svc: svc, hub: hub}
}

// --- REST Handlers ---

// ListNotifications GET /notifications
func (h *Handler) ListNotifications(c echo.Context) error {
	tenantKey, userID := mustClaims(c)

	filter := domain.NotificationFilter{
		TenantKey: tenantKey,
		UserID:    userID,
		Limit:     parseIntQuery(c, "limit", 20),
		Offset:    parseIntQuery(c, "offset", 0),
	}

	if t := c.QueryParam("type"); t != "" {
		filter.Type = domain.NotificationType(t)
	}
	if r := c.QueryParam("is_read"); r != "" {
		isRead := r == "true"
		filter.IsRead = &isRead
	}

	notifications, err := h.svc.List(c.Request().Context(), filter)
	if err != nil {
		return echo.ErrInternalServerError
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data":   notifications,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// GetUnreadCount GET /notifications/unread-count
func (h *Handler) GetUnreadCount(c echo.Context) error {
	tenantKey, userID := mustClaims(c)

	count, err := h.svc.CountUnread(c.Request().Context(), tenantKey, userID)
	if err != nil {
		return echo.ErrInternalServerError
	}
	return c.JSON(http.StatusOK, map[string]int64{"count": count})
}

// MarkRead PATCH /notifications/:id/read
func (h *Handler) MarkRead(c echo.Context) error {
	tenantKey, userID := mustClaims(c)
	id := c.Param("id")

	if err := h.svc.MarkRead(c.Request().Context(), id, tenantKey, userID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// MarkAllRead POST /notifications/read-all
func (h *Handler) MarkAllRead(c echo.Context) error {
	tenantKey, userID := mustClaims(c)

	count, err := h.svc.MarkAllRead(c.Request().Context(), tenantKey, userID)
	if err != nil {
		return echo.ErrInternalServerError
	}
	return c.JSON(http.StatusOK, map[string]int64{"marked": count})
}

// Delete DELETE /notifications/:id
func (h *Handler) Delete(c echo.Context) error {
	tenantKey, userID := mustClaims(c)
	id := c.Param("id")

	if err := h.svc.Delete(c.Request().Context(), id, tenantKey, userID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// --- SSE Handler ---

// Stream GET /notifications/stream — SSE endpoint
func (h *Handler) Stream(c echo.Context) error {
	tenantKey, userID := mustClaims(c)

	// SSE headers
	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable Nginx/APISIX buffering

	// Register client
	sendCh := make(chan []byte, 32)
	client := h.hub.Register(tenantKey, userID, sendCh)
	defer h.hub.Unregister(client)

	// Send initial "connected" event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
	w.Flush()

	log.Info().Str("tenant", tenantKey).Str("user", userID).Msg("SSE stream opened")

	ctx := c.Request().Context()
	for {
		select {
		case msg, ok := <-sendCh:
			if !ok {
				return nil
			}
			if _, err := w.Write(msg); err != nil {
				return nil
			}
			w.Flush()

		case <-ctx.Done():
			log.Info().Str("user", userID).Msg("SSE stream closed by client")
			return nil
		}
	}
}

// --- Healthcheck ---

// Health GET /health
func (h *Handler) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"status":           "ok",
		"sse_clients":       h.hub.ConnectedCount(),
	})
}

// --- Helpers ---

func mustClaims(c echo.Context) (tenantKey, userID string) {
	tenantKey = c.Get("tenantKey").(string)
	userID = c.Get("userID").(string)
	return
}

func parseIntQuery(c echo.Context, key string, def int) int {
	v, err := strconv.Atoi(c.QueryParam(key))
	if err != nil || v < 0 {
		return def
	}
	return v
}

// buildSSEMessage formats a notification as an SSE data frame.
func buildSSEMessage(n any) []byte {
	b, _ := json.Marshal(n)
	return []byte("event: notification\ndata: " + string(b) + "\n\n")
}
