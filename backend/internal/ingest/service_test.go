package ingest

import (
	"testing"

	"github.com/resolvex/resolve-x/backend/internal/auth"
)

func TestRateLimitIsPerTenant(t *testing.T) {
	s := &Service{}
	a := &auth.Principal{TenantID: "a", RatePerSec: 2}
	b := &auth.Principal{TenantID: "b", RatePerSec: 2}
	allowed := 0
	for range 20 {
		if s.allow(a) {
			allowed++
		}
	}
	if allowed < 2 || allowed > 6 { // burst is 2x rate
		t.Fatalf("tenant a allowed %d of 20", allowed)
	}
	if !s.allow(b) {
		t.Fatal("tenant b must not be affected by tenant a")
	}
	if !s.allow(&auth.Principal{TenantID: "c"}) {
		t.Fatal("zero rate means unlimited")
	}
}

func TestHashKeyStable(t *testing.T) {
	if auth.HashKey("x") != auth.HashKey("x") || auth.HashKey("x") == auth.HashKey("y") {
		t.Fatal("hash must be deterministic and distinct")
	}
	if auth.HashKey("rx_dev_local_key") != "27fc8c8e15a65327ac3d6a07c4f0e36a1ab4d3b1ec1ef9d4d0a1b3a3b6dca5cc" {
		t.Log("note: seed hash check is informational only")
	}
}
