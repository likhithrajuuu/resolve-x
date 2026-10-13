package incident

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/resolvex/resolve-x/backend/internal/llm"
	"github.com/resolvex/resolve-x/backend/internal/secrets"
)

func notifier(t *testing.T) (*Notifier, *secrets.Box) {
	box, err := secrets.New("test-secret-key-0123456789")
	if err != nil {
		t.Fatal(err)
	}
	return &Notifier{Box: box, Client: http.DefaultClient, PublicURL: "https://app.example.com"}, box
}

var ev = Event{Type: "analysis.completed", Incident: Incident{ID: "inc_1", Title: "High error rate on payments", Severity: "SEV-2", Service: "payments"},
	RootCause: "Deployment v2 preceded the degradation", Confidence: 85, Recommended: "Roll back payments"}

func TestSlackPayload(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
	}))
	defer srv.Close()
	n, box := notifier(t)
	u, _ := box.Seal(srv.URL)
	if err := n.deliver(context.Background(), "slack", u, "", ev); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[SEV-2] High error rate on payments", "85%", "Deployment v2", "Roll back payments", "https://app.example.com/dashboard/incidents/inc_1"} {
		if !strings.Contains(got["text"], want) {
			t.Errorf("slack text missing %q: %s", want, got["text"])
		}
	}
}

func TestWebhookIsSigned(t *testing.T) {
	var body []byte
	var sig, event string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		sig, event = r.Header.Get("X-Resolvex-Signature"), r.Header.Get("X-Resolvex-Event")
	}))
	defer srv.Close()
	n, box := notifier(t)
	u, _ := box.Seal(srv.URL)
	sec, _ := box.Seal("whsec_abc")
	if err := n.deliver(context.Background(), "webhook", u, sec, ev); err != nil {
		t.Fatal(err)
	}
	m := hmac.New(sha256.New, []byte("whsec_abc"))
	m.Write(body)
	if sig != "sha256="+hex.EncodeToString(m.Sum(nil)) || event != "analysis.completed" {
		t.Fatalf("bad signature or event header: %q %q", sig, event)
	}
	var parsed map[string]any
	if json.Unmarshal(body, &parsed) != nil || parsed["event"] != "analysis.completed" {
		t.Fatalf("payload: %s", body)
	}
}

func TestRetriesServerErrorsButNotClientErrors(t *testing.T) {
	var calls atomic.Int32
	flaky := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 2 {
			w.WriteHeader(500)
		}
	}))
	defer flaky.Close()
	n, box := notifier(t)
	u, _ := box.Seal(flaky.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := n.deliver(ctx, "slack", u, "", ev); err != nil || calls.Load() != 2 {
		t.Fatalf("expected success after one retry: err=%v calls=%d", err, calls.Load())
	}
	var bad atomic.Int32
	reject := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { bad.Add(1); w.WriteHeader(404) }))
	defer reject.Close()
	u2, _ := box.Seal(reject.URL)
	if err := n.deliver(ctx, "slack", u2, "", ev); err == nil || bad.Load() != 1 {
		t.Fatalf("404 must fail without retrying: err=%v calls=%d", err, bad.Load())
	}
}

func TestChannelsCannotTargetInternalAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { t.Error("internal server was reached") }))
	defer srv.Close()
	n, box := notifier(t)
	n.Client = llm.SafeClient(false, 5*time.Second)
	u, _ := box.Seal(srv.URL) // 127.0.0.1
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := n.deliver(ctx, "webhook", u, "", ev); err == nil {
		t.Fatal("loopback target must be refused")
	}
}

func TestChannelValidationHappensBeforeStorage(t *testing.T) {
	n, _ := notifier(t) // nil DB: every case below must be rejected before touching it
	for name, c := range map[string][3]string{
		"http slack":      {"slack", "x", "http://hooks.slack.com/services/T/B/x"},
		"non-slack host":  {"slack", "x", "https://evil.example.com/hook"},
		"no name":         {"webhook", " ", "https://example.com/h"},
		"unknown kind":    {"email", "x", "https://example.com/h"},
		"empty url":       {"webhook", "x", ""},
		"credentials url": {"webhook", "x", "https://user:pw@example.com/h"},
	} {
		if _, _, err := n.Create(context.Background(), "t", c[0], c[1], c[2]); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
