package http

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"vn.io.arda/notification/internal/transport/mw"
)

// NewRouter sets up all Echo routes and middleware.
func NewRouter(h *Handler, keycloakBaseURL string) *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	// Global middleware
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{"Authorization", "Content-Type", "X-Tenant-Key"},
		AllowMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
	}))

	// Health (no auth required)
	e.GET("/health", h.Health)

	// API — requires authentication
	v1 := e.Group("")
	v1.Use(mw.JWTAuth(keycloakBaseURL))
	v1.Use(mw.TenantResolver())

	// REST endpoints
	v1.GET("/notifications", h.ListNotifications)
	v1.GET("/notifications/unread-count", h.GetUnreadCount)
	v1.PATCH("/notifications/:id/read", h.MarkRead)
	v1.POST("/notifications/read-all", h.MarkAllRead)
	v1.DELETE("/notifications/:id", h.Delete)

	// SSE endpoint
	v1.GET("/notifications/stream", h.Stream)

	return e
}
