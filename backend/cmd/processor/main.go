// Telemetry Processor: telemetry.raw -> split topics + ClickHouse.
package main

import (
	"os"

	"github.com/resolvex/resolve-x/backend/internal/config"
	"github.com/resolvex/resolve-x/backend/internal/platform"
	"github.com/resolvex/resolve-x/backend/internal/processor"
	"github.com/resolvex/resolve-x/backend/internal/store"
)

func main() {
	log := platform.NewLogger("telemetry-processor")
	ctx, stop := platform.SignalContext()
	defer stop()

	ch, err := store.OpenCH(ctx,
		config.String("CLICKHOUSE_ADDR", "localhost:9000"), config.String("CLICKHOUSE_DB", "default"),
		config.String("CLICKHOUSE_USER", "default"), config.String("CLICKHOUSE_PASSWORD", ""))
	if err != nil {
		log.Error("clickhouse", "err", err)
		os.Exit(1)
	}
	defer ch.Close()

	health := &platform.Health{}
	health.SetReady(true)
	platform.ServeOps(ctx, config.String("OPS_ADDR", ":9100"), health, log)

	r := &processor.Runner{
		Brokers: config.List("KAFKA_BROKERS", []string{"localhost:9092"}),
		Group:   config.String("CONSUMER_GROUP", "telemetry-processor"),
		CH:      ch, Log: log,
	}
	log.Info("telemetry-processor started")
	if err := r.Run(ctx); err != nil {
		log.Error("processor", "err", err)
		os.Exit(1)
	}
}
