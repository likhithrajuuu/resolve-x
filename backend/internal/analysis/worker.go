package analysis

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/resolvex/resolve-x/backend/internal/events"
)

type Worker struct {
	G   *Gatherer
	Pub *events.Producer
	LLM *LLM // optional
	Log *slog.Logger
	Now func() time.Time
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
		if w.LLM != nil {
			llmCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			refined, err := w.LLM.Analyze(llmCtx, in.Title, bundle, res)
			cancel()
			if err != nil {
				w.Log.Warn("llm analysis failed; using heuristic result", "err", err)
			} else {
				res = refined
			}
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
