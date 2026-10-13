package analysis

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/resolvex/resolve-x/backend/internal/aiconfig"
	"github.com/resolvex/resolve-x/backend/internal/events"
	"github.com/resolvex/resolve-x/backend/internal/llm"
)

type Worker struct {
	G      *Gatherer
	Pub    *events.Producer
	AI     *aiconfig.Store // optional: per-organization LLM settings
	Client *http.Client    // SSRF-safe client for provider calls
	Log    *slog.Logger
	Now    func() time.Time
}

type incidentPayload struct {
	IncidentID string `json:"incidentId"`
	Title      string `json:"title"`
	Service    string `json:"service"`
}

// Handle analyses one incident.created / analysis.requested event and
// publishes analysis.completed. The result's event id derives from the
// triggering event id, so redelivery produces an identical, ignorable event.
func (w *Worker) Handle(ctx context.Context, env events.Envelope) error {
	var in incidentPayload
	if json.Unmarshal(env.Payload, &in) != nil || in.IncidentID == "" {
		return nil // can never succeed; do not retry
	}
	now := time.Now()
	if w.Now != nil {
		now = w.Now()
	}
	var res Result
	var evidence []Evidence
	bundle, err := w.G.Gather(ctx, env.TenantID, in.Service, now)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res = Result{RootCause: "No recent telemetry was found for this tenant, so no analysis was possible.", Confidence: 0, Analyzer: "heuristic",
			Recommendation: Recommendation{Description: "Send traces, logs and deployment events to Resolve-X, then request a new analysis."}}
	case err != nil:
		return err // infrastructure error: retry
	default:
		evidence = bundle.Evidence
		res = Heuristic(bundle)
		if w.AI != nil {
			res = w.refine(ctx, env.TenantID, in.Title, bundle, res)
		}
	}
	if evidence == nil {
		evidence = []Evidence{}
	}
	ev, _ := json.Marshal(evidence)
	rec, _ := json.Marshal(res.Recommendation)
	out, err := events.New(events.TopicAnalysisCompleted, "analysis-service", env.TenantID, map[string]any{
		"incidentId": in.IncidentID, "rootCause": res.RootCause, "confidence": res.Confidence,
		"evidence": json.RawMessage(ev), "recommendation": json.RawMessage(rec), "analyzer": res.Analyzer,
	})
	if err != nil {
		return err
	}
	out.EventID = "ana_" + env.EventID
	return w.Pub.Publish(ctx, events.TopicAnalysisCompleted, events.Key(env.TenantID, in.IncidentID), out)
}

// refine upgrades the heuristic result with the tenant's configured model.
// Any failure leaves the heuristic result in place: analysis never blocks on an LLM.
func (w *Worker) refine(ctx context.Context, tenant, title string, b *Bundle, prelim Result) Result {
	cfg, _, err := w.AI.Resolve(ctx, tenant)
	if err != nil {
		if !errors.Is(err, llm.ErrNotConfigured) {
			w.Log.Warn("ai settings unavailable; using heuristic result", "err", err)
		}
		return prelim
	}
	p, err := llm.New(cfg, w.Client)
	if err != nil {
		w.Log.Warn("ai provider invalid; using heuristic result", "err", err)
		return prelim
	}
	llmCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	refined, err := Refine(llmCtx, p, cfg.Provider+":"+cfg.Model, title, b, prelim)
	if err != nil {
		w.Log.Warn("llm analysis failed; using heuristic result", "err", err)
		return prelim
	}
	return refined
}
