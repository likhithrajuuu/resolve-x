package incident

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/resolvex/resolve-x/backend/internal/store"
)

type Rule struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Kind          string     `json:"kind"`
	Service       string     `json:"service"`
	Metric        string     `json:"metric"`
	Agg           string     `json:"agg"`
	Op            string     `json:"op"`
	Threshold     float64    `json:"threshold"`
	WindowMinutes int        `json:"windowMinutes"`
	Severity      string     `json:"severity"`
	Enabled       bool       `json:"enabled"`
	LastState     string     `json:"lastState"`
	LastValue     *float64   `json:"lastValue,omitempty"`
	LastEvalAt    *time.Time `json:"lastEvalAt,omitempty"`
}

const ruleCols = `id, name, kind, service, metric, agg, op, threshold, window_minutes, severity, enabled, last_state, last_value, last_eval_at`

func scanRule(row pgx.Row) (Rule, error) {
	var r Rule
	err := row.Scan(&r.ID, &r.Name, &r.Kind, &r.Service, &r.Metric, &r.Agg, &r.Op, &r.Threshold, &r.WindowMinutes, &r.Severity, &r.Enabled, &r.LastState, &r.LastValue, &r.LastEvalAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

var metricAggs = map[string]string{"avg": "avg(value)", "sum": "sum(value)", "max": "max(value)", "min": "min(value)", "p95": "quantile(0.95)(value)"}

// Validate normalises and checks a rule definition.
func (r *Rule) Validate() error {
	if r.Name == "" || len(r.Name) > 120 {
		return errors.New("name is required (max 120 characters)")
	}
	switch r.Kind {
	case "error_rate":
		if r.Service == "" || r.Threshold <= 0 || r.Threshold > 1 {
			return errors.New("error_rate needs a service and a threshold between 0 and 1")
		}
	case "latency_p95":
		if r.Service == "" || r.Threshold <= 0 {
			return errors.New("latency_p95 needs a service and a threshold in ms")
		}
	case "metric":
		if _, ok := metricAggs[r.Agg]; !ok || r.Metric == "" {
			return errors.New("metric rules need a metric name and agg of avg, sum, max, min or p95")
		}
	default:
		return errors.New("kind must be error_rate, latency_p95 or metric")
	}
	if r.Op == "" {
		r.Op = ">"
	}
	if r.Op != ">" && r.Op != "<" {
		return errors.New("op must be > or <")
	}
	if r.WindowMinutes == 0 {
		r.WindowMinutes = 5
	}
	if r.WindowMinutes < 1 || r.WindowMinutes > 1440 {
		return errors.New("windowMinutes must be 1-1440")
	}
	if !severities[r.Severity] {
		r.Severity = "SEV-2"
	}
	return nil
}

func (s *Service) ListRules(ctx context.Context, tenant string) ([]Rule, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+ruleCols+` FROM alert_rules WHERE tenant_id=$1 ORDER BY created_at DESC`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) CreateRule(ctx context.Context, tenant string, r Rule) (Rule, error) {
	if err := r.Validate(); err != nil {
		return r, err
	}
	return scanRule(s.DB.QueryRow(ctx, `INSERT INTO alert_rules (id, tenant_id, name, kind, service, metric, agg, op, threshold, window_minutes, severity, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,true) RETURNING `+ruleCols,
		newID("rul"), tenant, r.Name, r.Kind, r.Service, r.Metric, orDefault(r.Agg, "avg"), r.Op, r.Threshold, r.WindowMinutes, r.Severity))
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func (s *Service) SetRuleEnabled(ctx context.Context, tenant, id string, enabled bool) error {
	tag, err := s.DB.Exec(ctx, `UPDATE alert_rules SET enabled=$3 WHERE id=$1 AND tenant_id=$2`, id, tenant, enabled)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Service) DeleteRule(ctx context.Context, tenant, id string) error {
	tag, err := s.DB.Exec(ctx, `DELETE FROM alert_rules WHERE id=$1 AND tenant_id=$2`, id, tenant)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// Evaluator evaluates every enabled alert rule on an interval and opens
// incidents (deduplicated by rule) for breaching rules.
type Evaluator struct {
	Svc      *Service
	CH       *store.CH
	Interval time.Duration
}

func (e *Evaluator) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := e.once(ctx); err != nil && ctx.Err() == nil {
			e.Svc.Log.Error("rule evaluator", "err", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(e.Interval):
		}
	}
}

type ruleRow struct {
	tenant string
	Rule
}

func (e *Evaluator) once(ctx context.Context) error {
	rows, err := e.Svc.DB.Query(ctx, `SELECT tenant_id, `+ruleCols+` FROM alert_rules WHERE enabled`)
	if err != nil {
		return err
	}
	var rules []ruleRow
	for rows.Next() {
		var rr ruleRow
		r := &rr.Rule
		if err := rows.Scan(&rr.tenant, &r.ID, &r.Name, &r.Kind, &r.Service, &r.Metric, &r.Agg, &r.Op, &r.Threshold, &r.WindowMinutes, &r.Severity, &r.Enabled, &r.LastState, &r.LastValue, &r.LastEvalAt); err != nil {
			rows.Close()
			return err
		}
		rules = append(rules, rr)
	}
	rows.Close()
	for _, rr := range rules {
		val, ok, err := e.value(ctx, rr)
		if err != nil {
			e.Svc.Log.Warn("rule evaluation failed", "rule", rr.ID, "err", err)
			continue
		}
		state := "ok"
		if !ok {
			state = "nodata"
		} else if rr.Op == ">" && val > rr.Threshold || rr.Op == "<" && val < rr.Threshold {
			state = "firing"
		}
		if _, err := e.Svc.DB.Exec(ctx, `UPDATE alert_rules SET last_state=$2, last_value=$3, last_eval_at=now() WHERE id=$1`, rr.ID, state, nullable(val, ok)); err != nil {
			return err
		}
		if state == "firing" {
			if _, _, err := e.Svc.Create(ctx, rr.tenant, NewIncident{
				Title: fmt.Sprintf("Alert: %s", rr.Name), Service: rr.Service, Severity: rr.Severity, Source: "rule",
				Description: fmt.Sprintf("%s: value %.4g is %s the threshold %.4g over the last %d min.", rr.Kind, val, map[string]string{">": "above", "<": "below"}[rr.Op], rr.Threshold, rr.WindowMinutes),
				Fingerprint: "rule:" + rr.ID}); err != nil {
				return err
			}
		}
	}
	return nil
}

func nullable(v float64, ok bool) any {
	if !ok {
		return nil
	}
	return v
}

// value computes the rule's current value; ok is false when there is no data.
func (e *Evaluator) value(ctx context.Context, rr ruleRow) (float64, bool, error) {
	since := time.Now().Add(-time.Duration(rr.WindowMinutes) * time.Minute)
	switch rr.Kind {
	case "error_rate", "latency_p95":
		var n uint64
		var v float64
		expr := "countIf(status_code = 2) / count()"
		if rr.Kind == "latency_p95" {
			expr = "quantile(0.95)(duration_ns) / 1e6"
		}
		err := e.CH.Conn().QueryRow(ctx, `SELECT count(), `+expr+` FROM spans
			WHERE tenant_id = ? AND service = ? AND start_time >= ? AND (kind = 2 OR parent_span_id = '')`, rr.tenant, rr.Service, since).Scan(&n, &v)
		return v, n >= 5, err
	default:
		sql := `SELECT count(), ` + metricAggs[rr.Agg] + ` FROM metrics WHERE tenant_id = ? AND name = ? AND ts >= ?`
		args := []any{rr.tenant, rr.Metric, since}
		if rr.Service != "" {
			sql += " AND service = ?"
			args = append(args, rr.Service)
		}
		var n uint64
		var v float64
		err := e.CH.Conn().QueryRow(ctx, sql, args...).Scan(&n, &v)
		return v, n > 0, err
	}
}
