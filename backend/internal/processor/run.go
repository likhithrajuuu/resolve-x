package processor

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/resolvex/resolve-x/backend/internal/store"
	"github.com/resolvex/resolve-x/backend/internal/telemetry"
)

var (
	processed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "processor_records_total", Help: "telemetry.raw records by signal and outcome.",
	}, []string{"signal", "outcome"})
	lag = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "processor_ingest_to_store_seconds", Help: "Ingestion receipt to ClickHouse write.",
		Buckets: prometheus.ExponentialBuckets(0.05, 2, 12),
	})
)

type Runner struct {
	Brokers []string
	Group   string
	CH      *store.CH
	Log     *slog.Logger
}

func header(r *kgo.Record, key string) string {
	for _, h := range r.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

// Run consumes telemetry.raw until ctx is cancelled.
//
// Delivery is at-least-once: offsets are committed only after the ClickHouse
// insert and the split-topic produce both succeed. A crash between the two
// replays the batch, so ClickHouse rows may duplicate (data-flow.md, delivery
// rules). Records that can never decode go to telemetry.raw.dlq.
func (r *Runner) Run(ctx context.Context) error {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(r.Brokers...),
		kgo.ConsumerGroup(r.Group),
		kgo.ConsumeTopics(telemetry.TopicRaw),
		kgo.DisableAutoCommit(),
		kgo.FetchMaxBytes(32<<20),
		kgo.ProducerBatchCompression(kgo.Lz4Compression(), kgo.NoCompression()),
		kgo.ProducerLinger(10*time.Millisecond),
		kgo.AllowAutoTopicCreation(),
		kgo.RecordPartitioner(kgo.UniformBytesPartitioner(64<<10, true, true, nil)),
	)
	if err != nil {
		return err
	}
	defer cl.Close()

	for ctx.Err() == nil {
		fetches := cl.PollRecords(ctx, 2000)
		if fetches.IsClientClosed() || ctx.Err() != nil {
			return nil
		}
		fetches.EachError(func(t string, p int32, err error) { r.Log.Error("fetch", "topic", t, "partition", p, "err", err) })

		var (
			spans   []store.Span
			logs    []store.Log
			metrics []store.Metric
			out     []*kgo.Record
			dead    []*kgo.Record
			oldest  = time.Now().UnixNano()
		)
		fetches.EachRecord(func(rec *kgo.Record) {
			sig := telemetry.Signal(header(rec, telemetry.HdrSignal))
			t := Tenant{ID: header(rec, telemetry.HdrTenantID), Project: header(rec, telemetry.HdrProjectID), Env: header(rec, telemetry.HdrEnvironmentID)}
			ct, enc := header(rec, telemetry.HdrContentType), header(rec, telemetry.HdrContentEncoding)
			if t.ID == "" { // events without a tenant are rejected (ADR-010)
				processed.WithLabelValues(string(sig), "dead_letter").Inc()
				dead = append(dead, rec)
				return
			}
			var d Decoded
			var err error
			switch sig {
			case telemetry.Traces:
				d, err = DecodeTraces(t, rec.Value, ct, enc)
				spans = append(spans, d.Spans...)
			case telemetry.Logs:
				d, err = DecodeLogs(t, rec.Value, ct, enc)
				logs = append(logs, d.Logs...)
			case telemetry.Metrics:
				d, err = DecodeMetrics(t, rec.Value, ct, enc)
				metrics = append(metrics, d.Metrics...)
			default:
				err = errUnknownSignal
			}
			if err != nil {
				r.Log.Warn("undecodable record", "signal", sig, "err", err)
				processed.WithLabelValues(string(sig), "dead_letter").Inc()
				dead = append(dead, rec)
				return
			}
			processed.WithLabelValues(string(sig), "ok").Inc()
			if ts, err := strconv.ParseInt(header(rec, telemetry.HdrReceivedAt), 10, 64); err == nil && ts < oldest {
				oldest = ts
			}
			// Split topic keyed tenant:service so a service's data stays together.
			out = append(out, &kgo.Record{
				Topic: sig.Topic(), Key: []byte(t.ID + ":" + d.Service), Value: rec.Value, Headers: rec.Headers,
			})
		})
		if len(out) == 0 && len(dead) == 0 {
			continue
		}

		if err := r.flush(ctx, cl, spans, logs, metrics, out, dead); err != nil {
			return nil // only returns on context cancellation; uncommitted offsets replay
		}
		lag.Observe(time.Since(time.Unix(0, oldest)).Seconds())
		if err := cl.CommitUncommittedOffsets(ctx); err != nil && ctx.Err() == nil {
			r.Log.Error("commit failed", "err", err)
		}
	}
	return nil
}

// flush performs each write step with retry. A step that succeeded is never
// repeated, so a transient failure on a later step does not duplicate rows
// written by an earlier one. It returns an error only when ctx is cancelled.
func (r *Runner) flush(ctx context.Context, cl *kgo.Client, sp []store.Span, lg []store.Log, mt []store.Metric, out, dead []*kgo.Record) error {
	for _, d := range dead {
		out = append(out, &kgo.Record{Topic: telemetry.TopicRaw + ".dlq", Value: d.Value, Headers: d.Headers})
	}
	steps := []struct {
		name string
		fn   func() error
	}{
		{"spans", func() error { return r.CH.InsertSpans(ctx, sp) }},
		{"logs", func() error { return r.CH.InsertLogs(ctx, lg) }},
		{"metrics", func() error { return r.CH.InsertMetrics(ctx, mt) }},
		{"split-topics", func() error {
			if len(out) == 0 {
				return nil
			}
			return cl.ProduceSync(ctx, out...).FirstErr()
		}},
	}
	for _, st := range steps {
		for backoff := time.Second; ; backoff = min(backoff*2, 15*time.Second) {
			err := st.fn()
			if err == nil {
				break
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			r.Log.Error("flush step failed; retrying", "step", st.name, "err", err)
			sleep(ctx, backoff)
		}
	}
	return nil
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
