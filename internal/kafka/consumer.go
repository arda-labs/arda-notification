package kafka

import (
	"context"
	"encoding/json"

	"github.com/rs/zerolog/log"
	"github.com/twmb/franz-go/pkg/kgo"
	"vn.io.arda/notification/internal/application"
	"vn.io.arda/notification/internal/kafka/registry"

	// Blank imports trigger init() in each handler file,
	// registering all event handlers into the registry.
	_ "vn.io.arda/notification/internal/kafka/handlers"
)

// Consumer wraps the franz-go Kafka client.
type Consumer struct {
	client  *kgo.Client
	service *application.Service
}

// New creates a Consumer with the given brokers, group ID, and topics.
func New(brokers []string, groupID string, topics []string, svc *application.Service) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, err
	}
	return &Consumer{client: client, service: svc}, nil
}

// Start begins polling Kafka and processing records. Blocks until ctx is cancelled.
func (c *Consumer) Start(ctx context.Context) {
	log.Info().Msg("kafka consumer started")

	for {
		fetches := c.client.PollFetches(ctx)
		if fetches.IsClientClosed() || ctx.Err() != nil {
			break
		}

		fetches.EachError(func(topic string, partition int32, err error) {
			log.Error().Err(err).Str("topic", topic).Int32("partition", partition).Msg("kafka fetch error")
		})

		fetches.EachRecord(func(r *kgo.Record) {
			c.process(ctx, r)
		})

		if err := c.client.CommitUncommittedOffsets(ctx); err != nil {
			log.Error().Err(err).Msg("kafka commit error")
		}
	}

	c.client.Close()
	log.Info().Msg("kafka consumer stopped")
}

// process dispatches a Kafka record to the registered handler via the registry,
// then calls Fanout on the result.
func (c *Consumer) process(ctx context.Context, r *kgo.Record) {
	log.Debug().
		Str("topic", r.Topic).
		Str("key", string(r.Key)).
		Msg("processing kafka record")

	// notification-commands doesn't use eventType routing
	var fanout = registry.DispatchDirect(r.Topic, r.Value)
	if fanout == nil {
		fanout = registry.Dispatch(r.Topic, r.Value)
	}

	if fanout == nil {
		log.Debug().Str("topic", r.Topic).Msg("no handler matched, skipping")
		return
	}

	if err := c.service.Fanout(ctx, *fanout); err != nil {
		log.Error().Err(err).
			Str("topic", r.Topic).
			Str("scope", string(fanout.TargetScope)).
			Str("target_id", fanout.TargetID).
			Str("source_event_id", fanout.SourceEventID).
			Msg("failed to fan-out notification from kafka event")
	}
}

// --- Shared event envelope ---

// EventEnvelope is the common wrapper used by all arda services for Kafka messages.
type EventEnvelope struct {
	EventType string          `json:"eventType"`
	EventID   string          `json:"eventId"`
	TenantKey string          `json:"tenantKey"`
	Payload   json.RawMessage `json:"payload"`
}

// ParseEnvelope decodes the common event envelope.
func ParseEnvelope(data []byte) (*EventEnvelope, error) {
	var env EventEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	return &env, nil
}
