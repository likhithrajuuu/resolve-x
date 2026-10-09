// Analysis Service: correlation + root-cause analysis worker.
// Consumes incident.created / analysis.requested, publishes analysis.completed.
package main

import (
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/resolvex/resolve-x/backend/internal/analysis"
	"github.com/resolvex/resolve-x/backend/internal/config"
	"github.com/resolvex/resolve-x/backend/internal/events"
	"github.com/resolvex/resolve-x/backend/internal/platform"
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

	w := &analysis.Worker{G: &analysis.Gatherer{CH: ch, DB: db}, Pub: pub, Log: log}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		w.LLM = &analysis.LLM{APIKey: key, Model: config.String("ANTHROPIC_MODEL", "claude-sonnet-5-5"), Client: &http.Client{Timeout: 60 * time.Second}}
		log.Warn("LLM analysis enabled: correlated evidence is sent to the Anthropic API", "model", w.LLM.Model)
	} else {
		log.Info("LLM analysis disabled (no ANTHROPIC_API_KEY); using the heuristic analyzer")
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
