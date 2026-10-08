// API Gateway on :8080, ops on :9100.
package main

import (
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/resolvex/resolve-x/backend/internal/config"
	"github.com/resolvex/resolve-x/backend/internal/gatewayapi"
	"github.com/resolvex/resolve-x/backend/internal/platform"
)

func upstreams(log *slog.Logger) map[string]*url.URL {
	routes := map[string]string{
		"/api/v1/services/":     "SERVICE_REGISTRY_URL",
		"/api/v1/dependencies/": "SERVICE_REGISTRY_URL",
		"/api/v1/incidents/":    "INCIDENT_SERVICE_URL",
		"/api/v1/analyses/":     "ANALYSIS_SERVICE_URL",
	}
	out := map[string]*url.URL{}
	for prefix, env := range routes {
		if v := os.Getenv(env); v != "" {
			u, err := url.Parse(v)
			if err != nil {
				log.Error("bad upstream url", "env", env, "err", err)
				os.Exit(1)
			}
			out[prefix] = u
		}
	}
	return out
}

func main() {
	log := platform.NewLogger("api-gateway")
	ctx, stop := platform.SignalContext()
	defer stop()

	db, err := pgxpool.New(ctx, config.String("POSTGRES_URL", "postgres://resolvex:resolvex@localhost:5432/resolvex"))
	if err != nil {
		log.Error("postgres", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	rdb := redis.NewClient(&redis.Options{Addr: config.String("REDIS_ADDR", "localhost:6379")})
	defer rdb.Close()

	devToken := os.Getenv("GATEWAY_DEV_TOKEN")
	if devToken == "" {
		log.Error("no authenticator configured: set GATEWAY_DEV_TOKEN for local development (OIDC is not implemented yet)")
		os.Exit(1)
	}
	gw := &gatewayapi.Gateway{
		DB: db, Redis: rdb, Log: log, Upstreams: upstreams(log),
		Auth: gatewayapi.DevAuthenticator{Token: devToken, TenantID: config.String("GATEWAY_DEV_TENANT", "tnt_dev")},
	}

	health := &platform.Health{}
	health.SetReady(true)
	platform.ServeOps(ctx, config.String("OPS_ADDR", ":9100"), health, log)

	srv := &http.Server{Addr: config.String("HTTP_ADDR", ":8080"), Handler: gw.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second}
	log.Info("api-gateway started", "addr", srv.Addr)
	if err := platform.Serve(ctx, srv, log); err != nil {
		log.Error("http", "err", err)
		os.Exit(1)
	}
}
