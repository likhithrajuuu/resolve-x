// Package analysis implements correlation (evidence gathering) and root-cause
// analysis. Correlation is deterministic SQL over ClickHouse and PostgreSQL;
// the analyzer then ranks candidate causes, optionally refined by an LLM.
package analysis

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/resolvex/resolve-x/backend/internal/query"
	"github.com/resolvex/resolve-x/backend/internal/store"
)

type Evidence struct {
	Kind    string    `json:"kind"` // metric | dependency | log | trace | deploy
	At      time.Time `json:"at"`
	Summary string    `json:"summary"`
}

type ServiceStat struct {
	Spans, Errors uint64
	ErrRate       float64
	P95Ms         float64
}

type DepStat struct {
	Target      string
	Calls, Errs uint64
	ErrRate     float64
	P95Ms       float64
	BaseErrRate float64
	BaseP95Ms   float64
	External    bool
}

func (d DepStat) Degraded() bool {
	return d.Calls >= 5 && (d.ErrRate >= 0.05 && d.ErrRate >= 3*d.BaseErrRate || d.P95Ms >= 200 && d.P95Ms >= 2*d.BaseP95Ms && d.BaseP95Ms > 0)
}

type Deploy struct {
	Service, Version string
	At               time.Time
}

type LogGroup struct {
	Service, Body string
	Count         uint64
}

// Bundle is everything correlation learned about one incident.
type Bundle struct {
	Service      string
	Now          time.Time
	AnomalyStart time.Time
	Recent, Base ServiceStat
	Deps         []DepStat
	Deploys      []Deploy
	Logs         []LogGroup
	Failures     []Failure
	Evidence     []Evidence
}

type Failure struct {
	Service, Span, Message string
	Count                  uint64
}

type Gatherer struct {
	CH *store.CH
	DB *pgxpool.Pool
}

const recentWindow = 10 * time.Minute

func (g *Gatherer) serviceStat(ctx context.Context, tenant, svc string, from, to time.Time) (ServiceStat, error) {
	var s ServiceStat
	err := g.CH.Conn().QueryRow(ctx, `
		SELECT count(), countIf(status_code = 2), quantile(0.95)(duration_ns) / 1e6
		FROM spans WHERE tenant_id = ? AND service = ? AND start_time >= ? AND start_time < ? AND (kind = 2 OR parent_span_id = '')`,
		tenant, svc, from, to).Scan(&s.Spans, &s.Errors, &s.P95Ms)
	if s.Spans > 0 {
		s.ErrRate = float64(s.Errors) / float64(s.Spans)
	}
	return s, err
}

// worstService picks the service with the highest recent error rate when the
// incident did not name one.
func (g *Gatherer) worstService(ctx context.Context, tenant string, now time.Time) (string, error) {
	var svc string
	err := g.CH.Conn().QueryRow(ctx, `
		SELECT service FROM spans WHERE tenant_id = ? AND start_time >= ? AND (kind = 2 OR parent_span_id = '')
		GROUP BY service HAVING count() >= 5 ORDER BY countIf(status_code = 2) / count() DESC, count() DESC LIMIT 1`,
		tenant, now.Add(-recentWindow)).Scan(&svc)
	return svc, err
}

func (g *Gatherer) Gather(ctx context.Context, tenant, svc string, now time.Time) (*Bundle, error) {
	b := &Bundle{Now: now}
	if svc == "" {
		var err error
		if svc, err = g.worstService(ctx, tenant, now); err != nil {
			return nil, fmt.Errorf("no telemetry to analyse: %w", err)
		}
	}
	b.Service = svc
	recentFrom, baseFrom := now.Add(-recentWindow), now.Add(-70*time.Minute)

	var err error
	if b.Recent, err = g.serviceStat(ctx, tenant, svc, recentFrom, now.Add(time.Minute)); err != nil {
		return nil, err
	}
	if b.Base, err = g.serviceStat(ctx, tenant, svc, baseFrom, recentFrom); err != nil {
		return nil, err
	}
	b.AnomalyStart, err = g.anomalyStart(ctx, tenant, svc, now, b.Base)
	if err != nil {
		return nil, err
	}
	b.Evidence = append(b.Evidence, Evidence{"metric", now, fmt.Sprintf(
		"%s: error rate %.1f%% (baseline %.1f%%), p95 %.0f ms (baseline %.0f ms) over %d entry spans",
		svc, b.Recent.ErrRate*100, b.Base.ErrRate*100, b.Recent.P95Ms, b.Base.P95Ms, b.Recent.Spans)})

	if err := g.dependencies(ctx, tenant, b, now, recentFrom, baseFrom); err != nil {
		return nil, err
	}
	involved := []string{svc}
	for _, d := range b.Deps {
		if d.Degraded() && !d.External {
			involved = append(involved, d.Target)
		}
	}
	if err := g.logsAndFailures(ctx, tenant, b, involved, recentFrom, now); err != nil {
		return nil, err
	}
	if err := g.deploys(ctx, tenant, b, involved, now); err != nil {
		return nil, err
	}
	sort.SliceStable(b.Evidence, func(i, j int) bool { return b.Evidence[i].At.Before(b.Evidence[j].At) })
	return b, nil
}

