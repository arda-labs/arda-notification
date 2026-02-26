package http

import (
	"sync"

	"github.com/rs/zerolog/log"
	"vn.io.arda/notification/internal/domain"
)

// Client represents a connected SSE client.
type Client struct {
	tenantKey string
	userID    string
	send      chan []byte
}

// Hub manages all active SSE client connections.
// Single-instance model: all broadcast is in-process.
// For multi-instance: replace with Redis Pub/Sub.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[string][]*Client // tenant -> userID -> clients
}

// NewHub creates a new SSE Hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]map[string][]*Client),
	}
}

// Register adds a new SSE client.
func (h *Hub) Register(tenantKey, userID string, send chan []byte) *Client {
	c := &Client{tenantKey: tenantKey, userID: userID, send: send}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[tenantKey] == nil {
		h.clients[tenantKey] = make(map[string][]*Client)
	}
	h.clients[tenantKey][userID] = append(h.clients[tenantKey][userID], c)

	log.Debug().Str("tenant", tenantKey).Str("user", userID).Msg("SSE client connected")
	return c
}

// Unregister removes an SSE client.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	users := h.clients[c.tenantKey]
	if users == nil {
		return
	}

	clients := users[c.userID]
	updated := make([]*Client, 0, len(clients))
	for _, existing := range clients {
		if existing != c {
			updated = append(updated, existing)
		}
	}

	if len(updated) == 0 {
		delete(users, c.userID)
	} else {
		users[c.userID] = updated
	}

	log.Debug().Str("tenant", c.tenantKey).Str("user", c.userID).Msg("SSE client disconnected")
}

// Broadcast sends a notification to all connected SSE clients for a user.
// This satisfies the application.SSEHub interface.
func (h *Hub) Broadcast(tenantKey, userID string, n *domain.Notification) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	users := h.clients[tenantKey]
	if users == nil {
		return
	}

	clients := users[userID]
	if len(clients) == 0 {
		return
	}

	// Build SSE message: "data: {...}\n\n"
	msg := buildSSEMessage(n)

	for _, c := range clients {
		select {
		case c.send <- msg:
		default:
			// Client is slow/disconnected, skip
			log.Warn().Str("user", userID).Msg("SSE client send buffer full, skipping")
		}
	}
}

// ConnectedCount returns the total number of connected SSE clients.
func (h *Hub) ConnectedCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	total := 0
	for _, users := range h.clients {
		for _, clients := range users {
			total += len(clients)
		}
	}
	return total
}
