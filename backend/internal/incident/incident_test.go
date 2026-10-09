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
