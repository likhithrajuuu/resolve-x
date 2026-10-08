// loadgen pushes synthetic OTLP/HTTP traces at an ingestion endpoint and
// reports throughput and latency percentiles.
//
//	go run ./cmd/loadgen -url http://localhost:4318 -c 64 -d 30s -spans 100
package main

import (
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	tracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	resv1 "go.opentelemetry.io/proto/otlp/resource/v1"
	spanv1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func payload(spans int, services []string) []byte {
	now := uint64(time.Now().UnixNano())
	rs := make([]*spanv1.ResourceSpans, 0, len(services))
	per := max(spans/len(services), 1)
	for _, svc := range services {
		ss := make([]*spanv1.Span, per)
		for i := range ss {
			tid, sid := make([]byte, 16), make([]byte, 8)
			_, _ = rand.Read(tid)
			_, _ = rand.Read(sid)
			ss[i] = &spanv1.Span{
				TraceId: tid, SpanId: sid, Name: "GET /checkout", Kind: spanv1.Span_SPAN_KIND_SERVER,
				StartTimeUnixNano: now - 20_000_000, EndTimeUnixNano: now,
				Attributes: []*commonv1.KeyValue{{Key: "http.status_code", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 200}}}},
			}
		}
		rs = append(rs, &spanv1.ResourceSpans{
			Resource:   &resv1.Resource{Attributes: []*commonv1.KeyValue{{Key: "service.name", Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: svc}}}}},
			ScopeSpans: []*spanv1.ScopeSpans{{Spans: ss}},
		})
	}
	b, _ := proto.Marshal(&tracev1.ExportTraceServiceRequest{ResourceSpans: rs})
	return b
}

func main() {
	url := flag.String("url", "http://localhost:4318", "ingestion base URL")
	key := flag.String("key", "rx_dev_local_key", "API key")
	conc := flag.Int("c", 32, "concurrent workers")
	dur := flag.Duration("d", 15*time.Second, "test duration")
	spans := flag.Int("spans", 100, "spans per request")
	flag.Parse()

	body := payload(*spans, []string{"checkout", "payments", "inventory", "gateway"})
	client := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: *conc * 2}, Timeout: 10 * time.Second}

	var (
		mu     sync.Mutex
		lats   []time.Duration
		codes  = map[int]int{}
		errs   atomic.Int64
		stopAt = time.Now().Add(*dur)
		wg     sync.WaitGroup
	)
	for range *conc {
		wg.Go(func() {
			local := make([]time.Duration, 0, 4096)
			lc := map[int]int{}
			for time.Now().Before(stopAt) {
				req, _ := http.NewRequest(http.MethodPost, *url+"/v1/traces", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/x-protobuf")
				req.Header.Set("X-Resolvex-Key", *key)
				t := time.Now()
				resp, err := client.Do(req)
				if err != nil {
					errs.Add(1)
					continue
				}
				resp.Body.Close()
				local = append(local, time.Since(t))
				lc[resp.StatusCode]++
			}
			mu.Lock()
			lats = append(lats, local...)
			for c, n := range lc {
				codes[c] += n
			}
			mu.Unlock()
		})
	}
	wg.Wait()

	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	pct := func(p float64) time.Duration {
		if len(lats) == 0 {
			return 0
		}
		return lats[min(int(float64(len(lats))*p), len(lats)-1)]
	}
	n := len(lats)
	fmt.Printf("requests: %d  (%.0f req/s)  spans/s: %.0f\n", n, float64(n)/dur.Seconds(), float64(n**spans)/dur.Seconds())
	fmt.Printf("latency:  p50=%v p95=%v p99=%v max=%v\n", pct(.50), pct(.95), pct(.99), pct(1))
	fmt.Printf("status:   %v  transport errors: %d\n", codes, errs.Load())
}
