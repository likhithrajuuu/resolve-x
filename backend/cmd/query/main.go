// Query API: telemetry reads, service catalog and dependency graph. Behind Kong.
package main

import (
	"net/http"
	"os"
	"time"

	"github.com/resolvex/resolve-x/backend/internal/config"
	"github.com/resolvex/resolve-x/backend/internal/platform"
	"github.com/resolvex/resolve-x/backend/internal/query"
	"github.com/resolvex/resolve-x/backend/internal/store"
)

func main() {
	log := platform.NewLogger("query-service")
	ctx, stop := platform.SignalContext()
	defer stop()

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		log.Error("JWT_SECRET must be set to at least 32 characters")
		os.Exit(1)
	}
	ch, err := store.OpenCH(ctx, config.String("CLICKHOUSE_ADDR", "localhost:9000"), config.String("CLICKHOUSE_DB", "default"),
		config.String("CLICKHOUSE_USER", "default"), config.String("CLICKHOUSE_PASSWORD", ""))
	if err != nil {
		log.Error("clickhouse", "err", err)
		os.Exit(1)
	}
	defer ch.Close()

	health := &platform.Health{}
	health.SetReady(true)
	platform.ServeOps(ctx, config.String("OPS_ADDR", ":9100"), health, log)

	srv := &http.Server{Addr: config.String("HTTP_ADDR", ":8081"), Handler: (&query.API{CH: ch, Secret: []byte(secret)}).Handler(),
		ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 30 * time.Second}
	log.Info("query-service started", "addr", srv.Addr)
	if err := platform.Serve(ctx, srv, log); err != nil {
		log.Error("http", "err", err)
		os.Exit(1)
	}
}
