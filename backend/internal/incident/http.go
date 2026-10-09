package incident

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/resolvex/resolve-x/backend/internal/auth"
	"github.com/resolvex/resolve-x/backend/internal/authn"
	"github.com/resolvex/resolve-x/backend/internal/store"
)

type API struct {
	CH     *store.CH
	Svc    *Service
	Keys   *auth.KeyStore
	Secret []byte
}

func (a *API) Handler() http.Handler {
	user := http.NewServeMux()
	user.HandleFunc("GET /api/v1/incidents", a.list)
	user.HandleFunc("POST /api/v1/incidents", a.create)
	user.HandleFunc("GET /api/v1/incidents/{id}", a.get)
	user.HandleFunc("PATCH /api/v1/incidents/{id}", a.patch)
	user.HandleFunc("POST /api/v1/incidents/{id}/analyze", a.analyze)
	user.HandleFunc("POST /api/v1/analyses/{id}/approve", a.decide("approved"))
	user.HandleFunc("POST /api/v1/analyses/{id}/reject", a.decide("rejected"))
	user.HandleFunc("GET /api/v1/deployments", a.deployments)
	user.HandleFunc("GET /api/v1/alert-rules", a.listRules)
	user.HandleFunc("POST /api/v1/alert-rules", a.createRule)
	user.HandleFunc("PATCH /api/v1/alert-rules/{id}", a.patchRule)
	user.HandleFunc("DELETE /api/v1/alert-rules/{id}", a.deleteRule)
	user.HandleFunc("GET /api/v1/slos", a.listSLOs)
	user.HandleFunc("POST /api/v1/slos", a.createSLO)
	user.HandleFunc("DELETE /api/v1/slos/{id}", a.deleteSLO)

	root := http.NewServeMux()
	root.HandleFunc("POST /api/v1/webhooks/alerts", a.alertWebhook)
	root.HandleFunc("POST /api/v1/webhooks/deployments", a.deploymentWebhook)
	root.Handle("/", authn.Require(a.Secret, user))
	return root
}

func claims(r *http.Request) *authn.Claims { return authn.FromContext(r.Context()) }

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	if json.NewDecoder(r.Body).Decode(v) != nil {
		authn.WriteError(w, 400, "invalid_request", "invalid JSON body")
		return false
	}
	return true
}

func serverErr(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		authn.WriteError(w, 404, "not_found", "not found")
		return
	}
	authn.WriteError(w, 500, "internal", "request failed")
}

var severities = map[string]bool{"SEV-1": true, "SEV-2": true, "SEV-3": true}

func (a *API) list(w http.ResponseWriter, r *http.Request) {
	out, err := a.Svc.List(r.Context(), claims(r).Tenant, r.URL.Query().Get("status"))
	if err != nil {
		serverErr(w, err)
		return
	}
	authn.WriteJSON(w, 200, out)
}

func (a *API) create(w http.ResponseWriter, r *http.Request) {
	var in struct{ Title, Service, Severity, Description string }
	if !decodeBody(w, r, &in) {
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" || len(in.Title) > 200 || !severities[in.Severity] {
		authn.WriteError(w, 400, "invalid_request", "title and a severity of SEV-1, SEV-2 or SEV-3 are required")
		return
	}
	inc, _, err := a.Svc.Create(r.Context(), claims(r).Tenant, NewIncident{Title: in.Title, Service: strings.TrimSpace(in.Service),
		Severity: in.Severity, Description: in.Description, Source: "manual"})
	if err != nil {
		serverErr(w, err)
		return
	}
	authn.WriteJSON(w, 201, inc)
}

func (a *API) get(w http.ResponseWriter, r *http.Request) {
	d, err := a.Svc.Get(r.Context(), claims(r).Tenant, r.PathValue("id"))
	if err != nil {
		serverErr(w, err)
		return
	}
	authn.WriteJSON(w, 200, d)
}

