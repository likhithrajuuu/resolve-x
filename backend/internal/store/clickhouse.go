// Package store writes normalised telemetry to ClickHouse (ADR-012).
package store

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// All tables are partitioned by day and ordered tenant-first so a tenant's
// queries prune to its own data. TTL is the retention default; per-plan
// retention is a later change.
var ddl = []string{
	`CREATE TABLE IF NOT EXISTS spans (
		tenant_id LowCardinality(String), project_id LowCardinality(String), environment_id LowCardinality(String),
		service LowCardinality(String), trace_id String, span_id String, parent_span_id String,
		name LowCardinality(String), kind UInt8, start_time DateTime64(9), duration_ns UInt64,
		status_code UInt8, status_message String,
		attrs Map(String, String), resource_attrs Map(String, String)
	) ENGINE = MergeTree PARTITION BY toDate(start_time)
	ORDER BY (tenant_id, project_id, service, start_time, trace_id)
	TTL toDateTime(start_time) + INTERVAL 30 DAY`,

	`CREATE TABLE IF NOT EXISTS logs (
		tenant_id LowCardinality(String), project_id LowCardinality(String), environment_id LowCardinality(String),
		service LowCardinality(String), ts DateTime64(9), severity_number UInt8, severity_text LowCardinality(String),
		body String, trace_id String, span_id String,
		attrs Map(String, String), resource_attrs Map(String, String)
	) ENGINE = MergeTree PARTITION BY toDate(ts)
	ORDER BY (tenant_id, project_id, service, ts)
	TTL toDateTime(ts) + INTERVAL 14 DAY`,

	`CREATE TABLE IF NOT EXISTS metrics (
		tenant_id LowCardinality(String), project_id LowCardinality(String), environment_id LowCardinality(String),
		service LowCardinality(String), name LowCardinality(String), unit LowCardinality(String), kind LowCardinality(String),
		ts DateTime64(9), value Float64, count UInt64,
		attrs Map(String, String), resource_attrs Map(String, String)
	) ENGINE = MergeTree PARTITION BY toDate(ts)
	ORDER BY (tenant_id, project_id, service, name, ts)
	TTL toDateTime(ts) + INTERVAL 30 DAY`,
}

type CH struct{ conn driver.Conn }

func OpenCH(ctx context.Context, addr, db, user, pass string) (*CH, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr:            []string{addr},
		Auth:            clickhouse.Auth{Database: db, Username: user, Password: pass},
		Compression:     &clickhouse.Compression{Method: clickhouse.CompressionLZ4},
		DialTimeout:     5 * time.Second,
		MaxOpenConns:    8,
		ConnMaxLifetime: time.Hour,
	})
	if err != nil {
		return nil, err
	}
	for _, q := range ddl {
		if err := conn.Exec(ctx, q); err != nil {
			return nil, err
		}
	}
	return &CH{conn: conn}, nil
}

func (c *CH) Close() error { return c.conn.Close() }

type Span struct {
	TenantID, ProjectID, EnvID, Service, TraceID, SpanID, ParentSpanID, Name string
	Kind                                                                     uint8
	Start                                                                    time.Time
	DurationNs                                                               uint64
	StatusCode                                                               uint8
	StatusMessage                                                            string
	Attrs, ResAttrs                                                          map[string]string
}

type Log struct {
	TenantID, ProjectID, EnvID, Service string
	TS                                  time.Time
	SeverityNumber                      uint8
	SeverityText, Body, TraceID, SpanID string
	Attrs, ResAttrs                     map[string]string
}

type Metric struct {
	TenantID, ProjectID, EnvID, Service, Name, Unit, Kind string
	TS                                                    time.Time
	Value                                                 float64
	Count                                                 uint64
	Attrs, ResAttrs                                       map[string]string
}

func (c *CH) InsertSpans(ctx context.Context, rows []Span) error {
	if len(rows) == 0 {
		return nil
	}
	b, err := c.conn.PrepareBatch(ctx, "INSERT INTO spans")
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := b.Append(r.TenantID, r.ProjectID, r.EnvID, r.Service, r.TraceID, r.SpanID, r.ParentSpanID,
			r.Name, r.Kind, r.Start, r.DurationNs, r.StatusCode, r.StatusMessage, r.Attrs, r.ResAttrs); err != nil {
			return err
		}
	}
	return b.Send()
}

func (c *CH) InsertLogs(ctx context.Context, rows []Log) error {
	if len(rows) == 0 {
		return nil
	}
	b, err := c.conn.PrepareBatch(ctx, "INSERT INTO logs")
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := b.Append(r.TenantID, r.ProjectID, r.EnvID, r.Service, r.TS, r.SeverityNumber, r.SeverityText,
			r.Body, r.TraceID, r.SpanID, r.Attrs, r.ResAttrs); err != nil {
			return err
		}
	}
	return b.Send()
}

func (c *CH) InsertMetrics(ctx context.Context, rows []Metric) error {
	if len(rows) == 0 {
		return nil
	}
	b, err := c.conn.PrepareBatch(ctx, "INSERT INTO metrics")
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := b.Append(r.TenantID, r.ProjectID, r.EnvID, r.Service, r.Name, r.Unit, r.Kind,
			r.TS, r.Value, r.Count, r.Attrs, r.ResAttrs); err != nil {
			return err
		}
	}
	return b.Send()
}
