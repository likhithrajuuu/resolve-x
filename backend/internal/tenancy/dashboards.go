package tenancy

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type dashboard struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Widgets   json.RawMessage `json:"widgets"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

func (g *Service) mountDashboards(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/dashboards", g.secure(g.listDashboards))
	mux.HandleFunc("POST /api/v1/dashboards", g.secure(g.createDashboard))
	mux.HandleFunc("GET /api/v1/dashboards/{id}", g.secure(g.getDashboard))
	mux.HandleFunc("PUT /api/v1/dashboards/{id}", g.secure(g.putDashboard))
	mux.HandleFunc("DELETE /api/v1/dashboards/{id}", g.secure(g.deleteDashboard))
}

func (g *Service) listDashboards(w http.ResponseWriter, r *http.Request, id Identity) {
	rows, err := g.DB.Query(r.Context(), `SELECT id, name, widgets, updated_at FROM dashboards WHERE tenant_id=$1 ORDER BY created_at`, id.TenantID)
	if err != nil {
		writeErr(w, 500, "internal", "query failed")
		return
	}
	defer rows.Close()
	out := []dashboard{}
	for rows.Next() {
		var d dashboard
		if rows.Scan(&d.ID, &d.Name, &d.Widgets, &d.UpdatedAt) == nil {
			out = append(out, d)
		}
	}
	writeJSON(w, 200, out)
}

func validWidgets(raw json.RawMessage) bool {
	var ws []map[string]any
	return len(raw) <= 256<<10 && json.Unmarshal(raw, &ws) == nil && len(ws) <= 60
}

func (g *Service) createDashboard(w http.ResponseWriter, r *http.Request, id Identity) {
	var in dashboard
	if !decode(r, &in) || strings.TrimSpace(in.Name) == "" || len(in.Name) > 120 {
		writeErr(w, 400, "invalid_request", "name is required")
		return
	}
	if len(in.Widgets) == 0 {
		in.Widgets = json.RawMessage("[]")
	}
	if !validWidgets(in.Widgets) {
		writeErr(w, 400, "invalid_request", "widgets must be a JSON array of at most 60 widgets")
		return
	}
	in.ID = newID("dsh")
	if err := g.DB.QueryRow(r.Context(), `INSERT INTO dashboards (id, tenant_id, name, widgets) VALUES ($1,$2,$3,$4) RETURNING updated_at`,
		in.ID, id.TenantID, strings.TrimSpace(in.Name), in.Widgets).Scan(&in.UpdatedAt); err != nil {
		writeErr(w, 500, "internal", "could not create dashboard")
		return
	}
	g.audit(r.Context(), id, "dashboard.create", in.ID)
	writeJSON(w, 201, in)
}

func (g *Service) getDashboard(w http.ResponseWriter, r *http.Request, id Identity) {
	var d dashboard
	err := g.DB.QueryRow(r.Context(), `SELECT id, name, widgets, updated_at FROM dashboards WHERE id=$1 AND tenant_id=$2`, r.PathValue("id"), id.TenantID).
		Scan(&d.ID, &d.Name, &d.Widgets, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, 404, "not_found", "dashboard not found")
		return
	}
	if err != nil {
		writeErr(w, 500, "internal", "query failed")
		return
	}
	writeJSON(w, 200, d)
}

func (g *Service) putDashboard(w http.ResponseWriter, r *http.Request, id Identity) {
	var in dashboard
	if !decode(r, &in) || strings.TrimSpace(in.Name) == "" || len(in.Name) > 120 || !validWidgets(in.Widgets) {
		writeErr(w, 400, "invalid_request", "a name and a widgets array (max 60) are required")
		return
	}
	tag, err := g.DB.Exec(r.Context(), `UPDATE dashboards SET name=$3, widgets=$4, updated_at=now() WHERE id=$1 AND tenant_id=$2`,
		r.PathValue("id"), id.TenantID, strings.TrimSpace(in.Name), in.Widgets)
	if err != nil {
		writeErr(w, 500, "internal", "could not save dashboard")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, 404, "not_found", "dashboard not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (g *Service) deleteDashboard(w http.ResponseWriter, r *http.Request, id Identity) {
	tag, err := g.DB.Exec(r.Context(), `DELETE FROM dashboards WHERE id=$1 AND tenant_id=$2`, r.PathValue("id"), id.TenantID)
	if err != nil || tag.RowsAffected() == 0 {
		writeErr(w, 404, "not_found", "dashboard not found")
		return
	}
	g.audit(r.Context(), id, "dashboard.delete", r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}
