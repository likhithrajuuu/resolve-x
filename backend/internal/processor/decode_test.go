package processor

import (
	"bytes"
	"compress/gzip"
	"testing"

	tracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	resv1 "go.opentelemetry.io/proto/otlp/resource/v1"
	spanv1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func sample() *tracev1.ExportTraceServiceRequest {
	return &tracev1.ExportTraceServiceRequest{ResourceSpans: []*spanv1.ResourceSpans{{
		Resource: &resv1.Resource{Attributes: []*commonv1.KeyValue{{Key: "service.name", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: "checkout"}}}}},
		ScopeSpans: []*spanv1.ScopeSpans{{Spans: []*spanv1.Span{{
			TraceId: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, SpanId: []byte{1, 2, 3, 4, 5, 6, 7, 8},
			Name: "GET /", StartTimeUnixNano: 1_000, EndTimeUnixNano: 5_000_000,
			Status: &spanv1.Status{Code: spanv1.Status_STATUS_CODE_ERROR, Message: "boom"},
		}}}},
	}}}
}

func check(t *testing.T, d Decoded, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Spans) != 1 || d.Service != "checkout" {
		t.Fatalf("got %d spans, service %q", len(d.Spans), d.Service)
	}
	s := d.Spans[0]
	if s.TenantID != "t1" || s.StatusCode != 2 || s.StatusMessage != "boom" || s.DurationNs != 4_999_000 || s.TraceID != "0102030405060708090a0b0c0d0e0f10" {
		t.Fatalf("unexpected row: %+v", s)
	}
}

func TestDecodeTracesProtobufJSONAndGzip(t *testing.T) {
	tn := Tenant{ID: "t1", Project: "p", Env: "e"}
	raw, _ := proto.Marshal(sample())
	d, err := DecodeTraces(tn, raw, "application/x-protobuf", "")
	check(t, d, err)

	js, _ := protojson.Marshal(sample())
	d, err = DecodeTraces(tn, js, "application/json", "")
	check(t, d, err)

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(raw)
	zw.Close()
	d, err = DecodeTraces(tn, buf.Bytes(), "application/x-protobuf", "gzip")
	check(t, d, err)
}

func TestDecodeRejectsGarbage(t *testing.T) {
	if _, err := DecodeTraces(Tenant{ID: "t"}, []byte("not protobuf \xff\xff"), "application/x-protobuf", ""); err == nil {
		t.Fatal("expected error")
	}
	if _, err := DecodeTraces(Tenant{ID: "t"}, []byte("nope"), "application/x-protobuf", "gzip"); err == nil {
		t.Fatal("expected gzip error")
	}
}
