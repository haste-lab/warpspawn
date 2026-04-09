package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/haste-lab/warpspawn/internal/config"
	"github.com/haste-lab/warpspawn/internal/db"
)

// Server is the HTTP server for the Warpspawn UI and API.
type Server struct {
	token    string
	port     int
	db       *db.DB
	cfg      config.Config
	mux      *http.ServeMux
	sseClients map[chan SSEEvent]bool
	sseMu      sync.RWMutex
}

// SSEEvent is an event sent to the frontend via Server-Sent Events.
type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// New creates a new server instance.
func New(port int, token string, database *db.DB, cfg config.Config) *Server {
	s := &Server{
		token:      token,
		port:       port,
		db:         database,
		cfg:        cfg,
		mux:        http.NewServeMux(),
		sseClients: make(map[chan SSEEvent]bool),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// API routes (authenticated)
	s.mux.HandleFunc("GET /api/health", s.auth(s.handleHealth))
	s.mux.HandleFunc("GET /api/projects", s.auth(s.handleListProjects))
	s.mux.HandleFunc("GET /api/budget", s.auth(s.handleBudget))
	s.mux.HandleFunc("GET /api/events", s.auth(s.handleSSE))

	// Static frontend (no auth — token is in the URL query param, checked on API calls)
	// TODO: serve embedded frontend assets
	s.mux.HandleFunc("GET /", s.handleFrontend)
}

// auth wraps a handler with session token authentication.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "Bearer "+s.token {
			next(w, r)
			return
		}

		// Check query parameter (for SSE EventSource which can't set headers)
		if r.URL.Query().Get("token") == s.token {
			next(w, r)
			return
		}

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": "dev",
	})
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.db.GetProjectSummaries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

func (s *Server) handleBudget(w http.ResponseWriter, r *http.Request) {
	cost, err := s.db.GetDailyCost()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"daily_cost_usd":  cost,
		"daily_limit_usd": s.cfg.Budget.DailyLimitUSD,
		"date":            time.Now().Format("2006-01-02"),
	})
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := make(chan SSEEvent, 32)
	s.sseMu.Lock()
	s.sseClients[ch] = true
	s.sseMu.Unlock()

	defer func() {
		s.sseMu.Lock()
		delete(s.sseClients, ch)
		s.sseMu.Unlock()
		close(ch)
	}()

	// Send initial connected event
	fmt.Fprintf(w, "data: {\"type\":\"connected\"}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// Broadcast sends an event to all connected SSE clients.
func (s *Server) Broadcast(event SSEEvent) {
	s.sseMu.RLock()
	defer s.sseMu.RUnlock()
	for ch := range s.sseClients {
		select {
		case ch <- event:
		default:
			// Client channel full — skip to avoid blocking
		}
	}
}

func (s *Server) handleFrontend(w http.ResponseWriter, r *http.Request) {
	// Placeholder until we embed the Svelte build
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Warpspawn</title></head>
<body style="background:#0a0a0a;color:#eee;font-family:monospace;padding:40px;">
<h1>Warpspawn</h1>
<p>Autonomous agentic software delivery</p>
<p>Frontend not yet built. API is active.</p>
<p>Try: <a href="/api/health?token=%s" style="color:#72e6b8;">/api/health</a></p>
</body>
</html>`, s.token)
}

// securityHeaders adds CSP, CORS, and other security headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// Start starts the HTTP server. Returns the actual port (if auto-selected) and a shutdown function.
func (s *Server) Start(ctx context.Context) (int, func(), error) {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// Try to find a free port
		for attempt := s.port + 1; attempt < s.port+100; attempt++ {
			addr = fmt.Sprintf("127.0.0.1:%d", attempt)
			listener, err = net.Listen("tcp", addr)
			if err == nil {
				s.port = attempt
				break
			}
		}
		if err != nil {
			return 0, nil, fmt.Errorf("no available port found near %d: %w", s.port, err)
		}
	}

	actualPort := listener.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: securityHeaders(s.mux)}

	go func() {
		slog.Info("server started", "addr", addr)
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	shutdown := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}

	return actualPort, shutdown, nil
}
