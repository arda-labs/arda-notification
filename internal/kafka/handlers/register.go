package handlers

import (
	"vn.io.arda/notification/internal/domain"
	"vn.io.arda/notification/internal/kafka/registry"
)

// Register is a convenience alias so each domain file calls Register(...)
// instead of registry.Register(...), keeping imports minimal.
func Register(topic, eventType string, h registry.EventHandler) {
	registry.Register(topic, eventType, h)
}

// RegisterDirect registers a handler for topics that don't use eventType routing.
func RegisterDirect(topic string, h registry.EventHandler) {
	registry.Register(topic, "", h)
}

// Ensure domain is imported (used by all handler files).
var _ = domain.TypeSystem
