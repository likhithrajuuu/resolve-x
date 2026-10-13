// Package assistant is the AI chat: a tool-using agent that answers questions
// about an organization's telemetry and incidents.
//
// Safety model: every tool is a read-only GET against Resolve-X's own APIs,
// called with the *user's own token*, so the assistant can see exactly what that
// user can see and nothing else. It has no write, delete or remediation tools.
package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/resolvex/resolve-x/backend/internal/llm"
)

type Backend struct {
	QueryURL    string
	IncidentURL string
	HTTP        *http.Client
}

var (
	nameRe   = regexp.MustCompile(`^[\w.\-:/ ]{1,200}$`)
	windowRe = regexp.MustCompile(`^(\d{1,4})([mhd])$`)
	idRe     = regexp.MustCompile(`^[a-z0-9_]{1,80}$`)
	hexRe    = regexp.MustCompile(`^[0-9a-fA-F]{1,32}$`)
)

type toolDef struct {
	llm.Tool
	run func(ctx context.Context, b *Backend, token string, args map[string]any) ([]byte, error)
}

func obj(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

var (
	pWindow  = map[string]any{"type": "string", "description": "Look-back window like 15m, 1h, 6h, 24h or 7d. Default 1h."}
	pService = map[string]any{"type": "string", "description": "Service name exactly as returned by list_services."}
)

func str(a map[string]any, k string) string { s, _ := a[k].(string); return s }

func windowArg(a map[string]any) (string, error) {
	w := str(a, "window")
	if w == "" {
		return "1h", nil
	}
	m := windowRe.FindStringSubmatch(w)
	if m == nil {
		return "", fmt.Errorf("window must look like 15m, 1h, 6h, 24h or 7d")
	}
	n, _ := strconv.Atoi(m[1])
	if m[2] == "d" {
		return fmt.Sprintf("%dh", n*24), nil
	}
	return w, nil
}

func serviceArg(a map[string]any, required bool) (string, error) {
	s := str(a, "service")
	if s == "" && !required {
		return "", nil
	}
	if !nameRe.MatchString(s) {
		return "", fmt.Errorf("service is missing or invalid")
	}
	return s, nil
}

func intArg(a map[string]any, k string, def, max int) int {
	if f, ok := a[k].(float64); ok && f > 0 {
		return min(int(f), max)
	}
	return def
}

func (b *Backend) get(ctx context.Context, base, path string, q url.Values, token string) ([]byte, error) {
	u := base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil) // GET only: tools cannot change anything
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := b.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("data service unavailable")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("not found")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request failed (HTTP %d)", resp.StatusCode)
	}
	return body, nil
}

func windowQ(a map[string]any) (url.Values, error) {
	w, err := windowArg(a)
	if err != nil {
		return nil, err
	}
	return url.Values{"window": {w}}, nil
}

