// simulate emits a realistic multi-service system (traces with parent/child
// links, database client spans, error logs) so every Resolve-X feature has
// data to work on, with optional fault injection and a deployment event.
//
//	go run ./cmd/simulate -backfill 30m -d 2m                       # healthy
//	go run ./cmd/simulate -d 3m -fault payments -deploy payments=v2.14.0
package main

import (
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"math"
	mrand "math/rand/v2"
	"net/http"
	"strings"
	"time"

	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	logv1 "go.opentelemetry.io/proto/otlp/logs/v1"
	resv1 "go.opentelemetry.io/proto/otlp/resource/v1"
	spanv1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func str(k, v string) *commonv1.KeyValue {
	return &commonv1.KeyValue{Key: k, Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: v}}}
}

type batch struct {
	spans map[string][]*spanv1.Span
	logs  map[string][]*logv1.LogRecord
}

func newBatch() *batch {
	return &batch{spans: map[string][]*spanv1.Span{}, logs: map[string][]*logv1.LogRecord{}}
}

func rid(n int) []byte { b := make([]byte, n); _, _ = rand.Read(b); return b }

type cfg struct {
	fault string
}

// request simulates one user request starting at t0 and adds its spans/logs.
func (c cfg) request(b *batch, t0 time.Time) {
	trace := rid(16)
	failing := c.fault != "" && mrand.Float64() < 0.4
	jitter := func(ms float64) time.Duration {
		return time.Duration(ms * (0.7 + 0.6*mrand.Float64()) * float64(time.Millisecond))
	}

	type node struct {
		svc, name string
		kind      spanv1.Span_SpanKind
		attrs     []*commonv1.KeyValue
		self      time.Duration
		children  []*node
		err       string
	}
	pay := &node{svc: "payments", name: "POST /charge", kind: spanv1.Span_SPAN_KIND_SERVER, self: jitter(15)}
	dbMs := 12.0
	if c.fault == "payments" {
		dbMs = 900
	}
	db := &node{svc: "payments", name: "INSERT payments", kind: spanv1.Span_SPAN_KIND_CLIENT, self: jitter(dbMs),
		attrs: []*commonv1.KeyValue{str("db.system", "postgresql"), str("peer.service", "postgres-payments")}}
	if failing && c.fault == "payments" {
		db.err, pay.err = "connection pool exhausted", "connection pool exhausted"
	}
	pay.children = []*node{db}

	rds := &node{svc: "inventory", name: "GET stock", kind: spanv1.Span_SPAN_KIND_CLIENT, self: jitter(2), attrs: []*commonv1.KeyValue{str("db.system", "redis")}}
	inv := &node{svc: "inventory", name: "GET /stock", kind: spanv1.Span_SPAN_KIND_SERVER, self: jitter(6), children: []*node{rds}}
	checkout := &node{svc: "checkout", name: "POST /checkout", kind: spanv1.Span_SPAN_KIND_SERVER, self: jitter(10), children: []*node{inv, pay}}
	if pay.err != "" {
		checkout.err = "upstream payments failed"
	}
	root := &node{svc: "gateway", name: "POST /api/checkout", kind: spanv1.Span_SPAN_KIND_SERVER, self: jitter(4), children: []*node{checkout}}
	root.err = checkout.err

	var emit func(n *node, parent []byte, start time.Time) time.Duration
	emit = func(n *node, parent []byte, start time.Time) time.Duration {
		id := rid(8)
		cur := start.Add(n.self / 2)
		for _, ch := range n.children {
			cur = cur.Add(emit(ch, id, cur))
		}
		end := cur.Add(n.self / 2)
		sp := &spanv1.Span{TraceId: trace, SpanId: id, ParentSpanId: parent, Name: n.name, Kind: n.kind,
			StartTimeUnixNano: uint64(start.UnixNano()), EndTimeUnixNano: uint64(end.UnixNano()), Attributes: n.attrs}
		if n.err != "" {
			sp.Status = &spanv1.Status{Code: spanv1.Status_STATUS_CODE_ERROR, Message: n.err}
			b.logs[n.svc] = append(b.logs[n.svc], &logv1.LogRecord{
				TimeUnixNano: uint64(end.UnixNano()), SeverityNumber: logv1.SeverityNumber_SEVERITY_NUMBER_ERROR, SeverityText: "ERROR",
				Body:    &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: "ERROR " + n.err + " (max=10)"}},
				TraceId: trace, SpanId: id})
		}
		b.spans[n.svc] = append(b.spans[n.svc], sp)
		return end.Sub(start)
	}
	emit(root, nil, t0)
}

