// Package telemetry defines the wire contract for telemetry.raw.
//
// telemetry.raw is the one high-volume topic that does NOT use the JSON event
// envelope from data-flow.md: the record value is the untouched OTLP request
// body and the envelope fields travel as Kafka headers. This keeps the
// ingestion hot path free of parsing and re-encoding.
package telemetry

type Signal string

const (
	Traces  Signal = "traces"
	Metrics Signal = "metrics"
	Logs    Signal = "logs"
)

const (
	TopicRaw     = "telemetry.raw"
	TopicTraces  = "telemetry.traces"
	TopicMetrics = "telemetry.metrics"
	TopicLogs    = "telemetry.logs"
)

func (s Signal) Topic() string { return "telemetry." + string(s) }

// Header names on telemetry.raw records.
const (
	HdrEventID         = "event-id"
	HdrTenantID        = "tenant-id"
	HdrProjectID       = "project-id"
	HdrEnvironmentID   = "environment-id"
	HdrSignal          = "signal"
	HdrContentType     = "content-type"
	HdrContentEncoding = "content-encoding"
	HdrReceivedAt      = "received-at" // unix nanoseconds
)
