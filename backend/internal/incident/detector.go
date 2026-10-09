package incident

import (
	"context"
	"fmt"
	"time"

	"github.com/resolvex/resolve-x/backend/internal/query"
	"github.com/resolvex/resolve-x/backend/internal/store"
)

// Detector opens incidents automatically from telemetry. It looks at the last
// five minutes of entry spans per tenant and service and flags:
//   - error rate >= ErrorRate with at least MinSpans spans
//   - p95 latency >= LatencyFactor x the previous hour's p95, and above 500 ms
//
// Fingerprints make repeated detections update one open incident.
type Detector struct {
	Svc           *Service
	CH            *store.CH
	Interval      time.Duration
	MinSpans      uint64
	ErrorRate     float64
	LatencyFactor float64
}

type stat struct {
	tenant, service string
	spans, errors   uint64
	p95Ms           float64
}

func (d *Detector) stats(ctx context.Context, from, to time.Time) (map[string]stat, error) {
	rows, err := d.CH.Conn().Query(ctx, `
		SELECT tenant_id, service, count(), countIf(status_code = 2), quantile(0.95)(duration_ns) / 1e6
		FROM spans WHERE start_time >= ? AND start_time < ? AND (kind = 2 OR parent_span_id = '')
		GROUP BY tenant_id, service`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]stat{}
	for rows.Next() {
		var s stat
		if err := rows.Scan(&s.tenant, &s.service, &s.spans, &s.errors, &s.p95Ms); err != nil {
			return nil, err
		}
		out[s.tenant+"|"+s.service] = s
	}
	return out, rows.Err()
}

func (d *Detector) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := d.once(ctx, time.Now()); err != nil && ctx.Err() == nil {
			d.Svc.Log.Error("detector", "err", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(d.Interval):
		}
	}
}

func (d *Detector) once(ctx context.Context, now time.Time) error {
	recent, err := d.stats(ctx, now.Add(-5*time.Minute), now.Add(time.Minute))
	if err != nil {
		return err
	}
	base, err := d.stats(ctx, now.Add(-65*time.Minute), now.Add(-5*time.Minute))
	if err != nil {
		return err
	}
	type finding struct {
		inc NewIncident
		s   stat
	}
	var found []finding
	for k, s := range recent {
		if s.spans < d.MinSpans {
			continue
		}
		if rate := float64(s.errors) / float64(s.spans); rate >= d.ErrorRate {
			sev := "SEV-2"
			if rate >= 0.5 {
				sev = "SEV-1"
			}
			found = append(found, finding{NewIncident{
				Title: fmt.Sprintf("High error rate on %s (%.0f%%)", s.service, rate*100), Service: s.service, Severity: sev,
				Description: fmt.Sprintf("%d of %d entry spans failed in the last 5 minutes.", s.errors, s.spans),
				Source:      "detector", Fingerprint: "detector:errors:" + s.service}, s})
		}
		if b, ok := base[k]; ok && b.spans >= d.MinSpans && s.p95Ms >= 500 && s.p95Ms >= d.LatencyFactor*b.p95Ms {
			found = append(found, finding{NewIncident{
				Title: fmt.Sprintf("Latency spike on %s (p95 %.0f ms)", s.service, s.p95Ms), Service: s.service, Severity: "SEV-2",
				Description: fmt.Sprintf("p95 latency is %.0f ms, up from %.0f ms over the previous hour.", s.p95Ms, b.p95Ms),
				Source:      "detector", Fingerprint: "detector:latency:" + s.service}, s})
		}
	}

	// One fault makes every caller up the chain look unhealthy. Keep only the
	// deepest anomalous services: drop a service if something it depends on
	// (directly or transitively) is also anomalous.
	anomalous := map[string]map[string]bool{} // tenant -> services
	for _, f := range found {
		if anomalous[f.s.tenant] == nil {
			anomalous[f.s.tenant] = map[string]bool{}
		}
		anomalous[f.s.tenant][f.s.service] = true
	}
	graphs := map[string]map[string][]string{}
	for tenant := range anomalous {
		edges, err := query.Edges(ctx, d.CH, tenant, now.Add(-5*time.Minute), now.Add(time.Minute))
		if err != nil {
			return err
		}
		g := map[string][]string{}
		for _, e := range edges {
			g[e.Source] = append(g[e.Source], e.Target)
		}
		graphs[tenant] = g
	}
	for _, f := range found {
		if dependsOnAnomalous(graphs[f.s.tenant], f.s.service, anomalous[f.s.tenant]) {
			continue
		}
		if _, _, err := d.Svc.Create(ctx, f.s.tenant, f.inc); err != nil {
			return err
		}
	}
	return nil
}

func dependsOnAnomalous(g map[string][]string, from string, anomalous map[string]bool) bool {
	seen := map[string]bool{from: true}
	stack := append([]string(nil), g[from]...)
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[n] {
			continue
		}
		seen[n] = true
		if anomalous[n] {
			return true
		}
		stack = append(stack, g[n]...)
	}
	return false
}