func (b *batch) send(base, key string) error {
	if len(b.spans) == 0 {
		return nil
	}
	var rs []*spanv1.ResourceSpans
	for svc, sp := range b.spans {
		rs = append(rs, &spanv1.ResourceSpans{Resource: &resv1.Resource{Attributes: []*commonv1.KeyValue{str("service.name", svc)}}, ScopeSpans: []*spanv1.ScopeSpans{{Spans: sp}}})
	}
	if err := post(base+"/v1/traces", key, &tracev1.ExportTraceServiceRequest{ResourceSpans: rs}); err != nil {
		return err
	}
	var rl []*logv1.ResourceLogs
	for svc, l := range b.logs {
		rl = append(rl, &logv1.ResourceLogs{Resource: &resv1.Resource{Attributes: []*commonv1.KeyValue{str("service.name", svc)}}, ScopeLogs: []*logv1.ScopeLogs{{LogRecords: l}}})
	}
	if len(rl) == 0 {
		return nil
	}
	return post(base+"/v1/logs", key, &logsv1.ExportLogsServiceRequest{ResourceLogs: rl})
}

func post(url, key string, m proto.Message) error {
	body, _ := proto.Marshal(m)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("X-Resolvex-Key", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: status %d", url, resp.StatusCode)
	}
	return nil
}

func main() {
	base := flag.String("url", "http://localhost:4318", "ingestion URL")
	api := flag.String("api", "http://localhost:8000", "Kong URL (for the deployment webhook)")
	key := flag.String("key", "rx_dev_local_key", "API key")
	dur := flag.Duration("d", time.Minute, "how long to emit live traffic")
	rps := flag.Int("rps", 20, "requests per second")
	backfill := flag.Duration("backfill", 0, "also emit healthy history for this long before now")
	fault := flag.String("fault", "", "inject a fault into this service (payments)")
	deploy := flag.String("deploy", "", "send a deployment webhook, as service=version, when live traffic starts")
	flag.Parse()

	if *backfill > 0 {
		healthy := cfg{}
		n := int(backfill.Seconds()) * 2 // 2 req/s of history is plenty for a baseline
		b := newBatch()
		for i := range n {
			healthy.request(b, time.Now().Add(-*backfill+time.Duration(float64(i)/float64(n)*float64(*backfill))))
			if (i+1)%200 == 0 {
				if err := b.send(*base, *key); err != nil {
					log.Fatal(err)
				}
				b = newBatch()
			}
		}
		if err := b.send(*base, *key); err != nil {
			log.Fatal(err)
		}
		log.Printf("backfilled %d healthy requests over %s", n, *backfill)
	}

	if *deploy != "" {
		svc, ver, ok := strings.Cut(*deploy, "=")
		if !ok {
			log.Fatal("-deploy must be service=version")
		}
		body := fmt.Sprintf(`{"service":%q,"version":%q,"environment":"production"}`, svc, ver)
		req, _ := http.NewRequest(http.MethodPost, *api+"/api/v1/webhooks/deployments", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Resolvex-Key", *key)
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 201 {
			log.Fatalf("deployment webhook failed: %v", err)
		}
		resp.Body.Close()
		log.Printf("recorded deployment %s", *deploy)
	}

	c := cfg{fault: *fault}
	end := time.Now().Add(*dur)
	for tick := time.NewTicker(time.Second); time.Now().Before(end); <-tick.C {
		b := newBatch()
		for range *rps {
			c.request(b, time.Now().Add(-time.Duration(math.Floor(mrand.Float64()*900))*time.Millisecond))
		}
		if err := b.send(*base, *key); err != nil {
			log.Printf("send: %v", err)
		}
	}
	log.Printf("done (fault=%q)", *fault)
}
