// Assistant service: AI chat and AI provider settings. Behind Kong.
package main

import (
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/redis/go-redis/v9"

	"github.com/resolvex/resolve-x/backend/internal/aiconfig"
	"github.com/resolvex/resolve-x/backend/internal/assistant"
	"github.com/resolvex/resolve-x/backend/internal/authn"
	"github.com/resolvex/resolve-x/backend/internal/config"
	"github.com/resolvex/resolve-x/backend/internal/llm"
	"github.com/resolvex/resolve-x/backend/internal/platform"
	"github.com/resolvex/resolve-x/backend/internal/secrets"
)

func main() {
	log := platform.NewLogger("assistant-service")
	ctx, stop := platform.SignalContext()
	defer stop()

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		log.Error("JWT_SECRET must be set to at least 32 characters")
		os.Exit(1)
	}
	box, err := secrets.New(os.Getenv("SECRETS_KEY"))
	if err != nil {
		log.Error("secrets", "err", err)
		os.Exit(1)
	}
	db, err := pgxpool.New(ctx, config.String("POSTGRES_URL", "postgres://resolvex:resolvex@localhost:5432/resolvex"))
	if err != nil {
		log.Error("postgres", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{Addr: config.String("REDIS_ADDR", "localhost:6379")})
	defer rdb.Close()

	allowPrivate := config.String("AI_ALLOW_PRIVATE_BASE_URLS", "false") == "true"
	if allowPrivate {
		log.Warn("AI_ALLOW_PRIVATE_BASE_URLS=true: tenants may point the AI provider at private addresses (use only for self-hosted installs)")
	}
	api := &assistant.API{
		Store:        &aiconfig.Store{DB: db, Box: box},
		Backend:      &assistant.Backend{QueryURL: config.String("QUERY_URL", "http://localhost:8081"), IncidentURL: config.String("INCIDENT_URL", "http://localhost:8082"), HTTP: &http.Client{Timeout: 20 * time.Second}},
		Secret:       []byte(secret),
		Revoker:      &authn.RedisRevoker{R: rdb, TTL: 13 * time.Hour},
		LLMClient:    llm.SafeClient(allowPrivate, 90*time.Second),
		AllowPrivate: allowPrivate,
	}
	health := &platform.Health{}
	health.SetReady(true)
	platform.ServeOps(ctx, config.String("OPS_ADDR", ":9100"), health, log)

	srv := &http.Server{Addr: config.String("HTTP_ADDR", ":8083"), Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 120 * time.Second}
	log.Info("assistant-service started", "addr", srv.Addr)
	if err := platform.Serve(ctx, srv, log); err != nil {
		log.Error("http", "err", err)
		os.Exit(1)
	}
}
