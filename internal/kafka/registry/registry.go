// Package registry provides a lightweight event handler registry for Kafka events.
// Each domain handler registers itself via init(), eliminating the need to modify
// the consumer when adding new event handlers.
package registry

import (
	"encoding/json"

	"github.com/rs/zerolog/log"
	"vn.io.arda/notification/internal/domain"
)

// EventHandler maps raw Kafka message bytes to a FanoutInput.
// Returning nil means "skip this event" (no notification to send).
type EventHandler func(data []byte) *domain.FanoutInput

var mu_handlers = map[string]EventHandler{}

// Register binds a handler to a {topic}:{eventType} key.
// Should be called from each domain handler's init() function.
// Panics on duplicate registration to catch config mistakes early.
func Register(topic, eventType string, h EventHandler) {
	key := topic + ":" + eventType
	if _, exists := mu_handlers[key]; exists {
		panic("registry: duplicate handler registered for key: " + key)
	}
	mu_handlers[key] = h
}

// Dispatch looks up and calls the handler for the given topic + eventType.
// The eventType is extracted from the "eventType" JSON field in data.
// Returns nil if no handler found or data cannot be parsed.
func Dispatch(topic string, data []byte) *domain.FanoutInput {
	// Extract eventType without full parse
	var probe struct {
		EventType string `json:"eventType"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		log.Warn().Str("topic", topic).Err(err).Msg("registry: failed to probe eventType")
		return nil
	}

	key := topic + ":" + probe.EventType
	h, ok := mu_handlers[key]
	if !ok {
		log.Debug().Str("key", key).Msg("registry: no handler registered")
		return nil
	}
	return h(data)
}

// DispatchDirect calls the handler registered for a topic without eventType routing.
// Used for topics like notification-commands where the entire message is the command.
func DispatchDirect(topic string, data []byte) *domain.FanoutInput {
	key := topic + ":"
	h, ok := mu_handlers[key]
	if !ok {
		return nil
	}
	return h(data)
}
