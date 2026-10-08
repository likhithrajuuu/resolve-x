package ingest

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/resolvex/resolve-x/backend/internal/auth"
	"github.com/resolvex/resolve-x/backend/internal/telemetry"
)

// HTTPHandler serves OTLP/HTTP: POST /v1/{traces,metrics,logs}.
func (s *Service) HTTPHandler(maxBody int64) http.Handler {
	mux := http.NewServeMux()
	for _, sig := range []telemetry.Signal{telemetry.Traces, telemetry.Metrics, telemetry.Logs} {
		mux.HandleFunc("POST /v1/"+string(sig), s.httpExport(sig, maxBody))
	}
	return mux
}

func apiKey(r *http.Request) string {
	if k := r.Header.Get("X-Resolvex-Key"); k != "" {
		return k
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

func (s *Service) httpExport(sig telemetry.Signal, maxBody int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		outcome := func(o string) { requests.WithLabelValues(string(sig), "http", o).Inc() }

		ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if ct != "application/x-protobuf" && ct != "application/json" {
			outcome("bad_request")
			http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
			return
		}
		key := apiKey(r)
		if key == "" {
			outcome("unauthenticated")
			http.Error(w, "missing api key", http.StatusUnauthorized)
			return
		}
		enc := r.Header.Get("Content-Encoding")
		if enc != "" && enc != "gzip" && enc != "identity" {
			outcome("bad_request")
			http.Error(w, "unsupported content encoding", http.StatusBadRequest)
			return
		}
		if enc == "identity" {
			enc = ""
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				outcome("too_large")
				http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
				return
			}
			outcome("bad_request")
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}
		if len(body) == 0 {
			outcome("bad_request")
			http.Error(w, "empty body", http.StatusBadRequest)
			return
		}

		switch err := s.Accept(r.Context(), key, sig, ct, enc, body); {
		case err == nil:
			outcome("accepted")
			w.Header().Set("Content-Type", ct)
			if ct == "application/json" {
				_, _ = w.Write([]byte("{}"))
			}
		case errors.Is(err, auth.ErrInvalidKey):
			outcome("unauthenticated")
			http.Error(w, "invalid api key", http.StatusUnauthorized)
		case errors.Is(err, ErrRateLimited):
			outcome("rate_limited")
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		default:
			outcome("unavailable")
			w.Header().Set("Retry-After", "2")
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
		}
	}
}
