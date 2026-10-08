// Package processor turns raw OTLP payloads into normalised rows.
package processor

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"time"

	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/resolvex/resolve-x/backend/internal/store"
)

// Tenant carries the identity stamped on the record by the Ingestion Service.
type Tenant struct{ ID, Project, Env string }

const maxDecompressed = 64 << 20

func inflate(body []byte, enc string) ([]byte, error) {
	if enc != "gzip" {
		return body, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	out, err := io.ReadAll(io.LimitReader(zr, maxDecompressed+1))
	if err != nil {
		return nil, err
	}
	if len(out) > maxDecompressed {
		return nil, fmt.Errorf("decompressed payload exceeds %d bytes", maxDecompressed)
	}
	return out, nil
}

func unmarshal(body []byte, contentType string, m proto.Message) error {
	if contentType == "application/json" {
		return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(body, m)
	}
	return proto.Unmarshal(body, m)
}

func attrs(kvs []*commonv1.KeyValue) map[string]string {
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		m[kv.Key] = anyString(kv.Value)
	}
	return m
}

func anyString(v *commonv1.AnyValue) string {
	if v == nil {
		return ""
	}
	switch x := v.Value.(type) {
	case *commonv1.AnyValue_StringValue:
		return x.StringValue
	case *commonv1.AnyValue_IntValue:
		return strconv.FormatInt(x.IntValue, 10)
	case *commonv1.AnyValue_DoubleValue:
		return strconv.FormatFloat(x.DoubleValue, 'g', -1, 64)
	case *commonv1.AnyValue_BoolValue:
		return strconv.FormatBool(x.BoolValue)
	case *commonv1.AnyValue_BytesValue:
		return hex.EncodeToString(x.BytesValue)
	default:
		if b, err := protojson.Marshal(v); err == nil {
			return string(b)
		}
		return ""
	}
}

func serviceName(res map[string]string) string {
	if s := res["service.name"]; s != "" {
		return s
	}
	return "unknown_service"
}

func nanos(n uint64) time.Time { return time.Unix(0, int64(n)).UTC() }

// Decoded is the result of decoding one telemetry.raw record.
type Decoded struct {
	Service string // first resource's service.name, used to key the split topic
	Spans   []store.Span
	Logs    []store.Log
	Metrics []store.Metric
}

func DecodeTraces(t Tenant, body []byte, ct, enc string) (Decoded, error) {
	raw, err := inflate(body, enc)
	if err != nil {
		return Decoded{}, err
	}
	var req tracev1.ExportTraceServiceRequest
	if err := unmarshal(raw, ct, &req); err != nil {
		return Decoded{}, err
	}
	var d Decoded
	for _, rs := range req.ResourceSpans {
		res := attrs(rs.GetResource().GetAttributes())
		svc := serviceName(res)
		if d.Service == "" {
			d.Service = svc
		}
		for _, ss := range rs.ScopeSpans {
			for _, s := range ss.Spans {
				var dur uint64
				if s.EndTimeUnixNano > s.StartTimeUnixNano {
					dur = s.EndTimeUnixNano - s.StartTimeUnixNano
				}
				d.Spans = append(d.Spans, store.Span{
					TenantID: t.ID, ProjectID: t.Project, EnvID: t.Env, Service: svc,
					TraceID: hex.EncodeToString(s.TraceId), SpanID: hex.EncodeToString(s.SpanId),
					ParentSpanID: hex.EncodeToString(s.ParentSpanId), Name: s.Name, Kind: uint8(s.Kind),
					Start: nanos(s.StartTimeUnixNano), DurationNs: dur,
					StatusCode: uint8(s.GetStatus().GetCode()), StatusMessage: s.GetStatus().GetMessage(),
					Attrs: attrs(s.Attributes), ResAttrs: res,
				})
			}
		}
	}
	return d, nil
}

func DecodeLogs(t Tenant, body []byte, ct, enc string) (Decoded, error) {
	raw, err := inflate(body, enc)
	if err != nil {
		return Decoded{}, err
	}
	var req logsv1.ExportLogsServiceRequest
	if err := unmarshal(raw, ct, &req); err != nil {
		return Decoded{}, err
	}
	var d Decoded
	for _, rl := range req.ResourceLogs {
		res := attrs(rl.GetResource().GetAttributes())
		svc := serviceName(res)
		if d.Service == "" {
			d.Service = svc
		}
		for _, sl := range rl.ScopeLogs {
			for _, l := range sl.LogRecords {
				ts := l.TimeUnixNano
				if ts == 0 {
					ts = l.ObservedTimeUnixNano
				}
				d.Logs = append(d.Logs, store.Log{
					TenantID: t.ID, ProjectID: t.Project, EnvID: t.Env, Service: svc,
					TS: nanos(ts), SeverityNumber: uint8(l.SeverityNumber), SeverityText: l.SeverityText,
					Body: anyString(l.Body), TraceID: hex.EncodeToString(l.TraceId), SpanID: hex.EncodeToString(l.SpanId),
					Attrs: attrs(l.Attributes), ResAttrs: res,
				})
			}
		}
	}
	return d, nil
}

func DecodeMetrics(t Tenant, body []byte, ct, enc string) (Decoded, error) {
	raw, err := inflate(body, enc)
	if err != nil {
		return Decoded{}, err
	}
	var req metricsv1.ExportMetricsServiceRequest
	if err := unmarshal(raw, ct, &req); err != nil {
		return Decoded{}, err
	}
	var d Decoded
	for _, rm := range req.ResourceMetrics {
		res := attrs(rm.GetResource().GetAttributes())
		svc := serviceName(res)
		if d.Service == "" {
			d.Service = svc
		}
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				row := func(kind string, ts uint64, v float64, c uint64, a []*commonv1.KeyValue) {
					d.Metrics = append(d.Metrics, store.Metric{
						TenantID: t.ID, ProjectID: t.Project, EnvID: t.Env, Service: svc,
						Name: m.Name, Unit: m.Unit, Kind: kind, TS: nanos(ts), Value: v, Count: c,
						Attrs: attrs(a), ResAttrs: res,
					})
				}
				num := func(p *metricspb.NumberDataPoint) float64 {
					if p.GetAsInt() != 0 {
						return float64(p.GetAsInt())
					}
					return p.GetAsDouble()
				}
				switch x := m.Data.(type) {
				case *metricspb.Metric_Gauge:
					for _, p := range x.Gauge.DataPoints {
						row("gauge", p.TimeUnixNano, num(p), 1, p.Attributes)
					}
				case *metricspb.Metric_Sum:
					for _, p := range x.Sum.DataPoints {
						row("sum", p.TimeUnixNano, num(p), 1, p.Attributes)
					}
				case *metricspb.Metric_Histogram:
					for _, p := range x.Histogram.DataPoints {
						row("histogram", p.TimeUnixNano, p.GetSum(), p.Count, p.Attributes)
					}
				case *metricspb.Metric_ExponentialHistogram:
					for _, p := range x.ExponentialHistogram.DataPoints {
						row("exp_histogram", p.TimeUnixNano, p.GetSum(), p.Count, p.Attributes)
					}
				case *metricspb.Metric_Summary:
					for _, p := range x.Summary.DataPoints {
						row("summary", p.TimeUnixNano, p.Sum, p.Count, p.Attributes)
					}
				}
			}
		}
	}
	return d, nil
}

var errUnknownSignal = fmt.Errorf("unknown signal")
