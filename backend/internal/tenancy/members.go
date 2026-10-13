package tenancy

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/resolvex/resolve-x/backend/internal/auth"
)

const linkTTL = 7 * 24 * time.Hour

func (g *Service) mountMembers(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/members", g.secure(g.listMembers))
	mux.HandleFunc("POST /api/v1/members/invites", g.admin(g.createInvite))
	mux.HandleFunc("DELETE /api/v1/members/invites/{id}", g.admin(g.deleteInvite))
	mux.HandleFunc("POST /api/v1/members/{userId}/reset-link", g.admin(g.resetLink))
	mux.HandleFunc("PATCH /api/v1/members/{userId}", g.admin(g.changeRole))
	mux.HandleFunc("DELETE /api/v1/members/{userId}", g.admin(g.removeMember))
	mux.HandleFunc("POST /api/v1/auth/accept-invite", g.acceptInvite)
	mux.HandleFunc("POST /api/v1/auth/reset-password", g.resetPassword)
}

func newToken(prefix string) (plain, hash string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	plain = prefix + base64.RawURLEncoding.EncodeToString(b)
	return plain, auth.HashKey(plain)
}

func validPassword(p string) bool { return len(p) >= 10 && len(p) <= 72 }

type memberRow struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Self   bool   `json:"self"`
}
type pendingRow struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (g *Service) listMembers(w http.ResponseWriter, r *http.Request, id Identity) {
	rows, err := g.DB.Query(r.Context(), `SELECT u.id, u.email, m.role FROM memberships m JOIN users u ON u.id = m.user_id
		WHERE m.tenant_id=$1 ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, u.email`, id.TenantID)
	if err != nil {
		writeErr(w, 500, "internal", "query failed")
		return
	}
	members := []memberRow{}
	for rows.Next() {
		var m memberRow
		if rows.Scan(&m.UserID, &m.Email, &m.Role) == nil {
			m.Self = m.UserID == id.UserID
			members = append(members, m)
		}
	}
	rows.Close()
	pending := []pendingRow{}
	if id.Role == "owner" || id.Role == "admin" {
		prow, err := g.DB.Query(r.Context(), `SELECT id, kind, email, role, expires_at FROM auth_tokens
			WHERE tenant_id=$1 AND used_at IS NULL AND expires_at > now() ORDER BY created_at DESC`, id.TenantID)
		if err == nil {
			for prow.Next() {
				var p pendingRow
				if prow.Scan(&p.ID, &p.Kind, &p.Email, &p.Role, &p.ExpiresAt) == nil {
					pending = append(pending, p)
				}
			}
			prow.Close()
		}
	}
	writeJSON(w, 200, map[string]any{"members": members, "pending": pending, "canManage": id.Role == "owner" || id.Role == "admin"})
}

