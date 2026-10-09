package incident

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/resolvex/resolve-x/backend/internal/store"
)

type SLO struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Service    string   `json:"service"`
	Kind       string   `json:"kind"`
	Objective  float64  `json:"objective"` // percent, e.g. 99.9
	LatencyMs  *float64 `json:"latencyMs,omitempty"`
	WindowDays int      `json:"windowDays"`
}

// SLOStatus is an SLO plus its computed attainment.
type SLOStatus struct {
	SLO
	Total           uint64  `json:"total"`
	Bad             uint64  `json:"bad"`
	SLI             float64 `json:"sli"`             // percent good
	BudgetRemaining float64 `json:"budgetRemaining"` // percent of the error budget left; negative = overspent
	BurnRate1h      float64 `json:"burnRate1h"`      // 1.0 = spending budget exactly on pace
	Status          string  `json:"status"`          // ok | warning | breached | nodata
}

func (s *SLO) Validate() error {
	if s.Name == "" || s.Service == "" {
		return errors.New("name and service are required")
	}
	if s.Objective <= 0 || s.Objective >= 100 {
		return errors.New("objective must be between 0 and 100 (exclusive), e.g. 99.9")
	}
	switch s.Kind {
	case "availability":
		s.LatencyMs = nil
	case "latency":
		if s.LatencyMs == nil || *s.LatencyMs <= 0 {
			return errors.New("latency SLOs need latencyMs")
		}
	default:
		return errors.New("kind must be availability or latency")
	}
	if s.WindowDays == 0 {
		s.WindowDays = 7
	}
	if s.WindowDays < 1 || s.WindowDays > 30 {
		return errors.New("windowDays must be 1-30")
	}
	return nil
}

func (s *Service) ListSLOs(ctx context.Context, tenant string) ([]SLO, error) {
	rows, err := s.DB.Query(ctx, `SELECT id, name, service, kind, objective, latency_ms, window_days FROM slos WHERE tenant_id=$1 ORDER BY created_at`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SLO{}
	for rows.Next() {
		var o SLO
		if err := rows.Scan(&o.ID, &o.Name, &o.Service, &o.Kind, &o.Objective, &o.LatencyMs, &o.WindowDays); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Service) CreateSLO(ctx context.Context, tenant string, o SLO) (SLO, error) {
	if err := o.Validate(); err != nil {
		return o, err
	}
	o.ID = newID("slo")
	_, err := s.DB.Exec(ctx, `INSERT INTO slos (id, tenant_id, name, service, kind, objective, latency_ms, window_days) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		o.ID, tenant, o.Name, o.Service, o.Kind, o.Objective, o.LatencyMs, o.WindowDays)
	return o, err
}

func (s *Service) DeleteSLO(ctx context.Context, tenant, id string) error {
	tag, err := s.DB.Exec(ctx, `DELETE FROM slos WHERE id=$1 AND tenant_id=$2`, id, tenant)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

var _ = pgx.ErrNoRows

// Evaluate computes attainment, remaining error budget and the 1-hour burn rate.
func Evaluate(ctx context.Context, ch *store.CH, tenant string, o SLO, now time.Time) (SLOStatus, error) {
	good := "status_code != 2"
	args := func(from time.Time) []any { return []any{tenant, o.Service, from} }
	if o.Kind == "latency" {
		good = "status_code != 2 AND duration_ns <= " + floatLit(*o.LatencyMs*1e6)
	}
	q := `SELECT count(), countIf(NOT (` + good + `)) FROM spans WHERE tenant_id = ? AND service = ? AND start_time >= ? AND (kind = 2 OR parent_span_id = '')`
	st := SLOStatus{SLO: o}
	if err := ch.Conn().QueryRow(ctx, q, args(now.AddDate(0, 0, -o.WindowDays))...).Scan(&st.Total, &st.Bad); err != nil {
		return st, err
	}
	if st.Total == 0 {
		st.Status = "nodata"
		return st, nil
	}
	allowed := 1 - o.Objective/100
	st.SLI = 100 * (1 - float64(st.Bad)/float64(st.Total))
	st.BudgetRemaining = 100 * (1 - (float64(st.Bad)/float64(st.Total))/allowed)
	var total1h, bad1h uint64
	if err := ch.Conn().QueryRow(ctx, q, args(now.Add(-time.Hour))...).Scan(&total1h, &bad1h); err != nil {
		return st, err
	}
	if total1h > 0 {
		st.BurnRate1h = (float64(bad1h) / float64(total1h)) / allowed
	}
	switch {
	case st.BudgetRemaining < 0:
		st.Status = "breached"
	case st.BurnRate1h >= 2 || st.BudgetRemaining < 25:
		st.Status = "warning"
	default:
		st.Status = "ok"
	}
	return st, nil
}

// floatLit renders a float safely for SQL (it is computed from a validated number).
func floatLit(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "0"
	}
	return formatFloat(f)
}

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
