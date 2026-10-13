// Analysis Service: correlation + root-cause analysis worker.
// Consumes incident.created / analysis.requested, publishes analysis.completed.
package main

import (
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/resolvex/resolve-x/backend/internal/aiconfig"
	"github.com/resolvex/resolve-x/backend/internal/analysis"
	"github.com/resolvex/resolve-x/backend/internal/config"
	"github.com/resolvex/resolve-x/backend/internal/events"
	"github.com/resolvex/resolve-x/backend/internal/llm"
	"github.com/resolvex/resolve-x/backend/internal/platform"
	"github.com/resolvex/resolve-x/backend/internal/secrets"
	"github.com/resolvex/resolve-x/backend/internal/store"
)

func main() {
	log := platform.NewLogger("analysis-service")
	ctx, stop := platform.SignalContext()
	defer stop()

	brokers := config.List("KAFKA_BROKERS", []string{"localhost:9092"})
	db, err := pgxpool.New(ctx, config.String("POSTGRES_URL", "postgres://resolvex:resolvex@localhost:5432/resolvex"))
	if err != nil {
		log.Error("postgres", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	ch, err := store.OpenCHReadOnly(ctx, config.String("CLICKHOUSE_ADDR", "localhost:9000"), config.String("CLICKHOUSE_DB", "default"),
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

	w := &analysis.Worker{G: &analysis.Gatherer{CH: ch, DB: db}, Pub: pub, Log: log}
	if key := os.Getenv("SECRETS_KEY"); key != "" {
		box, err := secrets.New(key)
		if err != nil {
			log.Error("secrets", "err", err)
			os.Exit(1)
		}
		w.AI = &aiconfig.Store{DB: db, Box: box}
		w.Client = llm.SafeClient(config.String("AI_ALLOW_PRIVATE_BASE_URLS", "false") == "true", 90*time.Second)
		log.Info("LLM refinement available: organizations that configure an AI provider send correlated evidence (including log text) to it")
	} else {
		log.Info("SECRETS_KEY not set; using the heuristic analyzer only")
	}

	health := &platform.Health{}
	health.SetReady(true)
	platform.ServeOps(ctx, config.String("OPS_ADDR", ":9100"), health, log)
	log.Info("analysis-service started")
	if err := events.Consume(ctx, log, brokers, "analysis-service", []string{events.TopicIncidentCreated, events.TopicAnalysisRequested}, w.Handle); err != nil {
		log.Error("consumer", "err", err)
		os.Exit(1)
	}
}
