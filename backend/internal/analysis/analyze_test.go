package analysis

import (
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

func base() *Bundle {
	return &Bundle{Service: "payments", Now: t0.Add(10 * time.Minute), AnomalyStart: t0,
		Recent: ServiceStat{Spans: 500, Errors: 100, ErrRate: 0.2, P95Ms: 900}, Base: ServiceStat{Spans: 5000, P95Ms: 30}}
}

func TestHeuristicPrefersRecentDeployment(t *testing.T) {
	b := base()
	b.Deploys = []Deploy{{Service: "payments", Version: "v2", At: t0.Add(-4 * time.Minute)}}
	b.Deps = []DepStat{{Target: "pg", Calls: 500, ErrRate: 0.2, P95Ms: 900, BaseP95Ms: 15}}
	r := Heuristic(b)
	if !strings.Contains(r.RootCause, "v2") || r.Recommendation.Action != "rollback" || r.Confidence < 70 {
		t.Fatalf("unexpected: %+v", r)
	}
}

func TestHeuristicIgnoresStaleDeployment(t *testing.T) {
	b := base()
	b.Deploys = []Deploy{{Service: "payments", Version: "v1", At: t0.Add(-3 * time.Hour)}}
	b.Deps = []DepStat{{Target: "pg", Calls: 500, ErrRate: 0.2, P95Ms: 900, BaseP95Ms: 15}}
	r := Heuristic(b)
	if r.Recommendation.Action != "" || !strings.Contains(r.RootCause, "pg") {
		t.Fatalf("expected dependency cause, got %+v", r)
	}
}

func TestHeuristicInternalErrors(t *testing.T) {
	b := base()
	b.Logs = []LogGroup{{Service: "payments", Body: "nil pointer", Count: 40}}
	r := Heuristic(b)
	if !strings.Contains(r.RootCause, "nil pointer") {
		t.Fatalf("got %+v", r)
	}
}

func TestHeuristicNoEvidenceIsHonest(t *testing.T) {
	b := base()
	b.Recent = ServiceStat{}
	r := Heuristic(b)
	if r.Confidence > 25 || r.Recommendation.Action != "" {
		t.Fatalf("should be low confidence, got %+v", r)
	}
}

func TestDegraded(t *testing.T) {
	if (DepStat{Calls: 2, ErrRate: 1}).Degraded() {
		t.Error("too few calls must not count")
	}
	if !(DepStat{Calls: 50, ErrRate: 0.2}).Degraded() {
		t.Error("error spike must count")
	}
	if (DepStat{Calls: 50, ErrRate: 0.2, BaseErrRate: 0.2}).Degraded() {
		t.Error("steady error rate is not degradation")
	}
}
