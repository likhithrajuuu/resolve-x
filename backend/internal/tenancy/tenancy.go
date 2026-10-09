// Package tenancy implements tenant, project, environment and API-key
// management. It sits behind Kong, which owns routing, rate limiting and TLS.
package tenancy

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"

	"github.com/resolvex/resolve-x/backend/internal/auth"
	"github.com/resolvex/resolve-x/backend/internal/authn"
)

type Service struct {
	DB     *pgxpool.Pool
	Redis  *redis.Client
	Secret []byte
	Log    *slog.Logger
}

func (g *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	g.mountAuth(mux)
	g.mountDashboards(mux)
	mux.HandleFunc("GET /api/v1/projects", g.secure(g.listProjects))
	mux.HandleFunc("POST /api/v1/projects", g.secure(g.createProject))
	mux.HandleFunc("GET /api/v1/projects/{projectId}/environments", g.secure(g.listEnvironments))
	mux.HandleFunc("POST /api/v1/projects/{projectId}/environments", g.secure(g.createEnvironment))
	mux.HandleFunc("GET /api/v1/projects/{projectId}/api-keys", g.secure(g.listAPIKeys))
	mux.HandleFunc("POST /api/v1/projects/{projectId}/api-keys", g.secure(g.createAPIKey))
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/api-keys/{keyId}", g.secure(g.revokeAPIKey))

	return requestID(mux)
}

func (g *Service) secure(h func(http.ResponseWriter, *http.Request, Identity)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := authn.Verify(g.Secret, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid credentials")
			return
		}
		h(w, r, Identity{TenantID: c.Tenant, Actor: c.Email})
	}
}

// Identity is the authenticated caller. TenantID always comes from the verified
// token, never from a request parameter (api-gateway.md, Security).
type Identity struct{ TenantID, Actor string }

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			rid = ulid.Make().String()
			r.Header.Set("X-Request-Id", rid)
		}
		w.Header().Set("X-Request-Id", rid)
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": code, "message": msg})
}

func decode(r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(nil, r.Body, 64<<10)
	return json.NewDecoder(r.Body).Decode(v) == nil
}

func newID(prefix string) string { return prefix + "_" + strings.ToLower(ulid.Make().String()) }

func (g *Service) audit(ctx context.Context, id Identity, action, target string) {
	_, err := g.DB.Exec(ctx, `INSERT INTO audit_log (tenant_id, actor, action, target) VALUES ($1,$2,$3,$4)`, id.TenantID, id.Actor, action, target)
	if err != nil {
		g.Log.Error("audit write failed", "action", action, "err", err)
	}
}

type project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func (g *Service) listProjects(w http.ResponseWriter, r *http.Request, id Identity) {
	rows, err := g.DB.Query(r.Context(), `SELECT id, name, created_at FROM projects WHERE tenant_id=$1 ORDER BY created_at`, id.TenantID)
	if err != nil {
		writeErr(w, 500, "internal", "query failed")
		return
	}
	defer rows.Close()
	out := []project{}
	for rows.Next() {
		var p project
		if rows.Scan(&p.ID, &p.Name, &p.CreatedAt) == nil {
			out = append(out, p)
		}
	}
	writeJSON(w, 200, out)
}

func (g *Service) createProject(w http.ResponseWriter, r *http.Request, id Identity) {
	var in struct{ Name string }
	if !decode(r, &in) || strings.TrimSpace(in.Name) == "" {
		writeErr(w, 400, "invalid_request", "name is required")
		return
	}
	p := project{ID: newID("prj"), Name: strings.TrimSpace(in.Name)}
	err := g.DB.QueryRow(r.Context(), `INSERT INTO projects (id, tenant_id, name) VALUES ($1,$2,$3) RETURNING created_at`, p.ID, id.TenantID, p.Name).Scan(&p.CreatedAt)
	if err != nil {
		writeErr(w, 409, "conflict", "project name already exists")
		return
	}
	g.audit(r.Context(), id, "project.create", p.ID)
	writeJSON(w, 201, p)
}

