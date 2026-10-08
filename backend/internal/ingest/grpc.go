package ingest

import (
	"context"
	"errors"
	"strings"

	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/resolvex/resolve-x/backend/internal/auth"
	"github.com/resolvex/resolve-x/backend/internal/telemetry"
)

// NewGRPCServer serves OTLP/gRPC on the standard collector services.
func (s *Service) NewGRPCServer(maxMsg int) *grpc.Server {
	srv := grpc.NewServer(grpc.MaxRecvMsgSize(maxMsg))
	tracev1.RegisterTraceServiceServer(srv, &traceSvc{s: s})
	metricsv1.RegisterMetricsServiceServer(srv, &metricsSvc{s: s})
	logsv1.RegisterLogsServiceServer(srv, &logsSvc{s: s})
	return srv
}

func grpcKey(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	if v := md.Get("x-resolvex-key"); len(v) > 0 {
		return v[0]
	}
	if v := md.Get("authorization"); len(v) > 0 {
		return strings.TrimPrefix(v[0], "Bearer ")
	}
	return ""
}

func (s *Service) grpcExport(ctx context.Context, sig telemetry.Signal, m proto.Message) error {
	outcome := func(o string) { requests.WithLabelValues(string(sig), "grpc", o).Inc() }
	key := grpcKey(ctx)
	if key == "" {
		outcome("unauthenticated")
		return status.Error(codes.Unauthenticated, "missing api key")
	}
	body, err := proto.Marshal(m)
	if err != nil {
		outcome("bad_request")
		return status.Error(codes.InvalidArgument, "cannot encode request")
	}
	switch err := s.Accept(ctx, key, sig, "application/x-protobuf", "", body); {
	case err == nil:
		outcome("accepted")
		return nil
	case errors.Is(err, auth.ErrInvalidKey):
		outcome("unauthenticated")
		return status.Error(codes.Unauthenticated, "invalid api key")
	case errors.Is(err, ErrRateLimited):
		outcome("rate_limited")
		return status.Error(codes.ResourceExhausted, "rate limit exceeded")
	default:
		outcome("unavailable")
		return status.Error(codes.Unavailable, "temporarily unavailable")
	}
}

type traceSvc struct {
	tracev1.UnimplementedTraceServiceServer
	s *Service
}

func (t *traceSvc) Export(ctx context.Context, r *tracev1.ExportTraceServiceRequest) (*tracev1.ExportTraceServiceResponse, error) {
	if err := t.s.grpcExport(ctx, telemetry.Traces, r); err != nil {
		return nil, err
	}
	return &tracev1.ExportTraceServiceResponse{}, nil
}

type metricsSvc struct {
	metricsv1.UnimplementedMetricsServiceServer
	s *Service
}

func (t *metricsSvc) Export(ctx context.Context, r *metricsv1.ExportMetricsServiceRequest) (*metricsv1.ExportMetricsServiceResponse, error) {
	if err := t.s.grpcExport(ctx, telemetry.Metrics, r); err != nil {
		return nil, err
	}
	return &metricsv1.ExportMetricsServiceResponse{}, nil
}

type logsSvc struct {
	logsv1.UnimplementedLogsServiceServer
	s *Service
}

func (t *logsSvc) Export(ctx context.Context, r *logsv1.ExportLogsServiceRequest) (*logsv1.ExportLogsServiceResponse, error) {
	if err := t.s.grpcExport(ctx, telemetry.Logs, r); err != nil {
		return nil, err
	}
	return &logsv1.ExportLogsServiceResponse{}, nil
}