func (g *Service) createInvite(w http.ResponseWriter, r *http.Request, id Identity) {
	var in struct{ Email, Role string }
	if !decode(r, &in) {
		writeErr(w, 400, "invalid_request", "invalid JSON")
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if a, err := mail.ParseAddress(email); err != nil || a.Address != email {
		writeErr(w, 400, "invalid_request", "a valid email is required")
		return
	}
	if in.Role != "admin" && in.Role != "member" {
		writeErr(w, 400, "invalid_request", "role must be admin or member")
		return
	}
	if in.Role == "admin" && id.Role != "owner" {
		writeErr(w, 403, "forbidden", "only the owner can invite admins")
		return
	}
	var exists bool
	_ = g.DB.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM users WHERE email=$1)`, email).Scan(&exists)
	if exists {
		writeErr(w, 409, "conflict", "that email already has an account")
		return
	}
	plain, hash := newToken("rxi_")
	tid := newID("tok")
	if _, err := g.DB.Exec(r.Context(), `INSERT INTO auth_tokens (id, kind, tenant_id, email, role, token_hash, expires_at, created_by) VALUES ($1,'invite',$2,$3,$4,$5,$6,$7)`,
		tid, id.TenantID, email, in.Role, hash, time.Now().Add(linkTTL), id.Actor); err != nil {
		writeErr(w, 500, "internal", "could not create invite")
		return
	}
	g.audit(r.Context(), id, "member.invite", email)
	// The token is shown once; the UI turns it into a link on its own origin.
	writeJSON(w, 201, map[string]any{"id": tid, "token": plain, "email": email, "role": in.Role, "expiresAt": time.Now().Add(linkTTL)})
}

func (g *Service) deleteInvite(w http.ResponseWriter, r *http.Request, id Identity) {
	tag, err := g.DB.Exec(r.Context(), `DELETE FROM auth_tokens WHERE id=$1 AND tenant_id=$2 AND used_at IS NULL`, r.PathValue("id"), id.TenantID)
	if err != nil || tag.RowsAffected() == 0 {
		writeErr(w, 404, "not_found", "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// target loads a member of the caller's tenant and applies the permission rules:
// nobody can act on the owner or themselves; admins can only act on members.
func (g *Service) target(w http.ResponseWriter, r *http.Request, id Identity) (userID, email, role string, ok bool) {
	userID = r.PathValue("userId")
	err := g.DB.QueryRow(r.Context(), `SELECT u.email, m.role FROM memberships m JOIN users u ON u.id=m.user_id WHERE m.user_id=$1 AND m.tenant_id=$2`, userID, id.TenantID).Scan(&email, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, 404, "not_found", "member not found")
		return "", "", "", false
	}
	if err != nil {
		writeErr(w, 500, "internal", "query failed")
		return "", "", "", false
	}
	if role == "owner" || (role == "admin" && id.Role != "owner") {
		writeErr(w, 403, "forbidden", "you cannot manage this member")
		return "", "", "", false
	}
	return userID, email, role, true
}

func (g *Service) resetLink(w http.ResponseWriter, r *http.Request, id Identity) {
	userID, email, role, ok := g.target(w, r, id)
	if !ok {
		return
	}
	plain, hash := newToken("rxr_")
	if _, err := g.DB.Exec(r.Context(), `INSERT INTO auth_tokens (id, kind, tenant_id, email, role, user_id, token_hash, expires_at, created_by) VALUES ($1,'reset',$2,$3,$4,$5,$6,$7,$8)`,
		newID("tok"), id.TenantID, email, role, userID, hash, time.Now().Add(24*time.Hour), id.Actor); err != nil {
		writeErr(w, 500, "internal", "could not create reset link")
		return
	}
	g.audit(r.Context(), id, "member.reset_link", email)
	writeJSON(w, 201, map[string]any{"token": plain, "email": email, "expiresAt": time.Now().Add(24 * time.Hour)})
}

func (g *Service) changeRole(w http.ResponseWriter, r *http.Request, id Identity) {
	if id.Role != "owner" {
		writeErr(w, 403, "forbidden", "only the owner can change roles")
		return
	}
	var in struct{ Role string }
	if !decode(r, &in) || (in.Role != "admin" && in.Role != "member") {
		writeErr(w, 400, "invalid_request", "role must be admin or member")
		return
	}
	userID, email, _, ok := g.target(w, r, id)
	if !ok {
		return
	}
	if _, err := g.DB.Exec(r.Context(), `UPDATE memberships SET role=$3 WHERE user_id=$1 AND tenant_id=$2`, userID, id.TenantID, in.Role); err != nil {
		writeErr(w, 500, "internal", "could not change role")
		return
	}
	g.revoke(r.Context(), userID) // the old token still carries the old role
	g.audit(r.Context(), id, "member.role."+in.Role, email)
	w.WriteHeader(http.StatusNoContent)
}

func (g *Service) removeMember(w http.ResponseWriter, r *http.Request, id Identity) {
	userID, email, _, ok := g.target(w, r, id)
	if !ok {
		return
	}
	if userID == id.UserID {
		writeErr(w, 403, "forbidden", "you cannot remove yourself")
		return
	}
	if _, err := g.DB.Exec(r.Context(), `DELETE FROM memberships WHERE user_id=$1 AND tenant_id=$2`, userID, id.TenantID); err != nil {
		writeErr(w, 500, "internal", "could not remove member")
		return
	}
	g.revoke(r.Context(), userID)
	g.audit(r.Context(), id, "member.remove", email)
	w.WriteHeader(http.StatusNoContent)
}

func (g *Service) revoke(ctx context.Context, userID string) {
	if g.Revoker != nil {
		if err := g.Revoker.RevokeUser(ctx, userID); err != nil {
			g.Log.Error("session revocation failed", "user", userID, "err", err)
		}
	}
}

// consume atomically marks a valid, unused token of the given kind as used.
func (g *Service) consume(ctx context.Context, tx pgx.Tx, kind, plain string) (tenant, email, role, userID string, err error) {
	var uid *string
	err = tx.QueryRow(ctx, `UPDATE auth_tokens SET used_at=now() WHERE token_hash=$1 AND kind=$2 AND used_at IS NULL AND expires_at > now()
		RETURNING tenant_id, email, role, user_id`, auth.HashKey(plain), kind).Scan(&tenant, &email, &role, &uid)
	if uid != nil {
		userID = *uid
	}
	return
}

func (g *Service) acceptInvite(w http.ResponseWriter, r *http.Request) {
	var in struct{ Token, Password string }
	if !decode(r, &in) || !validPassword(in.Password) {
		writeErr(w, 400, "invalid_request", "a token and a password of 10-72 characters are required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, 500, "internal", "could not create account")
		return
	}
	tx, err := g.DB.Begin(r.Context())
	if err != nil {
		writeErr(w, 500, "internal", "could not create account")
		return
	}
	defer tx.Rollback(r.Context())
	tenant, email, role, _, err := g.consume(r.Context(), tx, "invite", in.Token)
	if err != nil {
		writeErr(w, 400, "invalid_token", "this invite link is invalid, expired or already used")
		return
	}
	userID := newID("usr")
	if _, err := tx.Exec(r.Context(), `INSERT INTO users (id, email, password_hash) VALUES ($1,$2,$3)`, userID, email, string(hash)); err != nil {
		writeErr(w, 409, "conflict", "that email already has an account")
		return
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO memberships (user_id, tenant_id, role) VALUES ($1,$2,$3)`, userID, tenant, role); err != nil {
		writeErr(w, 500, "internal", "could not create account")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeErr(w, 500, "internal", "could not create account")
		return
	}
	resp, err := g.session(userID, email, tenant, role)
	if err != nil {
		writeErr(w, 500, "internal", "could not create session")
		return
	}
	writeJSON(w, 201, resp)
}

func (g *Service) resetPassword(w http.ResponseWriter, r *http.Request) {
	var in struct{ Token, Password string }
	if !decode(r, &in) || !validPassword(in.Password) {
		writeErr(w, 400, "invalid_request", "a token and a password of 10-72 characters are required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, 500, "internal", "could not reset password")
		return
	}
	tx, err := g.DB.Begin(r.Context())
	if err != nil {
		writeErr(w, 500, "internal", "could not reset password")
		return
	}
	defer tx.Rollback(r.Context())
	_, _, _, userID, err := g.consume(r.Context(), tx, "reset", in.Token)
	if err != nil || userID == "" {
		writeErr(w, 400, "invalid_token", "this reset link is invalid, expired or already used")
		return
	}
	if _, err := tx.Exec(r.Context(), `UPDATE users SET password_hash=$2 WHERE id=$1`, userID, string(hash)); err != nil {
		writeErr(w, 500, "internal", "could not reset password")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeErr(w, 500, "internal", "could not reset password")
		return
	}
	g.revoke(r.Context(), userID) // a reset ends every existing session
	w.WriteHeader(http.StatusNoContent)
}