// anomalyStart finds the first minute of the last hour where the service
// looked unhealthy relative to its baseline.
func (g *Gatherer) anomalyStart(ctx context.Context, tenant, svc string, now time.Time, base ServiceStat) (time.Time, error) {
	rows, err := g.CH.Conn().Query(ctx, `
		SELECT toStartOfMinute(start_time) AS t, count(), countIf(status_code = 2), quantile(0.95)(duration_ns) / 1e6
		FROM spans WHERE tenant_id = ? AND service = ? AND start_time >= ? AND (kind = 2 OR parent_span_id = '')
		GROUP BY t ORDER BY t`, tenant, svc, now.Add(-time.Hour))
	if err != nil {
		return now, err
	}
	defer rows.Close()
	// Track the start of the most recent unhealthy streak, so an old,
	// already-recovered blip does not masquerade as the current onset.
	var start, prev time.Time
	inStreak := false
	for rows.Next() {
		var t time.Time
		var n, e uint64
		var p95 float64
		if err := rows.Scan(&t, &n, &e, &p95); err != nil {
			return now, err
		}
		if n < 3 {
			continue
		}
		if !prev.IsZero() && t.Sub(prev) > 2*time.Minute {
			inStreak = false // a gap in telemetry ends the streak
		}
		prev = t
		rate := float64(e) / float64(n)
		if rate >= 0.05 && rate >= 3*base.ErrRate || p95 >= 200 && base.P95Ms > 0 && p95 >= 2*base.P95Ms {
			if !inStreak {
				start, inStreak = t, true
			}
		} else {
			inStreak = false
		}
	}
	if inStreak {
		return start, nil
	}
	return now.Add(-recentWindow), nil
}

func (g *Gatherer) dependencies(ctx context.Context, tenant string, b *Bundle, now, recentFrom, baseFrom time.Time) error {
	recent, err := query.Edges(ctx, g.CH, tenant, recentFrom, now.Add(time.Minute))
	if err != nil {
		return err
	}
	base, err := query.Edges(ctx, g.CH, tenant, baseFrom, recentFrom)
	if err != nil {
		return err
	}
	type key struct{ s, t string }
	baseBy := map[key]DepStat{}
	for _, e := range base {
		baseBy[key{e.Source, e.Target}] = DepStat{ErrRate: ratio(e.Errors, e.Calls), P95Ms: e.P95Ms}
	}
	for _, e := range recent {
		if e.Source != b.Service {
			continue
		}
		bs := baseBy[key{e.Source, e.Target}]
		d := DepStat{Target: e.Target, Calls: e.Calls, Errs: e.Errors, ErrRate: ratio(e.Errors, e.Calls), P95Ms: e.P95Ms,
			BaseErrRate: bs.ErrRate, BaseP95Ms: bs.P95Ms, External: e.Kind == "external"}
		b.Deps = append(b.Deps, d)
		if d.Degraded() {
			b.Evidence = append(b.Evidence, Evidence{"dependency", now, fmt.Sprintf(
				"%s -> %s degraded: error rate %.1f%% (was %.1f%%), p95 %.0f ms (was %.0f ms) across %d calls",
				b.Service, d.Target, d.ErrRate*100, d.BaseErrRate*100, d.P95Ms, d.BaseP95Ms, d.Calls)})
		}
	}
	return nil
}

func ratio(a, b uint64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func (g *Gatherer) logsAndFailures(ctx context.Context, tenant string, b *Bundle, services []string, from, to time.Time) error {
	rows, err := g.CH.Conn().Query(ctx, `
		SELECT service, substring(body, 1, 200) AS msg, count() FROM logs
		WHERE tenant_id = ? AND ts >= ? AND ts < ? AND severity_number >= 17 AND service IN (?)
		GROUP BY service, msg ORDER BY count() DESC LIMIT 5`, tenant, from, to.Add(time.Minute), services)
	if err != nil {
		return err
	}
	for rows.Next() {
		var l LogGroup
		if err := rows.Scan(&l.Service, &l.Body, &l.Count); err != nil {
			rows.Close()
			return err
		}
		b.Logs = append(b.Logs, l)
		b.Evidence = append(b.Evidence, Evidence{"log", to, fmt.Sprintf("%s logged %d x ERROR: %s", l.Service, l.Count, l.Body)})
	}
	rows.Close()

	rows, err = g.CH.Conn().Query(ctx, `
		SELECT service, name, substring(status_message, 1, 200), count() FROM spans
		WHERE tenant_id = ? AND start_time >= ? AND start_time < ? AND status_code = 2 AND service IN (?)
		GROUP BY service, name, status_message ORDER BY count() DESC LIMIT 3`, tenant, from, to.Add(time.Minute), services)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var f Failure
		if err := rows.Scan(&f.Service, &f.Span, &f.Message, &f.Count); err != nil {
			return err
		}
		b.Failures = append(b.Failures, f)
		msg := f.Message
		if msg == "" {
			msg = "no status message"
		}
		b.Evidence = append(b.Evidence, Evidence{"trace", to, fmt.Sprintf("%d failed spans in %s %q (%s)", f.Count, f.Service, f.Span, msg)})
	}
	return nil
}

func (g *Gatherer) deploys(ctx context.Context, tenant string, b *Bundle, services []string, now time.Time) error {
	rows, err := g.DB.Query(ctx, `SELECT service, version, at FROM deployments
		WHERE tenant_id=$1 AND service = ANY($2) AND at >= $3 ORDER BY at`, tenant, services, now.Add(-3*time.Hour))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var d Deploy
		if err := rows.Scan(&d.Service, &d.Version, &d.At); err != nil {
			return err
		}
		b.Deploys = append(b.Deploys, d)
		b.Evidence = append(b.Evidence, Evidence{"deploy", d.At, fmt.Sprintf("%s version %s deployed", d.Service, d.Version)})
	}
	return rows.Err()
}

func (b *Bundle) Describe() string {
	var sb strings.Builder
	for _, e := range b.Evidence {
		fmt.Fprintf(&sb, "- [%s %s] %s\n", e.Kind, e.At.UTC().Format("15:04:05"), e.Summary)
	}
	return sb.String()
}
