// Package query is the read API over ClickHouse telemetry. It also derives the
// service catalog and dependency graph from spans, so services appear without
// manual registration (service-registry.md).
package query

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/resolvex/resolve-x/backend/internal/authn"
	"github.com/resolvex/resolve-x/backend/internal/store"
)

type API struct {
	CH     *store.CH
	Secret []byte
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/services", a.services)
	mux.HandleFunc("GET /api/v1/services/{name}/timeseries", a.serviceSeries)
	mux.HandleFunc("GET /api/v1/dependencies", a.dependencies)
	mux.HandleFunc("GET /api/v1/traces", a.traces)
	mux.HandleFunc("GET /api/v1/traces/{traceId}", a.trace)
	mux.HandleFunc("GET /api/v1/logs", a.logs)
	mux.HandleFunc("GET /api/v1/metrics", a.metricNames)
	mux.HandleFunc("GET /api/v1/metrics/series", a.metricSeries)
	return authn.Require(a.Secret, mux)
}

// tenant is ALWAYS taken from the verified token.
func tenant(r *http.Request) string { return authn.FromContext(r.Context()).Tenant }

func window(r *http.Request) (time.Time, time.Duration) {
	d := time.Hour
	if v := r.URL.Query().Get("window"); v != "" {
		if p, err := time.ParseDuration(v); err == nil && p > 0 {
			d = p
		}
	}
	d = min(d, 7*24*time.Hour)
	return time.Now().Add(-d), d
}

func limit(r *http.Request, def, max int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 {
		return min(n, max)
	}
	return def
}

// bucketSeconds picks ~60 points over the window, at least 10 s apart.
func bucketSeconds(d time.Duration) int { return max(int(d.Seconds())/60, 10) }

var nameRe = regexp.MustCompile(`^[\w.\-:/ ]{1,200}$`)

func fail(w http.ResponseWriter, err error) {
	authn.WriteError(w, http.StatusInternalServerError, "internal", "query failed")
	_ = err
}

// isEntry selects entry-point spans: server spans or trace roots.
const isEntry = "(kind = 2 OR parent_span_id = '')"

