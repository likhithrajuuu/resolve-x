package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Recommendation struct {
	Action      string `json:"action"` // "rollback" needs human approval; "" is advice only
	Description string `json:"description"`
	Risk        string `json:"risk,omitempty"`
}

type Result struct {
	RootCause      string         `json:"rootCause"`
	Confidence     int            `json:"confidence"`
	Recommendation Recommendation `json:"recommendation"`
	Analyzer       string         `json:"analyzer"`
}

type candidate struct {
	score float64
	cause string
	rec   Recommendation
}

// Heuristic ranks candidate causes from correlated evidence. It is the
// always-available analyzer and the fallback when no LLM is configured.
func Heuristic(b *Bundle) Result {
	var cands []candidate
	degradedDeps := map[string]DepStat{}
	for _, d := range b.Deps {
		if d.Degraded() {
			degradedDeps[d.Target] = d
		}
	}

	// A deployment that landed shortly before degradation began.
	for _, d := range b.Deploys {
		lead := b.AnomalyStart.Sub(d.At)
		if lead < -2*time.Minute || lead > 45*time.Minute {
			continue
		}
		score := 0.65 + 0.2*(1-max(lead, 0).Minutes()/45)
		where := "the affected service"
		if d.Service != b.Service {
			if _, ok := degradedDeps[d.Service]; !ok {
				continue
			}
			score += 0.1
			where = "a degraded dependency"
		} else {
			score += 0.05
		}
		cands = append(cands, candidate{score,
			fmt.Sprintf("Deployment of %s %s (%s) at %s preceded the degradation", d.Service, d.Version, where, d.At.UTC().Format("15:04:05")),
			Recommendation{Action: "rollback", Description: fmt.Sprintf("Roll back %s to the version before %s.", d.Service, d.Version),
				Risk: "Rolling back reverts every change in that release."}})
	}

	// A degraded downstream dependency.
	for _, d := range degradedDeps {
		score := 0.5 + min(d.ErrRate, 0.2)
		if d.BaseP95Ms > 0 {
			score += min(d.P95Ms/d.BaseP95Ms/40, 0.15)
		}
		kind := "dependency"
		if d.External {
			kind = "external dependency"
		}
		cands = append(cands, candidate{score,
			fmt.Sprintf("%s's %s %s is degraded (error rate %.1f%%, p95 %.0f ms vs %.0f ms before)", b.Service, kind, d.Target, d.ErrRate*100, d.P95Ms, d.BaseP95Ms),
			Recommendation{Description: fmt.Sprintf("Investigate %s: check its health, capacity and recent changes.", d.Target)}})
	}

	// Errors inside the service itself.
	if b.Recent.ErrRate >= 0.05 && len(b.Logs) > 0 && len(degradedDeps) == 0 {
		l := b.Logs[0]
		cands = append(cands, candidate{0.45 + min(b.Recent.ErrRate, 0.3),
			fmt.Sprintf("%s is failing internally; most common error (%d occurrences): %s", l.Service, l.Count, l.Body),
			Recommendation{Description: "Inspect the failing code path and the error logs listed in the evidence."}})
	}

	if len(cands) == 0 {
		return Result{RootCause: fmt.Sprintf("No clear cause found for %s from the available telemetry.", b.Service),
			Confidence: 20, Analyzer: "heuristic",
			Recommendation: Recommendation{Description: "Check that the service and its dependencies are sending traces, logs and deployment events."}}
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if c.score > best.score {
			best = c
		}
	}
	return Result{RootCause: best.cause, Confidence: int(max(20, min(best.score*100, 95))), Recommendation: best.rec, Analyzer: "heuristic"}
}

// LLM refines the heuristic result using Claude. Evidence leaves Resolve-X for
// the Anthropic API only when ANTHROPIC_API_KEY is configured.
type LLM struct {
	APIKey string
	Model  string
	Client *http.Client
}

const systemPrompt = `You are a senior site-reliability engineer doing root-cause analysis for a production incident.
You are given correlated telemetry evidence and a preliminary heuristic conclusion. Decide the most probable root cause using ONLY the evidence.
Never invent facts. If evidence is insufficient, say so and lower the confidence.
Respond with a single JSON object and nothing else:
{"rootCause": string (1-2 sentences), "confidence": integer 0-100, "recommendation": {"action": "rollback" | "", "description": string, "risk": string}}
Use action "rollback" only if a specific deployment is the likely cause; otherwise use "".`

func (l *LLM) Analyze(ctx context.Context, title string, b *Bundle, prelim Result) (Result, error) {
	user := fmt.Sprintf("Incident: %s\nAffected service: %s\nDegradation began about: %s\n\nEvidence:\n%s\nHeuristic conclusion: %s (confidence %d)",
		title, b.Service, b.AnomalyStart.UTC().Format(time.RFC3339), b.Describe(), prelim.RootCause, prelim.Confidence)
	body, _ := json.Marshal(map[string]any{
		"model": l.Model, "max_tokens": 1024, "system": systemPrompt,
		"messages": []map[string]string{{"role": "user", "content": user}},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	req.Header.Set("x-api-key", l.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")
	resp, err := l.Client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return Result{}, fmt.Errorf("anthropic api: status %d", resp.StatusCode)
	}
	var out struct {
		Content []struct{ Type, Text string } `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Content) == 0 {
		return Result{}, fmt.Errorf("anthropic api: unexpected response")
	}
	text := out.Content[0].Text
	if i, j := strings.Index(text, "{"), strings.LastIndex(text, "}"); i >= 0 && j > i {
		text = text[i : j+1]
	}
	var parsed struct {
		RootCause      string         `json:"rootCause"`
		Confidence     int            `json:"confidence"`
		Recommendation Recommendation `json:"recommendation"`
	}
	if err := json.Unmarshal([]byte(text), &parsed); err != nil || parsed.RootCause == "" {
		return Result{}, fmt.Errorf("model returned unparseable output")
	}
	if parsed.Recommendation.Action != "rollback" {
		parsed.Recommendation.Action = ""
	}
	return Result{RootCause: parsed.RootCause, Confidence: max(0, min(parsed.Confidence, 100)), Recommendation: parsed.Recommendation, Analyzer: l.Model}, nil
}
