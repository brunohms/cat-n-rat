// Package api exposes the backend HTTP routes.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/brunohms/cat-n-rat/app/backend/internal/purchase"
)

// Generous limit for a full batch of events (each event is around 1 KB)
const maxBodyBytes = 64 << 20

type EventFetcher interface {
	FetchEvents(ctx context.Context, count int) ([]purchase.Event, error)
}

type Server struct {
	pool      *pgxpool.Pool
	store     *purchase.Store
	generator EventFetcher
	maxBatch  int
	logger    *slog.Logger

	eventsReceived metric.Int64Counter
	eventsInserted metric.Int64Counter
}

func NewServer(pool *pgxpool.Pool, store *purchase.Store, generator EventFetcher, maxBatch int, logger *slog.Logger) *Server {
	// Instruments of the global meter are no-ops when telemetry is disabled, so the errors can be ignored
	meter := otel.Meter("github.com/brunohms/cat-n-rat/app/backend/internal/api")
	eventsReceived, _ := meter.Int64Counter("catnrat.events.received",
		metric.WithDescription("Purchase events received, by source (push or pull)"), metric.WithUnit("{event}"))
	eventsInserted, _ := meter.Int64Counter("catnrat.events.inserted",
		metric.WithDescription("Purchase events stored for the first time, by source (push or pull)"), metric.WithUnit("{event}"))

	return &Server{
		pool:           pool,
		store:          store,
		generator:      generator,
		maxBatch:       maxBatch,
		logger:         logger,
		eventsReceived: eventsReceived,
		eventsInserted: eventsInserted,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /events", s.receiveEvents)
	mux.HandleFunc("POST /events/fetch", s.fetchEvents)
	mux.HandleFunc("GET /purchases", s.listPurchases)
	mux.HandleFunc("GET /purchases/{event_id}", s.getPurchase)
	mux.HandleFunc("GET /customers", s.listCustomers)
	mux.HandleFunc("GET /filters", s.filterOptions)
	return otelhttp.NewHandler(s.logRequests(mux), "backend",
		// Probes hit /health every few seconds and would drown the useful traces
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/health" }),
		// r.Pattern is the matched route, e.g. "GET /purchases/{event_id}", set by the mux after routing
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			if r.Pattern != "" {
				return r.Pattern
			}
			return r.Method
		}),
	)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// receiveEvents receives purchase events pushed by the generator (SINK=http).
func (s *Server) receiveEvents(w http.ResponseWriter, r *http.Request) {
	var events []purchase.Event
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&events); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body: "+err.Error())
		return
	}
	if err := s.checkCount(len(events)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateEvents(events); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	s.saveEvents(w, r, events, purchase.SourcePush)
}

// fetchEvents requests new events from the generator and stores them.
func (s *Server) fetchEvents(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Count int `json:"count"`
	}{Count: 10}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&req); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid JSON body: "+err.Error())
			return
		}
	}
	if req.Count < 1 {
		writeError(w, http.StatusUnprocessableEntity, "count must be at least 1")
		return
	}
	if err := s.checkCount(req.Count); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	events, err := s.generator.FetchEvents(r.Context(), req.Count)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Failed to fetch events from the generator: "+err.Error())
		return
	}
	if err := validateEvents(events); err != nil {
		writeError(w, http.StatusBadGateway, "Generator returned invalid events: "+err.Error())
		return
	}
	s.saveEvents(w, r, events, purchase.SourcePull)
}

func (s *Server) saveEvents(w http.ResponseWriter, r *http.Request, events []purchase.Event, source string) {
	result, err := s.store.Save(r.Context(), events, source)
	if err != nil {
		s.logger.Error("failed to store events", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to store events")
		return
	}
	sourceAttr := metric.WithAttributes(attribute.String("source", source))
	s.eventsReceived.Add(r.Context(), int64(result.Received), sourceAttr)
	s.eventsInserted.Add(r.Context(), int64(result.Inserted), sourceAttr)
	s.logger.Info("stored events", "source", source, "received", result.Received, "inserted", result.Inserted)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) getPurchase(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(r.PathValue("event_id"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "event_id must be a valid UUID")
		return
	}
	p, err := s.store.Get(r.Context(), eventID)
	if errors.Is(err, purchase.ErrNotFound) {
		writeError(w, http.StatusNotFound, "purchase not found")
		return
	}
	if err != nil {
		s.logger.Error("failed to get purchase", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get purchase")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) checkCount(count int) error {
	if count > s.maxBatch {
		return fmt.Errorf("maximum count is %d", s.maxBatch)
	}
	return nil
}

func validateEvents(events []purchase.Event) error {
	for i, e := range events {
		if err := e.Validate(); err != nil {
			return fmt.Errorf("event %d: %w", i, err)
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError keeps the {"detail": "..."} error format used by the generator.
func writeError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		attrs := []any{"method", r.Method, "path", r.URL.Path, "status", rec.status, "remote", r.RemoteAddr}
		// Lets the logs stored in ClickHouse be joined with their traces
		if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
			attrs = append(attrs, "trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
		}
		s.logger.Info("request", attrs...)
	})
}
