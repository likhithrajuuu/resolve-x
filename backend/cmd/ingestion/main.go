// Ingestion Service: OTLP/gRPC on :4317, OTLP/HTTP on :4318, ops on :9100.
package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/resolvex/resolve-x/backend/internal/auth"
	"github.com/resolvex/resolve-x/backend/internal/config"
	"github.com/resolvex/resolve-x/backend/internal/ingest"
	"github.com/resolvex/resolve-x/backend/internal/platform"
)

func main() {
	log := platform.NewLogger("ingestion-service")
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

	pub, err := ingest.NewPublisher(config.List("KAFKA_BROKERS", []string{"localhost:9092"}), config.Duration("PUBLISH_TIMEOUT", 5*time.Second))
	if err != nil {
		log.Error("kafka", "err", err)
		os.Exit(1)
	}
	defer pub.Close()

	svc := &ingest.Service{Keys: auth.NewKeyStore(db, rdb), Pub: pub}
	maxBody := int64(config.Int("MAX_BODY_BYTES", 4<<20))

	health := &platform.Health{}
	platform.ServeOps(ctx, config.String("OPS_ADDR", ":9100"), health, log)

	grpcLis, err := net.Listen("tcp", config.String("GRPC_ADDR", ":4317"))
	if err != nil {
		log.Error("grpc listen", "err", err)
		os.Exit(1)
	}
	gs := svc.NewGRPCServer(int(maxBody))
	go func() {
		if err := gs.Serve(grpcLis); err != nil {
			log.Error("grpc", "err", err)
		}
	}()
	go func() { <-ctx.Done(); gs.GracefulStop() }()

	srv := &http.Server{
		Addr:              config.String("HTTP_ADDR", ":4318"),
		Handler:           svc.HTTPHandler(maxBody),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	// Ready once Kafka is reachable; Kubernetes stops routing traffic otherwise.
	go func() {
		for ctx.Err() == nil {
			pc, cancel := context.WithTimeout(ctx, 3*time.Second)
			health.SetReady(pub.Ping(pc) == nil)
			cancel()
			select {
			case <-ctx.Done():
			case <-time.After(5 * time.Second):
			}
		}
	}()
	log.Info("ingestion-service started", "http", srv.Addr, "grpc", grpcLis.Addr().String())
	if err := platform.Serve(ctx, srv, log); err != nil {
		log.Error("http", "err", err)
		os.Exit(1)
	}
}
