package ingest

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Tenant is deliberately NOT a label: it is unbounded cardinality.
	// Per-tenant accounting belongs in the usage pipeline, not Prometheus.
	requests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingestion_requests_total", Help: "Export requests by signal, transport and outcome.",
	}, []string{"signal", "transport", "outcome"})
	payloadBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "ingestion_payload_bytes", Help: "Accepted payload size.",
		Buckets: prometheus.ExponentialBuckets(512, 4, 8),
	}, []string{"signal"})
	publishSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "ingestion_kafka_publish_seconds", Help: "Time to Kafka ack.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
	})
)
