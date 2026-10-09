package tenancy

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/resolvex/resolve-x/backend/internal/authn"
)

const tokenTTL = 12 * time.Hour

func (g *Service) mountAuth(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/signup", g.signup)
	mux.HandleFunc("POST /api/v1/auth/login", g.login)
	mux.HandleFunc("GET /api/v1/auth/me", g.secure(func(w http.ResponseWriter, _ *http.Request, id Identity) {
		writeJSON(w, 200, map[string]string{"email": id.Actor, "tenantId": id.TenantID})
	}))
}

type authResponse struct {
	Token    string `json:"token"`
	Email    string `json:"email"`
	TenantID string `json:"tenantId"`
}

func (g *Service) session(userID, email, tenant, role string) (authResponse, error) {
	tok, err := authn.Issue(g.Secret, authn.Claims{Sub: userID, Email: email, Tenant: tenant, Role: role}, tokenTTL)
	return authResponse{Token: tok, Email: email, TenantID: tenant}, err
}

func (g *Service) signup(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password, Company string }
	if !decode(r, &in) {
		writeErr(w, 400, "invalid_request", "invalid JSON")
		return
	}
	addr, err := mail.ParseAddress(strings.TrimSpace(in.Email))
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if err != nil || addr.Address != strings.TrimSpace(in.Email) || len(in.Password) < 10 || len(in.Password) > 72 {
		writeErr(w, 400, "invalid_request", "a valid email and a password of 10-72 characters are required")
		return
	}
	company := strings.TrimSpace(in.Company)
	if company == "" {
		company = email
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, 500, "internal", "could not create account")
		return
	}
	userID, tenantID := newID("usr"), newID("tnt")
	tx, err := g.DB.Begin(r.Context())
	if err != nil {
		writeErr(w, 500, "internal", "could not create account")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err := tx.Exec(r.Context(), `INSERT INTO users (id, email, password_hash) VALUES ($1,$2,$3)`, userID, email, string(hash)); err != nil {
		// Deliberately the same message a login would give: do not confirm which emails exist.
		writeErr(w, 409, "conflict", "could not create account with these details")
		return
	}
	projectID, envID := newID("prj"), newID("env")
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO tenants (id, name) VALUES ($1,$2)`, []any{tenantID, company}},
		{`INSERT INTO memberships (user_id, tenant_id, role) VALUES ($1,$2,'owner')`, []any{userID, tenantID}},
		{`INSERT INTO projects (id, tenant_id, name) VALUES ($1,$2,'default')`, []any{projectID, tenantID}},
		{`INSERT INTO environments (id, tenant_id, project_id, name) VALUES ($1,$2,$3,'production')`, []any{envID, tenantID, projectID}},
	} {
		if _, err := tx.Exec(r.Context(), q.sql, q.args...); err != nil {
			writeErr(w, 500, "internal", "could not create account")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeErr(w, 500, "internal", "could not create account")
		return
	}
	resp, err := g.session(userID, email, tenantID, "owner")
	if err != nil {
		writeErr(w, 500, "internal", "could not create session")
		return
	}
	writeJSON(w, 201, resp)
}

// dummyHash makes unknown-user logins cost the same as wrong-password ones.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("resolve-x-timing-pad"), bcrypt.DefaultCost)

func (g *Service) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password string }
	if !decode(r, &in) {
		writeErr(w, 400, "invalid_request", "invalid JSON")
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	var userID, hash, tenant, role string
	err := g.DB.QueryRow(r.Context(), `
		SELECT u.id, u.password_hash, m.tenant_id, m.role
		FROM users u JOIN memberships m ON m.user_id = u.id
		WHERE u.email = $1 ORDER BY m.role LIMIT 1`, email).Scan(&userID, &hash, &tenant, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(in.Password))
		writeErr(w, 401, "unauthenticated", "invalid email or password")
		return
	}
	if err != nil {
		writeErr(w, 500, "internal", "login failed")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		writeErr(w, 401, "unauthenticated", "invalid email or password")
		return
	}
	resp, err := g.session(userID, email, tenant, role)
	if err != nil {
		writeErr(w, 500, "internal", "could not create session")
		return
	}
	writeJSON(w, 200, resp)
}

// SeedDevUser ensures a local development login exists on the dev tenant.
func (g *Service) SeedDevUser(ctx context.Context, email, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := g.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO users (id, email, password_hash) VALUES ('usr_dev', $1, $2)
		ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email, password_hash = EXCLUDED.password_hash`, email, string(hash)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO memberships (user_id, tenant_id, role) VALUES ('usr_dev', 'tnt_dev', 'owner') ON CONFLICT DO NOTHING`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
