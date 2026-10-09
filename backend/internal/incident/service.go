// Package incident implements the Incident Service: lifecycle, timeline,
// alert deduplication, outbox relay and storage of analysis results.
package incident

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"

	"github.com/resolvex/resolve-x/backend/internal/events"
)

type Service struct {
	DB  *pgxpool.Pool
	Log *slog.Logger
}

type Incident struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Service     string     `json:"service"`
	Severity    string     `json:"severity"`
	Status      string     `json:"status"`
	Source      string     `json:"source"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`
}

type NewIncident struct {
	Title, Service, Severity, Description, Source, Fingerprint string
}

var ErrNotFound = errors.New("not found")

const incidentCols = `id, title, service, severity, status, source, description, created_at, updated_at, resolved_at`

func scanIncident(row pgx.Row) (Incident, error) {
	var i Incident
	err := row.Scan(&i.ID, &i.Title, &i.Service, &i.Severity, &i.Status, &i.Source, &i.Description, &i.CreatedAt, &i.UpdatedAt, &i.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return i, ErrNotFound
	}
	return i, err
}

func newID(prefix string) string { return prefix + "_" + strings.ToLower(ulid.Make().String()) }

func timeline(ctx context.Context, tx pgx.Tx, incidentID, tenant, kind, summary string) error {
	_, err := tx.Exec(ctx, `INSERT INTO incident_timeline (incident_id, tenant_id, kind, summary) VALUES ($1,$2,$3,$4)`, incidentID, tenant, kind, summary)
	return err
}

func outbox(ctx context.Context, tx pgx.Tx, topic, producer, tenant, key string, payload any) error {
	env, err := events.New(topic, producer, tenant, payload)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(env)
	_, err = tx.Exec(ctx, `INSERT INTO outbox (topic, key, envelope) VALUES ($1,$2,$3)`, topic, string(events.Key(tenant, key)), b)
	return err
}

type createdPayload struct {
	IncidentID string    `json:"incidentId"`
	Title      string    `json:"title"`
	Service    string    `json:"service"`
	Severity   string    `json:"severity"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Create opens an incident, or — when the fingerprint matches an unresolved
// incident — records the repeat on that incident's timeline instead.
// The incident row, its timeline entry and the incident.created event commit
// in one transaction (transactional outbox).
func (s *Service) Create(ctx context.Context, tenant string, in NewIncident) (Incident, bool, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return Incident{}, false, err
	}
	defer tx.Rollback(ctx)

	var fp any
	if in.Fingerprint != "" {
		fp = in.Fingerprint
	}
	src := in.Source
	if src == "" {
		src = "manual"
	}
	row := tx.QueryRow(ctx, `
		INSERT INTO incidents (id, tenant_id, title, service, severity, status, source, description, fingerprint)
		VALUES ($1,$2,$3,$4,$5,'investigating',$6,$7,$8)
		ON CONFLICT (tenant_id, fingerprint) WHERE fingerprint IS NOT NULL AND status <> 'resolved' DO NOTHING
		RETURNING `+incidentCols, newID("inc"), tenant, in.Title, in.Service, in.Severity, src, in.Description, fp)
	inc, err := scanIncident(row)
	if errors.Is(err, ErrNotFound) { // deduplicated
		existing, err := scanIncident(tx.QueryRow(ctx, `SELECT `+incidentCols+` FROM incidents
			WHERE tenant_id=$1 AND fingerprint=$2 AND status <> 'resolved'`, tenant, in.Fingerprint))
		if err != nil {
			return Incident{}, false, err
		}
		// Throttled: a detector that re-fires every 30 s must not flood the timeline.
		if _, err := tx.Exec(ctx, `INSERT INTO incident_timeline (incident_id, tenant_id, kind, summary)
			SELECT $1, $2, 'alert', $3 WHERE NOT EXISTS (
				SELECT 1 FROM incident_timeline WHERE incident_id=$1 AND kind='alert' AND at > now() - interval '10 minutes')`,
			existing.ID, tenant, "Condition still present: "+in.Title); err != nil {
			return Incident{}, false, err
		}
		return existing, false, tx.Commit(ctx)
	}
	if err != nil {
		return Incident{}, false, err
	}
	if err := timeline(ctx, tx, inc.ID, tenant, "created", fmt.Sprintf("Incident opened (%s): %s", src, inc.Title)); err != nil {
		return Incident{}, false, err
	}
	if err := outbox(ctx, tx, events.TopicIncidentCreated, "incident-service", tenant, inc.ID,
		createdPayload{inc.ID, inc.Title, inc.Service, inc.Severity, inc.CreatedAt}); err != nil {
		return Incident{}, false, err
	}
	return inc, true, tx.Commit(ctx)
}

func (s *Service) List(ctx context.Context, tenant, status string) ([]Incident, error) {
	q := `SELECT ` + incidentCols + ` FROM incidents WHERE tenant_id=$1`
	args := []any{tenant}
	if status != "" {
		q += " AND status=$2"
		args = append(args, status)
	}
	rows, err := s.DB.Query(ctx, q+" ORDER BY created_at DESC LIMIT 200", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Incident{}
	for rows.Next() {
		i, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

type TimelineEntry struct {
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"`
	Summary string    `json:"summary"`
}