var toolDefs = []toolDef{
	{Tool: llm.Tool{Name: "list_services", Description: "List services with request count, error rate and p50/p95/p99 latency over a window. Start here to see what exists and what looks unhealthy.", Schema: obj(map[string]any{"window": pWindow})},
		run: func(ctx context.Context, b *Backend, tok string, a map[string]any) ([]byte, error) {
			q, err := windowQ(a)
			if err != nil {
				return nil, err
			}
			return b.get(ctx, b.QueryURL, "/api/v1/services", q, tok)
		}},
	{Tool: llm.Tool{Name: "get_service_operations", Description: "Per-operation (endpoint) calls, error rate and latency for one service.", Schema: obj(map[string]any{"service": pService, "window": pWindow}, "service")},
		run: func(ctx context.Context, b *Backend, tok string, a map[string]any) ([]byte, error) {
			s, err := serviceArg(a, true)
			if err != nil {
				return nil, err
			}
			q, err := windowQ(a)
			if err != nil {
				return nil, err
			}
			return b.get(ctx, b.QueryURL, "/api/v1/services/"+url.PathEscape(s)+"/operations", q, tok)
		}},
	{Tool: llm.Tool{Name: "get_dependencies", Description: "Observed calls between services and to databases/external systems, with call counts, error counts and p95 latency.", Schema: obj(map[string]any{"window": pWindow})},
		run: func(ctx context.Context, b *Backend, tok string, a map[string]any) ([]byte, error) {
			q, err := windowQ(a)
			if err != nil {
				return nil, err
			}
			return b.get(ctx, b.QueryURL, "/api/v1/dependencies", q, tok)
		}},
	{Tool: llm.Tool{Name: "search_logs", Description: "Search log lines, newest first. Severity 17+ is error, 13+ is warning.", Schema: obj(map[string]any{
		"service": pService, "min_severity": map[string]any{"type": "integer", "description": "17 for errors only, 13 for warnings and above."},
		"text": map[string]any{"type": "string", "description": "Case-insensitive substring to look for."}, "window": pWindow, "limit": map[string]any{"type": "integer", "description": "Max lines, default 25, max 60."}})},
		run: func(ctx context.Context, b *Backend, tok string, a map[string]any) ([]byte, error) {
			q, err := windowQ(a)
			if err != nil {
				return nil, err
			}
			s, err := serviceArg(a, false)
			if err != nil {
				return nil, err
			}
			if s != "" {
				q.Set("service", s)
			}
			if n := intArg(a, "min_severity", 0, 24); n > 0 {
				q.Set("minSeverity", strconv.Itoa(n))
			}
			if t := str(a, "text"); t != "" {
				q.Set("q", t[:min(len(t), 200)])
			}
			q.Set("limit", strconv.Itoa(intArg(a, "limit", 25, 60)))
			return b.get(ctx, b.QueryURL, "/api/v1/logs", q, tok)
		}},
	{Tool: llm.Tool{Name: "log_patterns", Description: "Group log messages into patterns (numbers and IDs masked) with counts. Best for 'what errors are most common'.", Schema: obj(map[string]any{"service": pService, "window": pWindow})},
		run: func(ctx context.Context, b *Backend, tok string, a map[string]any) ([]byte, error) {
			q, err := windowQ(a)
			if err != nil {
				return nil, err
			}
			s, err := serviceArg(a, false)
			if err != nil {
				return nil, err
			}
			if s != "" {
				q.Set("service", s)
			}
			return b.get(ctx, b.QueryURL, "/api/v1/logs/patterns", q, tok)
		}},
	{Tool: llm.Tool{Name: "search_traces", Description: "Find traces, newest first, with duration, span count and error count.", Schema: obj(map[string]any{
		"service": pService, "errors_only": map[string]any{"type": "boolean"}, "min_duration_ms": map[string]any{"type": "number"}, "window": pWindow, "limit": map[string]any{"type": "integer", "description": "Default 15, max 40."}})},
		run: func(ctx context.Context, b *Backend, tok string, a map[string]any) ([]byte, error) {
			q, err := windowQ(a)
			if err != nil {
				return nil, err
			}
			s, err := serviceArg(a, false)
			if err != nil {
				return nil, err
			}
			if s != "" {
				q.Set("service", s)
			}
			if e, _ := a["errors_only"].(bool); e {
				q.Set("status", "error")
			}
			if d, ok := a["min_duration_ms"].(float64); ok && d > 0 {
				q.Set("minDurationMs", strconv.FormatFloat(d, 'f', -1, 64))
			}
			q.Set("limit", strconv.Itoa(intArg(a, "limit", 15, 40)))
			return b.get(ctx, b.QueryURL, "/api/v1/traces", q, tok)
		}},
	{Tool: llm.Tool{Name: "get_trace", Description: "All spans of one trace (service, operation, duration, status, error message).", Schema: obj(map[string]any{"trace_id": map[string]any{"type": "string"}}, "trace_id")},
		run: func(ctx context.Context, b *Backend, tok string, a map[string]any) ([]byte, error) {
			id := str(a, "trace_id")
			if !hexRe.MatchString(id) {
				return nil, fmt.Errorf("trace_id must be hex")
			}
			return b.get(ctx, b.QueryURL, "/api/v1/traces/"+id, nil, tok)
		}},
	{Tool: llm.Tool{Name: "list_metrics", Description: "Metric names available per service.", Schema: obj(map[string]any{"window": pWindow})},
		run: func(ctx context.Context, b *Backend, tok string, a map[string]any) ([]byte, error) {
			q, err := windowQ(a)
			if err != nil {
				return nil, err
			}
			return b.get(ctx, b.QueryURL, "/api/v1/metrics", q, tok)
		}},
	{Tool: llm.Tool{Name: "metric_series", Description: "A metric over time, aggregated. Returns a summary (min, max, avg, first, last) and a downsampled series per group.", Schema: obj(map[string]any{
		"name": map[string]any{"type": "string"}, "service": pService, "window": pWindow,
		"agg":      map[string]any{"type": "string", "enum": []string{"avg", "sum", "max", "min", "p50", "p95", "p99", "rate", "count"}},
		"group_by": map[string]any{"type": "string", "description": "Attribute key, or 'service'."}}, "name")},
		run: func(ctx context.Context, b *Backend, tok string, a map[string]any) ([]byte, error) {
			q, err := windowQ(a)
			if err != nil {
				return nil, err
			}
			n := str(a, "name")
			if !nameRe.MatchString(n) {
				return nil, fmt.Errorf("name is required")
			}
			q.Set("name", n)
			if s, err := serviceArg(a, false); err != nil {
				return nil, err
			} else if s != "" {
				q.Set("service", s)
			}
			if ag := str(a, "agg"); ag != "" {
				q.Set("agg", ag)
			}
			if g := str(a, "group_by"); g != "" {
				if !nameRe.MatchString(g) {
					return nil, fmt.Errorf("group_by invalid")
				}
				q.Set("groupBy", g)
			}
			raw, err := b.get(ctx, b.QueryURL, "/api/v1/metrics/series", q, tok)
			if err != nil {
				return nil, err
			}
			return summariseSeries(raw), nil
		}},
	{Tool: llm.Tool{Name: "list_incidents", Description: "Incidents, newest first. Optionally filter by status: open, investigating, identified, resolved.", Schema: obj(map[string]any{"status": map[string]any{"type": "string", "enum": []string{"open", "investigating", "identified", "resolved"}}})},
		run: func(ctx context.Context, b *Backend, tok string, a map[string]any) ([]byte, error) {
			q := url.Values{}
			if s := str(a, "status"); s != "" {
				q.Set("status", s)
			}
			return b.get(ctx, b.IncidentURL, "/api/v1/incidents", q, tok)
		}},
	{Tool: llm.Tool{Name: "get_incident", Description: "One incident with its timeline and every analysis (root cause, confidence, evidence, recommended action, approval state).", Schema: obj(map[string]any{"id": map[string]any{"type": "string"}}, "id")},
		run: func(ctx context.Context, b *Backend, tok string, a map[string]any) ([]byte, error) {
			id := str(a, "id")
			if !idRe.MatchString(id) {
				return nil, fmt.Errorf("invalid incident id")
			}
			return b.get(ctx, b.IncidentURL, "/api/v1/incidents/"+id, nil, tok)
		}},
	{Tool: llm.Tool{Name: "list_deployments", Description: "Recent deployments (service, version, time). Use to check what changed before a problem started.", Schema: obj(map[string]any{})},
		run: func(ctx context.Context, b *Backend, tok string, _ map[string]any) ([]byte, error) {
			return b.get(ctx, b.IncidentURL, "/api/v1/deployments", nil, tok)
		}},
	{Tool: llm.Tool{Name: "list_slos", Description: "SLOs with current SLI, error budget remaining and burn rate.", Schema: obj(map[string]any{})},
		run: func(ctx context.Context, b *Backend, tok string, _ map[string]any) ([]byte, error) {
			return b.get(ctx, b.IncidentURL, "/api/v1/slos", nil, tok)
		}},
	{Tool: llm.Tool{Name: "list_alert_rules", Description: "Alert rules and their current state and value.", Schema: obj(map[string]any{})},
		run: func(ctx context.Context, b *Backend, tok string, _ map[string]any) ([]byte, error) {
			return b.get(ctx, b.IncidentURL, "/api/v1/alert-rules", nil, tok)
		}},
}

