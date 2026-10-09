// Incident Service: lifecycle, webhooks, detector, outbox relay. Behind Kong.
package main

import (
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/resolvex/resolve-x/backend/internal/auth"
	"github.com/resolvex/resolve-x/backend/internal/config"
	"github.com/resolvex/resolve-x/backend/internal/events"
	"github.com/resolvex/resolve-x/backend/internal/incident"
	"github.com/resolvex/resolve-x/backend/internal/platform"
	"github.com/resolvex/resolve-x/backend/internal/store"
)

func main() {
	log := platform.NewLogger("incident-service")
	ctx, stop := platform.SignalContext()
	defer stop()

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		log.Error("JWT_SECRET must be set to at least 32 characters")
		os.Exit(1)
	}
	brokers := config.List("KAFKA_BROKERS", []string{"localhost:9092"})
	db, err := pgxpool.New(ctx, config.String("POSTGRES_URL", "postgres://resolvex:resolvex@localhost:5432/resolvex"))
	if err != nil {
		log.Error("postgres", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	rdb := redis.NewClient(&redis.Options{Addr: config.String("REDIS_ADDR", "localhost:6379")})
	defer rdb.Close()
	ch, err := store.OpenCH(ctx, config.String("CLICKHOUSE_ADDR", "localhost:9000"), config.String("CLICKHOUSE_DB", "default"),
		config.String("CLICKHOUSE_USER", "default"), config.String("CLICKHOUSE_PASSWORD", ""))
	if err != nil {
		log.Error("clickhouse", "err", err)
		os.Exit(1)
	}
	defer ch.Close()
	pub, err := events.NewProducer(brokers)
	if err != nil {
		log.Error("kafka", "err", err)
		os.Exit(1)
	}
	defer pub.Close()

	svc := &incident.Service{DB: db, Log: log}
	go svc.RelayOutbox(ctx, pub)
	go func() {
		if err := events.Consume(ctx, log, brokers, "incident-service", []string{events.TopicAnalysisCompleted}, svc.HandleAnalysisCompleted); err != nil {
			log.Error("consumer", "err", err)
		}
	}()
	go (&incident.Evaluator{Svc: svc, CH: ch, Interval: config.Duration("RULE_INTERVAL", 30*time.Second)}).Run(ctx)
	if config.String("DETECTOR_ENABLED", "true") == "true" {
		det := &incident.Detector{Svc: svc, CH: ch, Interval: config.Duration("DETECTOR_INTERVAL", 30*time.Second),
			MinSpans: uint64(config.Int("DETECTOR_MIN_SPANS", 20)), ErrorRate: 0.10, LatencyFactor: 3}
		go det.Run(ctx)
	}

	health := &platform.Health{}
	health.SetReady(true)
	platform.ServeOps(ctx, config.String("OPS_ADDR", ":9100"), health, log)

	api := &incident.API{CH: ch, Svc: svc, Keys: auth.NewKeyStore(db, rdb), Secret: []byte(secret)}
	srv := &http.Server{Addr: config.String("HTTP_ADDR", ":8082"), Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second}
	log.Info("incident-service started", "addr", srv.Addr)
	if err := platform.Serve(ctx, srv, log); err != nil {
		log.Error("http", "err", err)
		os.Exit(1)
	}
}
