package incident

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/resolvex/resolve-x/backend/internal/llm"
	"github.com/resolvex/resolve-x/backend/internal/secrets"
)

// Notifier delivers incident events to Slack-compatible and generic webhooks.
// Channel URLs are stored encrypted (Slack URLs embed a secret) and requests go
// through the SSRF-safe client, so a channel cannot target internal services.
type Notifier struct {
	DB           *pgxpool.Pool
	Box          *secrets.Box
	Client       *http.Client
	PublicURL    string
	AllowPrivate bool
	Log          *slog.Logger
}

type Channel struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	URLHint   string    `json:"urlHint"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
}

type LogEntry struct {
	ChannelID  string    `json:"channelId"`
	Event      string    `json:"event"`
	IncidentID string    `json:"incidentId"`
	OK         bool      `json:"ok"`
	Error      string    `json:"error"`
	At         time.Time `json:"at"`
}

func (n *Notifier) List(ctx context.Context, tenant string) ([]Channel, error) {
	rows, err := n.DB.Query(ctx, `SELECT id, kind, name, url_hint, enabled, created_at FROM notification_channels WHERE tenant_id=$1 ORDER BY created_at`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Channel{}
	for rows.Next() {
		var c Channel
		if err := rows.Scan(&c.ID, &c.Kind, &c.Name, &c.URLHint, &c.Enabled, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Create stores a channel. For generic webhooks it returns the signing secret once.
func (n *Notifier) Create(ctx context.Context, tenant, kind, name, rawURL string) (Channel, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 80 {
		return Channel{}, "", errors.New("a name of up to 80 characters is required")
	}
	if kind != "slack" && kind != "webhook" {
		return Channel{}, "", errors.New("kind must be slack or webhook")
	}
	if err := llm.ValidateBaseURL(rawURL, n.AllowPrivate); err != nil || rawURL == "" {
		return Channel{}, "", errors.New("the URL must be a valid https URL")
	}
	u, _ := url.Parse(rawURL)
	if kind == "slack" && !n.AllowPrivate && !strings.HasSuffix(u.Hostname(), ".slack.com") && !strings.HasSuffix(u.Hostname(), ".office.com") && !strings.HasSuffix(u.Hostname(), ".webhook.office.com") {
		return Channel{}, "", errors.New("Slack webhooks must be on hooks.slack.com; use the generic webhook type for other services")
	}
	urlCipher, err := n.Box.Seal(rawURL)
	if err != nil {
		return Channel{}, "", err
	}
	secret, secretCipher := "", ""
	if kind == "webhook" {
		b := make([]byte, 24)
		_, _ = rand.Read(b)
		secret = "whsec_" + hex.EncodeToString(b)
		if secretCipher, err = n.Box.Seal(secret); err != nil {
			return Channel{}, "", err
		}
	}
	c := Channel{ID: newID("chn"), Kind: kind, Name: name, URLHint: u.Scheme + "://" + u.Host + "/…", Enabled: true}
	err = n.DB.QueryRow(ctx, `INSERT INTO notification_channels (id, tenant_id, kind, name, url_cipher, url_hint, secret_cipher) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING created_at`,
		c.ID, tenant, kind, name, urlCipher, c.URLHint, secretCipher).Scan(&c.CreatedAt)
	return c, secret, err
}

func (n *Notifier) Delete(ctx context.Context, tenant, id string) error {
	tag, err := n.DB.Exec(ctx, `DELETE FROM notification_channels WHERE id=$1 AND tenant_id=$2`, id, tenant)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (n *Notifier) Log50(ctx context.Context, tenant string) ([]LogEntry, error) {
	rows, err := n.DB.Query(ctx, `SELECT channel_id, event, incident_id, ok, error, at FROM notification_log WHERE tenant_id=$1 ORDER BY at DESC LIMIT 50`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LogEntry{}
	for rows.Next() {
		var e LogEntry
		if err := rows.Scan(&e.ChannelID, &e.Event, &e.IncidentID, &e.OK, &e.Error, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Event is what gets announced.
type Event struct {
	Type        string // incident.created | analysis.completed | incident.resolved | test
	Incident    Incident
	RootCause   string
	Confidence  int
	Recommended string
}

func (n *Notifier) link(inc Incident) string {
	if inc.ID == "" {
		return n.PublicURL + "/dashboard"
	}
	return n.PublicURL + "/dashboard/incidents/" + inc.ID
}

func (e Event) headline() string {
	i := e.Incident
	switch e.Type {
	case "incident.created":
		return fmt.Sprintf("🚨 [%s] %s", i.Severity, i.Title)
	case "analysis.completed":
		return fmt.Sprintf("🔎 Probable cause found for [%s] %s", i.Severity, i.Title)
	case "incident.resolved":
		return fmt.Sprintf("✅ Resolved: %s", i.Title)
	}
	return "Resolve-X test notification: your channel is connected."
}

func (n *Notifier) slackBody(e Event) []byte {
	text := e.headline()
	if e.Incident.Service != "" && e.Type != "test" {
		text += "\nService: " + e.Incident.Service
	}
	if e.RootCause != "" {
		text += fmt.Sprintf("\nProbable cause (%d%%): %s", e.Confidence, e.RootCause)
	}
	if e.Recommended != "" {
		text += "\nSuggested action: " + e.Recommended
	}
	text += "\n<" + n.link(e.Incident) + "|Open in Resolve-X>"
	b, _ := json.Marshal(map[string]string{"text": text})
	return b
}

func (n *Notifier) webhookBody(e Event) []byte {
	b, _ := json.Marshal(map[string]any{
		"event": e.Type, "incident": e.Incident, "url": n.link(e.Incident), "sentAt": time.Now().UTC(),
		"analysis": map[string]any{"rootCause": e.RootCause, "confidence": e.Confidence, "recommendation": e.Recommended},
	})
	return b
}

// Notify sends an event to every enabled channel of the tenant, in the background.
func (n *Notifier) Notify(tenant string, e Event) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		rows, err := n.DB.Query(ctx, `SELECT id, kind, url_cipher, secret_cipher FROM notification_channels WHERE tenant_id=$1 AND enabled`, tenant)
		if err != nil {
			n.Log.Error("notify: load channels", "err", err)
			return
		}
		type ch struct{ id, kind, urlC, secC string }
		var chans []ch
		for rows.Next() {
			var c ch
			if rows.Scan(&c.id, &c.kind, &c.urlC, &c.secC) == nil {
				chans = append(chans, c)
			}
		}
		rows.Close()
		for _, c := range chans {
			err := n.deliver(ctx, c.kind, c.urlC, c.secC, e)
			n.record(ctx, tenant, c.id, e, err)
		}
	}()
}

// Test sends a test message to one channel and returns the result synchronously.
func (n *Notifier) Test(ctx context.Context, tenant, id string) error {
	var kind, urlC, secC string
	err := n.DB.QueryRow(ctx, `SELECT kind, url_cipher, secret_cipher FROM notification_channels WHERE id=$1 AND tenant_id=$2`, id, tenant).Scan(&kind, &urlC, &secC)
	if err != nil {
		return ErrNotFound
	}
	e := Event{Type: "test"}
	err = n.deliver(ctx, kind, urlC, secC, e)
	n.record(ctx, tenant, id, e, err)
	return err
}

func (n *Notifier) record(ctx context.Context, tenant, channelID string, e Event, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	if _, derr := n.DB.Exec(ctx, `INSERT INTO notification_log (tenant_id, channel_id, event, incident_id, ok, error) VALUES ($1,$2,$3,$4,$5,$6)`,
		tenant, channelID, e.Type, e.Incident.ID, err == nil, msg[:min(len(msg), 300)]); derr != nil {
		n.Log.Error("notify: log", "err", derr)
	}
}

func (n *Notifier) deliver(ctx context.Context, kind, urlCipher, secretCipher string, e Event) error {
	target, err := n.Box.Open(urlCipher)
	if err != nil {
		return err
	}
	var body []byte
	headers := map[string]string{"Content-Type": "application/json"}
	if kind == "slack" {
		body = n.slackBody(e)
	} else {
		body = n.webhookBody(e)
		headers["X-Resolvex-Event"] = e.Type
		if secretCipher != "" {
			secret, err := n.Box.Open(secretCipher)
			if err != nil {
				return err
			}
			m := hmac.New(sha256.New, []byte(secret))
			m.Write(body)
			headers["X-Resolvex-Signature"] = "sha256=" + hex.EncodeToString(m.Sum(nil))
		}
	}
	var last error
	for attempt, wait := 0, time.Second; attempt < 3; attempt, wait = attempt+1, wait*3 {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := n.Client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
			resp.Body.Close()
			if resp.StatusCode < 300 {
				return nil
			}
			last = fmt.Errorf("endpoint returned HTTP %d", resp.StatusCode)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
				return last // our request is wrong; retrying will not help
			}
		} else if strings.Contains(err.Error(), "are not allowed") {
			return errors.New("that address is not allowed (private or internal network)")
		} else {
			last = errors.New("could not reach the endpoint")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return last
}
