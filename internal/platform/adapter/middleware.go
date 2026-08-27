package adapter

import (
	"context"
	"crypto/subtle"
	"fmt"
	platform "github.com/enterprise-labs/seismic-event-associator/internal/platform/application"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type ctxKey struct{}

func RequestID(ctx context.Context) string { v, _ := ctx.Value(ctxKey{}).(string); return v }

type recorder struct {
	http.ResponseWriter
	status int
}

func (w *recorder) WriteHeader(s int) { w.status = s; w.ResponseWriter.WriteHeader(s) }

type Middleware struct {
	cfg     platform.Config
	logger  *slog.Logger
	metrics *platform.Metrics
	mu      sync.Mutex
	tokens  map[string]float64
	updated map[string]time.Time
}

func NewMiddleware(cfg platform.Config, l *slog.Logger, m *platform.Metrics) *Middleware {
	return &Middleware{cfg: cfg, logger: l, metrics: m, tokens: map[string]float64{}, updated: map[string]time.Time{}}
}
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	h := m.recover(next)
	h = http.TimeoutHandler(h, m.cfg.Timeout, "request timeout")
	h = m.bodyLimit(h)
	h = m.authenticate(h)
	h = m.rateLimit(h)
	h = m.observe(h)
	return h
}
func (m *Middleware) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				m.logger.Error("panic recovered", "value", fmt.Sprint(v))
				http.Error(w, "internal server error", 500)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func (m *Middleware) bodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, m.cfg.MaxBody)
		next.ServeHTTP(w, r)
	})
}
func (m *Middleware) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" && r.URL.Path != "/metrics" {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-API-Key")), []byte(m.cfg.APIKey)) != 1 {
				http.Error(w, "unauthorized", 401)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (m *Middleware) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, e := net.SplitHostPort(r.RemoteAddr)
		if e != nil {
			host = r.RemoteAddr
		}
		now := time.Now()
		m.mu.Lock()
		last := m.updated[host]
		tokens := m.tokens[host]
		tokens += now.Sub(last).Seconds() * float64(m.cfg.RateLimit)
		if tokens > float64(m.cfg.RateLimit) {
			tokens = float64(m.cfg.RateLimit)
		}
		allow := tokens >= 1
		if allow {
			tokens--
		}
		m.tokens[host], m.updated[host] = tokens, now
		m.mu.Unlock()
		if !allow {
			http.Error(w, "rate limit exceeded", 429)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (m *Middleware) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = "req-" + strconv.FormatInt(time.Now().UnixNano(), 36)
		}
		w.Header().Set("X-Request-ID", id)
		rec := &recorder{ResponseWriter: w, status: 200}
		start := time.Now()
		next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
		m.metrics.Request(rec.status >= 400)
		m.logger.Info("request completed", "path", r.URL.Path, "status", rec.status, "duration", time.Since(start), "request_id", id)
	})
}