// ownsProject guards every project-scoped route against cross-tenant access.
func (g *Service) ownsProject(ctx context.Context, id Identity, projectID string) bool {
	var ok bool
	err := g.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM projects WHERE id=$1 AND tenant_id=$2)`, projectID, id.TenantID).Scan(&ok)
	return err == nil && ok
}

func (g *Service) listEnvironments(w http.ResponseWriter, r *http.Request, id Identity) {
	pid := r.PathValue("projectId")
	if !g.ownsProject(r.Context(), id, pid) {
		writeErr(w, 404, "not_found", "project not found")
		return
	}
	rows, err := g.DB.Query(r.Context(), `SELECT id, name, created_at FROM environments WHERE project_id=$1 AND tenant_id=$2 ORDER BY created_at`, pid, id.TenantID)
	if err != nil {
		writeErr(w, 500, "internal", "query failed")
		return
	}
	defer rows.Close()
	out := []project{}
	for rows.Next() {
		var p project
		if rows.Scan(&p.ID, &p.Name, &p.CreatedAt) == nil {
			out = append(out, p)
		}
	}
	writeJSON(w, 200, out)
}

func (g *Service) createEnvironment(w http.ResponseWriter, r *http.Request, id Identity) {
	pid := r.PathValue("projectId")
	var in struct{ Name string }
	if !decode(r, &in) || strings.TrimSpace(in.Name) == "" {
		writeErr(w, 400, "invalid_request", "name is required")
		return
	}
	if !g.ownsProject(r.Context(), id, pid) {
		writeErr(w, 404, "not_found", "project not found")
		return
	}
	e := project{ID: newID("env"), Name: strings.TrimSpace(in.Name)}
	err := g.DB.QueryRow(r.Context(), `INSERT INTO environments (id, tenant_id, project_id, name) VALUES ($1,$2,$3,$4) RETURNING created_at`, e.ID, id.TenantID, pid, e.Name).Scan(&e.CreatedAt)
	if err != nil {
		writeErr(w, 409, "conflict", "environment name already exists")
		return
	}
	g.audit(r.Context(), id, "environment.create", e.ID)
	writeJSON(w, 201, e)
}

func (g *Service) createAPIKey(w http.ResponseWriter, r *http.Request, id Identity) {
	pid := r.PathValue("projectId")
	var in struct {
		EnvironmentID string `json:"environmentId"`
	}
	if !decode(r, &in) || in.EnvironmentID == "" {
		writeErr(w, 400, "invalid_request", "environmentId is required")
		return
	}
	var ok bool
	_ = g.DB.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM environments WHERE id=$1 AND project_id=$2 AND tenant_id=$3)`, in.EnvironmentID, pid, id.TenantID).Scan(&ok)
	if !ok {
		writeErr(w, 404, "not_found", "project or environment not found")
		return
	}
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	raw := "rx_" + base64.RawURLEncoding.EncodeToString(buf)
	keyID := newID("key")
	_, err := g.DB.Exec(r.Context(), `INSERT INTO api_keys (id, tenant_id, project_id, environment_id, key_hash, key_prefix) VALUES ($1,$2,$3,$4,$5,$6)`,
		keyID, id.TenantID, pid, in.EnvironmentID, auth.HashKey(raw), raw[:8])
	if err != nil {
		writeErr(w, 500, "internal", "could not create key")
		return
	}
	g.audit(r.Context(), id, "apikey.create", keyID)
	// The only time the plaintext key is ever returned.
	writeJSON(w, 201, map[string]string{"id": keyID, "key": raw, "prefix": raw[:8]})
}

func (g *Service) revokeAPIKey(w http.ResponseWriter, r *http.Request, id Identity) {
	var hash string
	err := g.DB.QueryRow(r.Context(), `
		UPDATE api_keys SET status='revoked', revoked_at=now()
		WHERE id=$1 AND project_id=$2 AND tenant_id=$3 AND status='active' RETURNING key_hash`,
		r.PathValue("keyId"), r.PathValue("projectId"), id.TenantID).Scan(&hash)
	if err == pgx.ErrNoRows {
		writeErr(w, 404, "not_found", "key not found")
		return
	}
	if err != nil {
		writeErr(w, 500, "internal", "could not revoke key")
		return
	}
	// Drop the shared cache entry so revocation takes effect within the
	// ingestion instances' short in-process TTL.
	if g.Redis != nil {
		_ = g.Redis.Del(r.Context(), auth.CacheKey(hash)).Err()
	}
	g.audit(r.Context(), id, "apikey.revoke", r.PathValue("keyId"))
	w.WriteHeader(http.StatusNoContent)
}

func (g *Service) listAPIKeys(w http.ResponseWriter, r *http.Request, id Identity) {
	rows, err := g.DB.Query(r.Context(), `
		SELECT id, environment_id, key_prefix, status, created_at FROM api_keys
		WHERE project_id=$1 AND tenant_id=$2 ORDER BY created_at DESC`, r.PathValue("projectId"), id.TenantID)
	if err != nil {
		writeErr(w, 500, "internal", "query failed")
		return
	}
	defer rows.Close()
	type key struct {
		ID            string    `json:"id"`
		EnvironmentID string    `json:"environmentId"`
		Prefix        string    `json:"prefix"`
		Status        string    `json:"status"`
		CreatedAt     time.Time `json:"createdAt"`
	}
	out := []key{}
	for rows.Next() {
		var k key
		if rows.Scan(&k.ID, &k.EnvironmentID, &k.Prefix, &k.Status, &k.CreatedAt) == nil {
			out = append(out, k)
		}
	}
	writeJSON(w, 200, out)
}