type serviceRow struct {
	Name      string    `json:"name"`
	Spans     uint64    `json:"spans"`
	Errors    uint64    `json:"errors"`
	ErrorRate float64   `json:"errorRate"`
	P50Ms     float64   `json:"p50Ms"`
	P95Ms     float64   `json:"p95Ms"`
	P99Ms     float64   `json:"p99Ms"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
}

func (a *API) services(w http.ResponseWriter, r *http.Request) {
	since, _ := window(r)
	rows, err := a.CH.Conn().Query(r.Context(), `
		SELECT service, count(), countIf(status_code = 2),
		       quantile(0.5)(duration_ns) / 1e6, quantile(0.95)(duration_ns) / 1e6, quantile(0.99)(duration_ns) / 1e6,
		       min(start_time), max(start_time)
		FROM spans WHERE tenant_id = ? AND start_time >= ? AND `+isEntry+`
		GROUP BY service ORDER BY count() DESC LIMIT 500`, tenant(r), since)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	out := []serviceRow{}
	for rows.Next() {
		var s serviceRow
		if err := rows.Scan(&s.Name, &s.Spans, &s.Errors, &s.P50Ms, &s.P95Ms, &s.P99Ms, &s.FirstSeen, &s.LastSeen); err != nil {
			fail(w, err)
			return
		}
		if s.Spans > 0 {
			s.ErrorRate = float64(s.Errors) / float64(s.Spans)
		}
		out = append(out, s)
	}
	authn.WriteJSON(w, 200, out)
}

type point struct {
	T      time.Time `json:"t"`
	Count  uint64    `json:"count"`
	Errors uint64    `json:"errors"`
	P95Ms  float64   `json:"p95Ms"`
}

func (a *API) serviceSeries(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !nameRe.MatchString(name) {
		authn.WriteError(w, 400, "invalid_request", "invalid service name")
		return
	}
	since, d := window(r)
	rows, err := a.CH.Conn().Query(r.Context(), fmt.Sprintf(`
		SELECT toStartOfInterval(start_time, INTERVAL %d SECOND) AS t, count(), countIf(status_code = 2), quantile(0.95)(duration_ns) / 1e6
		FROM spans WHERE tenant_id = ? AND service = ? AND start_time >= ? AND `+isEntry+`
		GROUP BY t ORDER BY t`, bucketSeconds(d)), tenant(r), name, since)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	out := []point{}
	for rows.Next() {
		var p point
		if err := rows.Scan(&p.T, &p.Count, &p.Errors, &p.P95Ms); err != nil {
			fail(w, err)
			return
		}
		out = append(out, p)
	}
	authn.WriteJSON(w, 200, out)
}

type edge struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Calls  uint64  `json:"calls"`
	Errors uint64  `json:"errors"`
	P95Ms  float64 `json:"p95Ms"`
	Kind   string  `json:"kind"` // "service" or "external"
}

// Edges returns observed dependencies: cross-service parent->child span links,
// plus client spans that name a database or peer service.
func Edges(ctx context.Context, ch *store.CH, tenantID string, since, until time.Time) ([]edge, error) {
	out := []edge{}
	rows, err := ch.Conn().Query(ctx, `
		SELECT p.service, c.service, count(), countIf(c.status_code = 2), quantile(0.95)(c.duration_ns) / 1e6
		FROM spans AS c INNER JOIN spans AS p
		  ON c.tenant_id = p.tenant_id AND c.trace_id = p.trace_id AND c.parent_span_id = p.span_id
		WHERE c.tenant_id = ? AND c.start_time >= ? AND c.start_time < ? AND p.tenant_id = ? AND p.start_time >= ? AND p.start_time < ? AND p.service != c.service
		GROUP BY p.service, c.service ORDER BY count() DESC LIMIT 1000`, tenantID, since, until, tenantID, since, until)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		e := edge{Kind: "service"}
		if err := rows.Scan(&e.Source, &e.Target, &e.Calls, &e.Errors, &e.P95Ms); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, e)
	}
	rows.Close()

	rows, err = ch.Conn().Query(ctx, `
		SELECT service, if(attrs['peer.service'] != '', attrs['peer.service'], attrs['db.system']) AS target,
		       count(), countIf(status_code = 2), quantile(0.95)(duration_ns) / 1e6
		FROM spans WHERE tenant_id = ? AND start_time >= ? AND start_time < ? AND kind = 3
		  AND (attrs['peer.service'] != '' OR attrs['db.system'] != '')
		GROUP BY service, target ORDER BY count() DESC LIMIT 1000`, tenantID, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		e := edge{Kind: "external"}
		if err := rows.Scan(&e.Source, &e.Target, &e.Calls, &e.Errors, &e.P95Ms); err != nil {
			return nil, err
		}
		if e.Source != e.Target {
			out = append(out, e)
		}
	}
	return out, nil
}

func (a *API) dependencies(w http.ResponseWriter, r *http.Request) {
	since, _ := window(r)
	edges, err := Edges(r.Context(), a.CH, tenant(r), since, time.Now().Add(time.Minute))
	if err != nil {
		fail(w, err)
		return
	}
	authn.WriteJSON(w, 200, edges)
}

type traceRow struct {
	TraceID    string    `json:"traceId"`
	Service    string    `json:"service"`
	Name       string    `json:"name"`
	Start      time.Time `json:"start"`
	DurationMs float64   `json:"durationMs"`
	Spans      uint64    `json:"spans"`
	Errors     uint64    `json:"errors"`
	Services   uint64    `json:"services"`
}

func (a *API) traces(w http.ResponseWriter, r *http.Request) {
	since, _ := window(r)
	q := r.URL.Query()
	args := []any{tenant(r), since}
	var sb strings.Builder
	sb.WriteString(`
		SELECT trace_id, argMin(service, start_time), argMin(name, start_time), min(start_time),
		       (max(toUnixTimestamp64Nano(start_time) + toInt64(duration_ns)) - min(toUnixTimestamp64Nano(start_time))) / 1e6 AS dur_ms,
		       count(), countIf(status_code = 2) AS errs, uniqExact(service)
		FROM spans WHERE tenant_id = ? AND start_time >= ?`)
	if svc := q.Get("service"); svc != "" {
		if !nameRe.MatchString(svc) {
			authn.WriteError(w, 400, "invalid_request", "invalid service")
			return
		}
		sb.WriteString(` AND trace_id IN (SELECT trace_id FROM spans WHERE tenant_id = ? AND service = ? AND start_time >= ?)`)
		args = append(args, tenant(r), svc, since)
	}
	sb.WriteString(` GROUP BY trace_id`)
	var having []string
	if q.Get("status") == "error" {
		having = append(having, "errs > 0")
	}
	if v, err := strconv.ParseFloat(q.Get("minDurationMs"), 64); err == nil && v > 0 {
		having = append(having, "dur_ms >= ?")
		args = append(args, v)
	}
	if len(having) > 0 {
		sb.WriteString(" HAVING " + strings.Join(having, " AND "))
	}
	sb.WriteString(" ORDER BY min(start_time) DESC LIMIT ?")
	args = append(args, limit(r, 50, 200))

	rows, err := a.CH.Conn().Query(r.Context(), sb.String(), args...)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	out := []traceRow{}
	for rows.Next() {
		var t traceRow
		if err := rows.Scan(&t.TraceID, &t.Service, &t.Name, &t.Start, &t.DurationMs, &t.Spans, &t.Errors, &t.Services); err != nil {
			fail(w, err)
			return
		}
		out = append(out, t)
	}
	authn.WriteJSON(w, 200, out)
}

type spanRow struct {
	SpanID       string            `json:"spanId"`
	ParentSpanID string            `json:"parentSpanId"`
	Service      string            `json:"service"`
	Name         string            `json:"name"`
	Kind         uint8             `json:"kind"`
	Start        time.Time         `json:"start"`
	DurationMs   float64           `json:"durationMs"`
	StatusCode   uint8             `json:"statusCode"`
	StatusMsg    string            `json:"statusMessage"`
	Attrs        map[string]string `json:"attrs"`
}

var hexRe = regexp.MustCompile(`^[0-9a-f]{1,32}$`)

func (a *API) trace(w http.ResponseWriter, r *http.Request) {
	id := strings.ToLower(r.PathValue("traceId"))
	if !hexRe.MatchString(id) {
		authn.WriteError(w, 400, "invalid_request", "invalid trace id")
		return
	}
	rows, err := a.CH.Conn().Query(r.Context(), `
		SELECT span_id, parent_span_id, service, name, kind, start_time, duration_ns / 1e6, status_code, status_message, attrs
		FROM spans WHERE tenant_id = ? AND trace_id = ? ORDER BY start_time LIMIT 5000`, tenant(r), id)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	out := []spanRow{}
	for rows.Next() {
		var s spanRow
		if err := rows.Scan(&s.SpanID, &s.ParentSpanID, &s.Service, &s.Name, &s.Kind, &s.Start, &s.DurationMs, &s.StatusCode, &s.StatusMsg, &s.Attrs); err != nil {
			fail(w, err)
			return
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		authn.WriteError(w, 404, "not_found", "trace not found")
		return
	}
	authn.WriteJSON(w, 200, out)
}

type logRow struct {
	TS       time.Time `json:"ts"`
	Service  string    `json:"service"`
	Severity uint8     `json:"severityNumber"`
	SevText  string    `json:"severityText"`
	Body     string    `json:"body"`
	TraceID  string    `json:"traceId"`
	SpanID   string    `json:"spanId"`
}

func (a *API) logs(w http.ResponseWriter, r *http.Request) {
	since, _ := window(r)
	q := r.URL.Query()
	args := []any{tenant(r), since}
	sql := `SELECT ts, service, severity_number, severity_text, body, trace_id, span_id FROM logs WHERE tenant_id = ? AND ts >= ?`
	if svc := q.Get("service"); svc != "" {
		if !nameRe.MatchString(svc) {
			authn.WriteError(w, 400, "invalid_request", "invalid service")
			return
		}
		sql += " AND service = ?"
		args = append(args, svc)
	}
	if n, err := strconv.Atoi(q.Get("minSeverity")); err == nil && n > 0 {
		sql += " AND severity_number >= ?"
		args = append(args, n)
	}
	if s := q.Get("q"); s != "" {
		sql += " AND positionCaseInsensitive(body, ?) > 0"
		args = append(args, s)
	}
	sql += " ORDER BY ts DESC LIMIT ?"
	args = append(args, limit(r, 100, 500))
	rows, err := a.CH.Conn().Query(r.Context(), sql, args...)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	out := []logRow{}
	for rows.Next() {
		var l logRow
		if err := rows.Scan(&l.TS, &l.Service, &l.Severity, &l.SevText, &l.Body, &l.TraceID, &l.SpanID); err != nil {
			fail(w, err)
			return
		}
		out = append(out, l)
	}
	authn.WriteJSON(w, 200, out)
}

func (a *API) metricNames(w http.ResponseWriter, r *http.Request) {
	since, _ := window(r)
	rows, err := a.CH.Conn().Query(r.Context(), `
		SELECT service, name, any(unit), any(kind) FROM metrics WHERE tenant_id = ? AND ts >= ?
		GROUP BY service, name ORDER BY service, name LIMIT 1000`, tenant(r), since)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	type m struct {
		Service string `json:"service"`
		Name    string `json:"name"`
		Unit    string `json:"unit"`
		Kind    string `json:"kind"`
	}
	out := []m{}
	for rows.Next() {
		var x m
		if err := rows.Scan(&x.Service, &x.Name, &x.Unit, &x.Kind); err != nil {
			fail(w, err)
			return
		}
		out = append(out, x)
	}
	authn.WriteJSON(w, 200, out)
}

func (a *API) metricSeries(w http.ResponseWriter, r *http.Request) {
	since, d := window(r)
	svc, name := r.URL.Query().Get("service"), r.URL.Query().Get("name")
	if !nameRe.MatchString(svc) || !nameRe.MatchString(name) {
		authn.WriteError(w, 400, "invalid_request", "service and name are required")
		return
	}
	rows, err := a.CH.Conn().Query(r.Context(), fmt.Sprintf(`
		SELECT toStartOfInterval(ts, INTERVAL %d SECOND) AS t, avg(value) FROM metrics
		WHERE tenant_id = ? AND service = ? AND name = ? AND ts >= ? GROUP BY t ORDER BY t`, bucketSeconds(d)),
		tenant(r), svc, name, since)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	type p struct {
		T     time.Time `json:"t"`
		Value float64   `json:"value"`
	}
	out := []p{}
	for rows.Next() {
		var x p
		if err := rows.Scan(&x.T, &x.Value); err != nil {
			fail(w, err)
			return
		}
		out = append(out, x)
	}
	authn.WriteJSON(w, 200, out)
}
