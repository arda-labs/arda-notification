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

// Suppress unused import warnings.
var _ = application.PreferenceUpdateInput{}

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

// --- Preferences Handlers ---

// GetPreferences GET /notifications/preferences
func (h *Handler) GetPreferences(c echo.Context) error {
	tenantKey, userID := mustClaims(c)

	prefs, err := h.svc.GetPreferences(c.Request().Context(), tenantKey, userID)
	if err != nil {
		return echo.ErrInternalServerError
	}
	if prefs == nil {
		prefs = []domain.Preference{}
	}
	return c.JSON(http.StatusOK, map[string]any{"data": prefs})
}

// UpdatePreferences PUT /notifications/preferences
func (h *Handler) UpdatePreferences(c echo.Context) error {
	tenantKey, userID := mustClaims(c)

	var inputs []application.PreferenceUpdateInput
	if err := c.Bind(&inputs); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if len(inputs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "no preferences provided")
	}

	prefs, err := h.svc.UpdatePreferences(c.Request().Context(), tenantKey, userID, inputs)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"data": prefs})
}

// --- Action Handlers ---

// ExecuteAction POST /notifications/:id/action
func (h *Handler) ExecuteAction(c echo.Context) error {
	tenantKey, userID := mustClaims(c)
	id := c.Param("id")

	var body struct {
		ActionIndex int `json:"actionIndex"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	result, err := h.svc.ExecuteAction(c.Request().Context(), id, tenantKey, userID, body.ActionIndex)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusOK, result)
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

// --- Template Admin Handlers ---

// ListTemplates GET /notifications/admin/templates
func (h *Handler) ListTemplates(c echo.Context) error {
	locale := c.QueryParam("locale")
	if locale == "" {
		locale = "vi"
	}
	templates, err := h.svc.ListTemplates(c.Request().Context(), locale)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if templates == nil {
		templates = []domain.Template{}
	}
	return c.JSON(http.StatusOK, map[string]any{"data": templates})
}

// UpsertTemplate PUT /notifications/admin/templates
func (h *Handler) UpsertTemplate(c echo.Context) error {
	var t domain.Template
	if err := c.Bind(&t); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if t.TemplateKey == "" || t.Locale == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "template_key and locale are required")
	}
	saved, err := h.svc.UpsertTemplate(c.Request().Context(), t)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"data": saved})
}

// DeleteTemplate DELETE /notifications/admin/templates/:key/:locale
func (h *Handler) DeleteTemplate(c echo.Context) error {
	key := c.Param("key")
	locale := c.Param("locale")
	if err := h.svc.DeleteTemplate(c.Request().Context(), key, locale); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}