func (a *API) patch(w http.ResponseWriter, r *http.Request) {
	var in struct{ Status string }
	if !decodeBody(w, r, &in) {
		return
	}
	if !validStatus[in.Status] {
		authn.WriteError(w, 400, "invalid_request", "status must be open, investigating, identified or resolved")
		return
	}
	if err := a.Svc.SetStatus(r.Context(), claims(r).Tenant, r.PathValue("id"), in.Status, claims(r).Email); err != nil {
		serverErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) analyze(w http.ResponseWriter, r *http.Request) {
	if err := a.Svc.RequestAnalysis(r.Context(), claims(r).Tenant, r.PathValue("id"), claims(r).Email); err != nil {
		serverErr(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (a *API) decide(decision string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := a.Svc.Decide(r.Context(), claims(r).Tenant, r.PathValue("id"), decision, claims(r).Email); err != nil {
			serverErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *API) deployments(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Svc.DB.Query(r.Context(), `SELECT id, service, version, environment, at FROM deployments WHERE tenant_id=$1 ORDER BY at DESC LIMIT 100`, claims(r).Tenant)
	if err != nil {
		serverErr(w, err)
		return
	}
	defer rows.Close()
	type dep struct {
		ID          string    `json:"id"`
		Service     string    `json:"service"`
		Version     string    `json:"version"`
		Environment string    `json:"environment"`
		At          time.Time `json:"at"`
	}
	out := []dep{}
	for rows.Next() {
		var d dep
		if rows.Scan(&d.ID, &d.Service, &d.Version, &d.Environment, &d.At) == nil {
			out = append(out, d)
		}
	}
	authn.WriteJSON(w, 200, out)
}

// webhookTenant authenticates a webhook caller by project API key.
func (a *API) webhookTenant(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := r.Header.Get("X-Resolvex-Key")
	if key == "" {
		key = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	if key == "" {
		authn.WriteError(w, 401, "unauthenticated", "missing api key")
		return "", false
	}
	p, err := a.Keys.Resolve(r.Context(), key)
	if errors.Is(err, auth.ErrInvalidKey) {
		authn.WriteError(w, 401, "unauthenticated", "invalid api key")
		return "", false
	}
	if err != nil {
		authn.WriteError(w, 503, "unavailable", "try again")
		return "", false
	}
	return p.TenantID, true
}

func mapSeverity(s string) string {
	switch strings.ToLower(s) {
	case "critical", "page", "sev-1", "sev1", "p1":
		return "SEV-1"
	case "warning", "sev-3", "sev3", "info", "p3":
		return "SEV-3"
	}
	return "SEV-2"
}

// alertWebhook accepts a generic alert or a Prometheus Alertmanager payload.
func (a *API) alertWebhook(w http.ResponseWriter, r *http.Request) {
	tenant, ok := a.webhookTenant(w, r)
	if !ok {
		return
	}
	var in struct {
		Title, Service, Severity, Description, Fingerprint string
		Alerts                                             []struct {
			Status      string            `json:"status"`
			Labels      map[string]string `json:"labels"`
			Annotations map[string]string `json:"annotations"`
			Fingerprint string            `json:"fingerprint"`
		} `json:"alerts"`
	}
	if !decodeBody(w, r, &in) {
		return
	}
	var items []NewIncident
	if len(in.Alerts) > 0 { // Alertmanager
		for _, al := range in.Alerts {
			if al.Status == "resolved" {
				continue
			}
			title := al.Annotations["summary"]
			if title == "" {
				title = al.Labels["alertname"]
			}
			svc := al.Labels["service"]
			fp := al.Fingerprint
			if fp == "" {
				fp = al.Labels["alertname"] + ":" + svc
			}
			items = append(items, NewIncident{Title: title, Service: svc, Severity: mapSeverity(al.Labels["severity"]),
				Description: al.Annotations["description"], Source: "alert", Fingerprint: "alert:" + fp})
		}
	} else {
		fp := in.Fingerprint
		if fp == "" {
			fp = in.Title + ":" + in.Service
		}
		items = append(items, NewIncident{Title: in.Title, Service: in.Service, Severity: mapSeverity(in.Severity),
			Description: in.Description, Source: "alert", Fingerprint: "alert:" + fp})
	}
	type result struct {
		ID      string `json:"id"`
		Created bool   `json:"created"`
	}
	out := []result{}
	for _, it := range items {
		if strings.TrimSpace(it.Title) == "" || len(it.Title) > 200 {
			continue
		}
		inc, created, err := a.Svc.Create(r.Context(), tenant, it)
		if err != nil {
			serverErr(w, err)
			return
		}
		out = append(out, result{inc.ID, created})
	}
	authn.WriteJSON(w, 202, out)
}

func (a *API) deploymentWebhook(w http.ResponseWriter, r *http.Request) {
	tenant, ok := a.webhookTenant(w, r)
	if !ok {
		return
	}
	var in struct {
		Service, Version, Environment string
		Timestamp                     *time.Time
	}
	if !decodeBody(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Service) == "" || strings.TrimSpace(in.Version) == "" {
		authn.WriteError(w, 400, "invalid_request", "service and version are required")
		return
	}
	at := time.Now()
	if in.Timestamp != nil {
		at = *in.Timestamp
	}
	id := newID("dep")
	if _, err := a.Svc.DB.Exec(r.Context(), `INSERT INTO deployments (id, tenant_id, service, version, environment, at) VALUES ($1,$2,$3,$4,$5,$6)`,
		id, tenant, in.Service, in.Version, in.Environment, at); err != nil {
		serverErr(w, err)
		return
	}
	authn.WriteJSON(w, 201, map[string]string{"id": id})
}

func (a *API) listRules(w http.ResponseWriter, r *http.Request) {
	out, err := a.Svc.ListRules(r.Context(), claims(r).Tenant)
	if err != nil {
		serverErr(w, err)
		return
	}
	authn.WriteJSON(w, 200, out)
}

func (a *API) createRule(w http.ResponseWriter, r *http.Request) {
	var in Rule
	if !decodeBody(w, r, &in) {
		return
	}
	if err := in.Validate(); err != nil {
		authn.WriteError(w, 400, "invalid_request", err.Error())
		return
	}
	out, err := a.Svc.CreateRule(r.Context(), claims(r).Tenant, in)
	if err != nil {
		serverErr(w, err)
		return
	}
	authn.WriteJSON(w, 201, out)
}

func (a *API) patchRule(w http.ResponseWriter, r *http.Request) {
	var in struct{ Enabled *bool }
	if !decodeBody(w, r, &in) || in.Enabled == nil {
		return
	}
	if err := a.Svc.SetRuleEnabled(r.Context(), claims(r).Tenant, r.PathValue("id"), *in.Enabled); err != nil {
		serverErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) deleteRule(w http.ResponseWriter, r *http.Request) {
	if err := a.Svc.DeleteRule(r.Context(), claims(r).Tenant, r.PathValue("id")); err != nil {
		serverErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listSLOs(w http.ResponseWriter, r *http.Request) {
	slos, err := a.Svc.ListSLOs(r.Context(), claims(r).Tenant)
	if err != nil {
		serverErr(w, err)
		return
	}
	out := make([]SLOStatus, 0, len(slos))
	for _, o := range slos {
		st, err := Evaluate(r.Context(), a.CH, claims(r).Tenant, o, time.Now())
		if err != nil {
			serverErr(w, err)
			return
		}
		out = append(out, st)
	}
	authn.WriteJSON(w, 200, out)
}

func (a *API) createSLO(w http.ResponseWriter, r *http.Request) {
	var in SLO
	if !decodeBody(w, r, &in) {
		return
	}
	if err := in.Validate(); err != nil {
		authn.WriteError(w, 400, "invalid_request", err.Error())
		return
	}
	out, err := a.Svc.CreateSLO(r.Context(), claims(r).Tenant, in)
	if err != nil {
		serverErr(w, err)
		return
	}
	authn.WriteJSON(w, 201, out)
}

func (a *API) deleteSLO(w http.ResponseWriter, r *http.Request) {
	if err := a.Svc.DeleteSLO(r.Context(), claims(r).Tenant, r.PathValue("id")); err != nil {
		serverErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
