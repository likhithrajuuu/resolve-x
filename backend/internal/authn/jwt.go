// Package authn issues and verifies the session tokens used by every
// user-facing Resolve-X service. Tokens are HS256 JWTs signed with a shared
// secret; services derive the tenant ONLY from a verified token.
package authn

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type Claims struct {
	Sub    string `json:"sub"` // user id
	Email  string `json:"email"`
	Tenant string `json:"tid"`
	Role   string `json:"role"`
	Exp    int64  `json:"exp"`
}

var ErrInvalidToken = errors.New("invalid or expired token")

var enc = base64.RawURLEncoding

const header = `{"alg":"HS256","typ":"JWT"}`

func sign(secret []byte, signingInput string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(signingInput))
	return enc.EncodeToString(m.Sum(nil))
}

func Issue(secret []byte, c Claims, ttl time.Duration) (string, error) {
	c.Exp = time.Now().Add(ttl).Unix()
	body, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	in := enc.EncodeToString([]byte(header)) + "." + enc.EncodeToString(body)
	return in + "." + sign(secret, in), nil
}

func Verify(secret []byte, token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	// Only HS256 is accepted; the header is checked so "alg":"none" is rejected.
	h, err := enc.DecodeString(parts[0])
	if err != nil || string(h) != header {
		return nil, ErrInvalidToken
	}
	want := sign(secret, parts[0]+"."+parts[1])
	if !hmac.Equal([]byte(want), []byte(parts[2])) {
		return nil, ErrInvalidToken
	}
	raw, err := enc.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var c Claims
	if json.Unmarshal(raw, &c) != nil || c.Tenant == "" || time.Now().Unix() >= c.Exp {
		return nil, ErrInvalidToken
	}
	return &c, nil
}

type ctxKey struct{}

func FromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(ctxKey{}).(*Claims)
	return c
}

// Require wraps a handler so it only runs for a valid bearer token.
func Require(secret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		c, err := Verify(secret, tok)
		if err != nil {
			WriteError(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid credentials")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, c)))
	})
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, code, msg string) {
	WriteJSON(w, status, map[string]string{"error": code, "message": msg})
}
