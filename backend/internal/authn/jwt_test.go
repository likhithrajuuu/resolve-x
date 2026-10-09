package authn

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var secret = []byte("0123456789abcdef0123456789abcdef")

func TestRoundTrip(t *testing.T) {
	tok, err := Issue(secret, Claims{Sub: "u1", Email: "a@b.co", Tenant: "t1", Role: "owner"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Verify(secret, tok)
	if err != nil || c.Tenant != "t1" || c.Email != "a@b.co" {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestRejects(t *testing.T) {
	good, _ := Issue(secret, Claims{Tenant: "t1"}, time.Hour)
	expired, _ := Issue(secret, Claims{Tenant: "t1"}, -time.Minute)
	noTenant, _ := Issue(secret, Claims{}, time.Hour)
	parts := strings.Split(good, ".")
	tampered := parts[0] + "." + parts[1] + "x." + parts[2]
	none := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." + parts[1] + "."
	other, _ := Issue([]byte("ffffffffffffffffffffffffffffffff"), Claims{Tenant: "t1"}, time.Hour)
	for name, tok := range map[string]string{"expired": expired, "no tenant": noTenant, "tampered": tampered, "alg none": none, "wrong key": other, "garbage": "abc", "empty": ""} {
		if _, err := Verify(secret, tok); err == nil {
			t.Errorf("%s: token accepted", name)
		}
	}
}

func TestRequireInjectsClaims(t *testing.T) {
	tok, _ := Issue(secret, Claims{Tenant: "t9"}, time.Hour)
	h := Require(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(FromContext(r.Context()).Tenant))
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != "t9" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: got %d", rec.Code)
	}
}
