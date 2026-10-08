package ingest

import (
	"context"
	"strconv"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/resolvex/resolve-x/backend/internal/auth"
	"github.com/resolvex/resolve-x/backend/internal/telemetry"
)

// Publisher writes accepted payloads to telemetry.raw and waits for the
// broker ack, so a 200 to the collector means the data is durable in Kafka.
type Publisher struct {
	cl      *kgo.Client
	timeout time.Duration
}

func NewPublisher(brokers []string, timeout time.Duration) (*Publisher, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.DefaultProduceTopic(telemetry.TopicRaw),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(kgo.Lz4Compression(), kgo.SnappyCompression(), kgo.NoCompression()),
		kgo.ProducerLinger(5*time.Millisecond),
		kgo.ProducerBatchMaxBytes(4<<20),
		kgo.MaxBufferedRecords(50_000),
		kgo.RecordPartitioner(kgo.UniformBytesPartitioner(64<<10, false, false, nil)),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, err
	}
	return &Publisher{cl: cl, timeout: timeout}, nil
}

func (p *Publisher) Close()                         { p.cl.Close() }
func (p *Publisher) Ping(ctx context.Context) error { return p.cl.Ping(ctx) }

// Publish blocks until Kafka acks the record or the timeout elapses.
// The record has no key: telemetry.raw needs no ordering, and unkeyed
// records spread evenly over partitions so one noisy tenant cannot create a
// hot partition. Processors re-key by tenant:service downstream.
func (p *Publisher) Publish(ctx context.Context, pr *auth.Principal, sig telemetry.Signal, contentType, encoding string, body []byte) error {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	hdrs := []kgo.RecordHeader{
		{Key: telemetry.HdrEventID, Value: []byte(ulid.Make().String())},
		{Key: telemetry.HdrTenantID, Value: []byte(pr.TenantID)},
		{Key: telemetry.HdrProjectID, Value: []byte(pr.ProjectID)},
		{Key: telemetry.HdrEnvironmentID, Value: []byte(pr.EnvironmentID)},
		{Key: telemetry.HdrSignal, Value: []byte(sig)},
		{Key: telemetry.HdrContentType, Value: []byte(contentType)},
		{Key: telemetry.HdrReceivedAt, Value: []byte(strconv.FormatInt(time.Now().UnixNano(), 10))},
	}
	if encoding != "" {
		hdrs = append(hdrs, kgo.RecordHeader{Key: telemetry.HdrContentEncoding, Value: []byte(encoding)})
	}
	done := make(chan error, 1)
	p.cl.Produce(ctx, &kgo.Record{Value: body, Headers: hdrs}, func(_ *kgo.Record, err error) { done <- err })
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
