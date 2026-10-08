// Tenancy service on :8080 (behind Kong), ops on :9100.
package main

import (
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/resolvex/resolve-x/backend/internal/config"
	"github.com/resolvex/resolve-x/backend/internal/platform"
	"github.com/resolvex/resolve-x/backend/internal/tenancy"
)

func main() {
	log := platform.NewLogger("tenancy-service")
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
	svc := &tenancy.Service{
		DB: db, Redis: rdb, Log: log,
		Auth: tenancy.DevAuthenticator{Token: devToken, TenantID: config.String("GATEWAY_DEV_TENANT", "tnt_dev")},
	}

	health := &platform.Health{}
	health.SetReady(true)
	platform.ServeOps(ctx, config.String("OPS_ADDR", ":9100"), health, log)

	srv := &http.Server{Addr: config.String("HTTP_ADDR", ":8080"), Handler: svc.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second}
	log.Info("tenancy-service started", "addr", srv.Addr)
	if err := platform.Serve(ctx, srv, log); err != nil {
		log.Error("http", "err", err)
		os.Exit(1)
	}
}