type Analysis struct {
	ID             string          `json:"id"`
	RootCause      string          `json:"rootCause"`
	Confidence     int             `json:"confidence"`
	Evidence       json.RawMessage `json:"evidence"`
	Recommendation json.RawMessage `json:"recommendation"`
	Analyzer       string          `json:"analyzer"`
	Approval       string          `json:"approval"`
	DecidedBy      *string         `json:"decidedBy,omitempty"`
	DecidedAt      *time.Time      `json:"decidedAt,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
}

type Detail struct {
	Incident
	Timeline []TimelineEntry `json:"timeline"`
	Analyses []Analysis      `json:"analyses"`
}

func (s *Service) Get(ctx context.Context, tenant, id string) (Detail, error) {
	inc, err := scanIncident(s.DB.QueryRow(ctx, `SELECT `+incidentCols+` FROM incidents WHERE id=$1 AND tenant_id=$2`, id, tenant))
	if err != nil {
		return Detail{}, err
	}
	d := Detail{Incident: inc, Timeline: []TimelineEntry{}, Analyses: []Analysis{}}
	rows, err := s.DB.Query(ctx, `SELECT at, kind, summary FROM incident_timeline WHERE incident_id=$1 AND tenant_id=$2 ORDER BY at, id`, id, tenant)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var t TimelineEntry
		if err := rows.Scan(&t.At, &t.Kind, &t.Summary); err != nil {
			rows.Close()
			return d, err
		}
		d.Timeline = append(d.Timeline, t)
	}
	rows.Close()
	rows, err = s.DB.Query(ctx, `SELECT id, root_cause, confidence, evidence, recommendation, analyzer, approval, decided_by, decided_at, created_at
		FROM analyses WHERE incident_id=$1 AND tenant_id=$2 ORDER BY created_at DESC`, id, tenant)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var a Analysis
		if err := rows.Scan(&a.ID, &a.RootCause, &a.Confidence, &a.Evidence, &a.Recommendation, &a.Analyzer, &a.Approval, &a.DecidedBy, &a.DecidedAt, &a.CreatedAt); err != nil {
			return d, err
		}
		d.Analyses = append(d.Analyses, a)
	}
	return d, rows.Err()
}

var validStatus = map[string]bool{"open": true, "investigating": true, "identified": true, "resolved": true}

func (s *Service) SetStatus(ctx context.Context, tenant, id, status, actor string) error {
	if !validStatus[status] {
		return fmt.Errorf("invalid status %q", status)
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE incidents SET status=$3, updated_at=now(),
		resolved_at = CASE WHEN $3='resolved' THEN now() ELSE NULL END WHERE id=$1 AND tenant_id=$2`, id, tenant, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := timeline(ctx, tx, id, tenant, "status", fmt.Sprintf("%s set status to %s", actor, status)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RequestAnalysis queues a (re-)analysis of an incident.
func (s *Service) RequestAnalysis(ctx context.Context, tenant, id, actor string) error {
	inc, err := scanIncident(s.DB.QueryRow(ctx, `SELECT `+incidentCols+` FROM incidents WHERE id=$1 AND tenant_id=$2`, id, tenant))
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := timeline(ctx, tx, id, tenant, "analysis", actor+" requested a new analysis"); err != nil {
		return err
	}
	if err := outbox(ctx, tx, events.TopicAnalysisRequested, "incident-service", tenant, id,
		createdPayload{inc.ID, inc.Title, inc.Service, inc.Severity, inc.CreatedAt}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Decide records a human decision on a recommended remediation (ADR-009).
// Resolve-X never executes remediation itself; the decision is the audit trail.
func (s *Service) Decide(ctx context.Context, tenant, analysisID, decision, actor string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var incidentID string
	err = tx.QueryRow(ctx, `UPDATE analyses SET approval=$3, decided_by=$4, decided_at=now()
		WHERE id=$1 AND tenant_id=$2 AND approval='pending' RETURNING incident_id`, analysisID, tenant, decision, actor).Scan(&incidentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := timeline(ctx, tx, incidentID, tenant, "remediation", fmt.Sprintf("%s %s the recommended remediation", actor, decision)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type completedPayload struct {
	IncidentID     string          `json:"incidentId"`
	RootCause      string          `json:"rootCause"`
	Confidence     int             `json:"confidence"`
	Evidence       json.RawMessage `json:"evidence"`
	Recommendation json.RawMessage `json:"recommendation"`
	Analyzer       string          `json:"analyzer"`
}

// HandleAnalysisCompleted stores an analysis. The event id is the analysis id,
// so a redelivered event is a no-op.
func (s *Service) HandleAnalysisCompleted(ctx context.Context, env events.Envelope) error {
	var p completedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return nil // malformed payloads can never succeed; drop rather than retry
	}
	var rec struct {
		Action string `json:"action"`
	}
	_ = json.Unmarshal(p.Recommendation, &rec)
	approval := "pending"
	if rec.Action == "" {
		approval = "none"
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO analyses (id, incident_id, tenant_id, root_cause, confidence, evidence, recommendation, analyzer, approval)
		SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9 WHERE EXISTS (SELECT 1 FROM incidents WHERE id=$2 AND tenant_id=$3)
		ON CONFLICT (id) DO NOTHING`, env.EventID, p.IncidentID, env.TenantID, p.RootCause, p.Confidence, p.Evidence, p.Recommendation, p.Analyzer, approval)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `UPDATE incidents SET status='identified', updated_at=now() WHERE id=$1 AND tenant_id=$2 AND status IN ('open','investigating')`, p.IncidentID, env.TenantID); err != nil {
		return err
	}
	if err := timeline(ctx, tx, p.IncidentID, env.TenantID, "analysis", fmt.Sprintf("Probable root cause (%d%%): %s", p.Confidence, p.RootCause)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RelayOutbox publishes pending outbox rows to Kafka until ctx ends.
func (s *Service) RelayOutbox(ctx context.Context, pub *events.Producer) {
	for ctx.Err() == nil {
		n, err := s.relayBatch(ctx, pub)
		if err != nil && ctx.Err() == nil {
			s.Log.Error("outbox relay", "err", err)
		}
		if n == 0 || err != nil {
			select {
			case <-ctx.Done():
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}

func (s *Service) relayBatch(ctx context.Context, pub *events.Producer) (int, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id, topic, key, envelope FROM outbox WHERE published_at IS NULL ORDER BY id LIMIT 100 FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return 0, err
	}
	type item struct {
		id         int64
		topic, key string
		env        events.Envelope
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.topic, &it.key, &it.env); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, it)
	}
	rows.Close()
	for _, it := range items {
		if err := pub.Publish(ctx, it.topic, []byte(it.key), it.env); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `UPDATE outbox SET published_at=now() WHERE id=$1`, it.id); err != nil {
			return 0, err
		}
	}
	return len(items), tx.Commit(ctx)
}
