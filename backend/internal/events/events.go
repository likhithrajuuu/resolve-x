// Package events implements the Resolve-X domain event envelope
// (data-flow.md §3) plus a small Kafka producer/consumer toolkit.
package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	TopicIncidentCreated   = "incident.created"
	TopicAnalysisRequested = "analysis.requested"
	TopicAnalysisCompleted = "analysis.completed"
)

type Envelope struct {
	EventID    string          `json:"eventId"`
	EventType  string          `json:"eventType"`
	Version    int             `json:"eventVersion"`
	OccurredAt time.Time       `json:"occurredAt"`
	Producer   string          `json:"producer"`
	TenantID   string          `json:"tenantId"`
	Payload    json.RawMessage `json:"payload"`
}

func New(eventType, producer, tenant string, payload any) (Envelope, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{EventID: ulid.Make().String(), EventType: eventType, Version: 1,
		OccurredAt: time.Now().UTC(), Producer: producer, TenantID: tenant, Payload: b}, nil
}

// Key follows the tenant-first keying rule (ADR-010).
func Key(tenant, id string) []byte { return []byte(tenant + ":" + id) }

type Producer struct{ cl *kgo.Client }

func NewProducer(brokers []string) (*Producer, error) {
	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.AllowAutoTopicCreation(), kgo.RequiredAcks(kgo.AllISRAcks()))
	if err != nil {
		return nil, err
	}
	return &Producer{cl: cl}, nil
}

func (p *Producer) Close() { p.cl.Close() }

func (p *Producer) Publish(ctx context.Context, topic string, key []byte, e Envelope) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return p.cl.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: key, Value: b}).FirstErr()
}

// Consume runs handler for each event on topics until ctx ends. Offsets are
// committed after the handler succeeds (at-least-once; handlers must be
// idempotent). A handler that keeps failing is retried with backoff and then
// sent to <topic>.dlq so one poison message cannot block the partition.
func Consume(ctx context.Context, log *slog.Logger, brokers []string, group string, topics []string, handler func(context.Context, Envelope) error) error {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...), kgo.ConsumerGroup(group), kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(), kgo.AllowAutoTopicCreation(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return err
	}
	defer cl.Close()
	for ctx.Err() == nil {
		fetches := cl.PollFetches(ctx)
		if fetches.IsClientClosed() || ctx.Err() != nil {
			return nil
		}
		fetches.EachError(func(t string, p int32, err error) { log.Error("fetch", "topic", t, "err", err) })
		fetches.EachRecord(func(rec *kgo.Record) {
			var env Envelope
			if err := json.Unmarshal(rec.Value, &env); err != nil || env.TenantID == "" {
				log.Warn("dropping malformed event", "topic", rec.Topic)
				dead(ctx, cl, rec)
				return
			}
			for attempt := 1; ; attempt++ {
				err := handler(ctx, env)
				if err == nil {
					return
				}
				if ctx.Err() != nil {
					return
				}
				log.Error("handler failed", "topic", rec.Topic, "event", env.EventID, "attempt", attempt, "err", err)
				if attempt >= 5 {
					dead(ctx, cl, rec)
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(attempt) * time.Second):
				}
			}
		})
		if ctx.Err() == nil {
			_ = cl.CommitUncommittedOffsets(ctx)
		}
	}
	return nil
}

func dead(ctx context.Context, cl *kgo.Client, rec *kgo.Record) {
	_ = cl.ProduceSync(ctx, &kgo.Record{Topic: rec.Topic + ".dlq", Key: rec.Key, Value: rec.Value}).FirstErr()
}
