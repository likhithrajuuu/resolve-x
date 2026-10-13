package secrets

import "testing"

func TestRoundTripAndTamper(t *testing.T) {
	b, err := New("a-sufficiently-long-secret")
	if err != nil {
		t.Fatal(err)
	}
	s1, _ := b.Seal("sk-live-abc")
	s2, _ := b.Seal("sk-live-abc")
	if s1 == s2 {
		t.Error("nonce must be random")
	}
	if got, err := b.Open(s1); err != nil || got != "sk-live-abc" {
		t.Fatalf("%q %v", got, err)
	}
	other, _ := New("a-different-long-secret!")
	if _, err := other.Open(s1); err == nil {
		t.Error("wrong key must fail")
	}
	if _, err := b.Open(s1[:len(s1)-4] + "AAAA"); err == nil {
		t.Error("tampered ciphertext must fail")
	}
	if _, err := New("short"); err == nil {
		t.Error("short secret accepted")
	}
}