func Tools() []llm.Tool {
	out := make([]llm.Tool, len(toolDefs))
	for i, t := range toolDefs {
		out[i] = t.Tool
	}
	return out
}

const maxToolBytes = 7000

// shrink keeps tool output small enough for a model context: arrays are cut to
// the elements that fit, with an explicit marker so the model knows.
func shrink(raw []byte) string {
	if len(raw) <= maxToolBytes {
		return string(raw)
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		kept, size := 0, 0
		for _, e := range arr {
			if size+len(e)+1 > maxToolBytes {
				break
			}
			size += len(e) + 1
			kept++
		}
		b, _ := json.Marshal(map[string]any{"items": arr[:kept], "truncated": true, "shownItems": kept, "totalItems": len(arr), "note": "Narrow the window or filters to see more."})
		return string(b)
	}
	return string(raw[:maxToolBytes]) + `…[truncated]`
}

// summariseSeries reduces each series to statistics plus at most 12 points.
func summariseSeries(raw []byte) []byte {
	var in []struct {
		Group  string `json:"group"`
		Points []struct {
			T     string  `json:"t"`
			Value float64 `json:"value"`
		} `json:"points"`
	}
	if json.Unmarshal(raw, &in) != nil {
		return raw
	}
	type pt struct {
		T string  `json:"t"`
		V float64 `json:"v"`
	}
	type out struct {
		Group  string  `json:"group"`
		Count  int     `json:"points"`
		Min    float64 `json:"min"`
		Max    float64 `json:"max"`
		Avg    float64 `json:"avg"`
		First  float64 `json:"first"`
		Last   float64 `json:"last"`
		Sample []pt    `json:"sample"`
	}
	res := []out{}
	for _, s := range in {
		if len(s.Points) == 0 {
			continue
		}
		o := out{Group: s.Group, Count: len(s.Points), Min: s.Points[0].Value, Max: s.Points[0].Value, First: s.Points[0].Value, Last: s.Points[len(s.Points)-1].Value}
		sum := 0.0
		for _, p := range s.Points {
			o.Min, o.Max = min(o.Min, p.Value), max(o.Max, p.Value)
			sum += p.Value
		}
		o.Avg = sum / float64(len(s.Points))
		step := max(len(s.Points)/12, 1)
		for i := 0; i < len(s.Points); i += step {
			o.Sample = append(o.Sample, pt{s.Points[i].T, s.Points[i].Value})
		}
		res = append(res, o)
	}
	b, _ := json.Marshal(res)
	return b
}

// Run executes one tool call and returns its (possibly shrunk) output.
func (b *Backend) Run(ctx context.Context, token string, c llm.Call) (string, error) {
	for _, t := range toolDefs {
		if t.Name != c.Name {
			continue
		}
		args := map[string]any{}
		if len(c.Args) > 0 && json.Unmarshal(c.Args, &args) != nil {
			return "", fmt.Errorf("arguments are not valid JSON")
		}
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		raw, err := t.run(ctx, b, token, args)
		if err != nil {
			return "", err
		}
		return shrink(raw), nil
	}
	return "", fmt.Errorf("unknown tool %q", strings.TrimSpace(c.Name))
}
