package incident

import "testing"

func TestDependsOnAnomalous(t *testing.T) {
	g := map[string][]string{"gateway": {"checkout"}, "checkout": {"payments", "inventory"}, "payments": {"db"}}
	anom := map[string]bool{"gateway": true, "checkout": true, "payments": true}
	if !dependsOnAnomalous(g, "gateway", anom) || !dependsOnAnomalous(g, "checkout", anom) {
		t.Error("callers of a failing service must be suppressed")
	}
	if dependsOnAnomalous(g, "payments", anom) {
		t.Error("the deepest anomalous service must be kept")
	}
	cyc := map[string][]string{"a": {"b"}, "b": {"a"}}
	if dependsOnAnomalous(cyc, "a", map[string]bool{"a": true}) {
		t.Error("cycles must terminate and not self-suppress")
	}
}

func TestMapSeverity(t *testing.T) {
	for in, want := range map[string]string{"critical": "SEV-1", "Warning": "SEV-3", "": "SEV-2", "weird": "SEV-2"} {
		if got := mapSeverity(in); got != want {
			t.Errorf("%q: got %s want %s", in, got, want)
		}
	}
}

func TestRuleValidation(t *testing.T) {
	ok := Rule{Name: "n", Kind: "error_rate", Service: "s", Threshold: 0.05}
	if err := ok.Validate(); err != nil || ok.WindowMinutes != 5 || ok.Op != ">" || ok.Severity != "SEV-2" {
		t.Fatalf("defaults: %+v %v", ok, err)
	}
	for name, r := range map[string]Rule{
		"no name":          {Kind: "error_rate", Service: "s", Threshold: .1},
		"rate > 1":         {Name: "n", Kind: "error_rate", Service: "s", Threshold: 5},
		"no service":       {Name: "n", Kind: "latency_p95", Threshold: 100},
		"bad agg":          {Name: "n", Kind: "metric", Metric: "m", Agg: "drop table"},
		"bad kind":         {Name: "n", Kind: "x"},
		"bad op":           {Name: "n", Kind: "error_rate", Service: "s", Threshold: .1, Op: "="},
		"window too large": {Name: "n", Kind: "error_rate", Service: "s", Threshold: .1, WindowMinutes: 5000},
	} {
		if err := r.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSLOValidation(t *testing.T) {
	ms := 300.0
	good := SLO{Name: "n", Service: "s", Kind: "latency", Objective: 99.5, LatencyMs: &ms}
	if err := good.Validate(); err != nil || good.WindowDays != 7 {
		t.Fatalf("%+v %v", good, err)
	}
	for name, o := range map[string]SLO{
		"objective 100":      {Name: "n", Service: "s", Kind: "availability", Objective: 100},
		"latency w/o target": {Name: "n", Service: "s", Kind: "latency", Objective: 99},
		"window 90":          {Name: "n", Service: "s", Kind: "availability", Objective: 99, WindowDays: 90},
	} {
		if err := o.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
