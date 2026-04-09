package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/haste-lab/warpspawn/internal/config"
	"github.com/haste-lab/warpspawn/internal/db"
	"github.com/haste-lab/warpspawn/internal/provider"
)

// Server is the HTTP server for the Warpspawn UI and API.
type Server struct {
	token      string
	port       int
	host       string
	db         *db.DB
	cfg        config.Config
	configDir  string
	providers  map[string]provider.Provider
	mux        *http.ServeMux
	sseClients map[chan SSEEvent]bool
	sseMu      sync.RWMutex
}

// SSEEvent is an event sent to the frontend via Server-Sent Events.
type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// New creates a new server instance.
func New(port int, host, token string, database *db.DB, cfg config.Config, configDir string, providers map[string]provider.Provider) *Server {
	s := &Server{
		token:      token,
		port:       port,
		host:       host,
		db:         database,
		cfg:        cfg,
		configDir:  configDir,
		providers:  providers,
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
	s.mux.HandleFunc("GET /api/project/{id}", s.auth(s.handleProjectDetail))
	s.mux.HandleFunc("GET /api/budget", s.auth(s.handleBudget))
	s.mux.HandleFunc("GET /api/events", s.auth(s.handleSSE))

	// Project actions
	s.mux.HandleFunc("POST /api/project/create", s.auth(s.handleCreateProject))
	s.mux.HandleFunc("DELETE /api/project/{id}", s.auth(s.handleDeleteProject))
	s.mux.HandleFunc("POST /api/project/{id}/chat", s.auth(s.handleProjectChat))
	s.mux.HandleFunc("GET /api/project/{id}/chat", s.auth(s.handleGetChat))
	s.mux.HandleFunc("POST /api/project/{id}/build", s.auth(s.handleStartBuild))

	// Settings and configuration
	s.mux.HandleFunc("GET /api/settings", s.auth(s.handleSettings))
	s.mux.HandleFunc("PUT /api/settings", s.auth(s.handleUpdateSettings))
	s.mux.HandleFunc("GET /api/models", s.auth(s.handleListModels))

	// Token-to-cookie exchange: browser opens URL with ?token=..., gets a cookie, then redirected to clean URL
	s.mux.HandleFunc("GET /auth", s.handleAuth)

	// Serve embedded frontend (no auth on static assets — token checked on API calls)
	s.mux.Handle("/", frontendHandler())
}

// handleAuth exchanges a URL token for an HttpOnly cookie, then redirects to the clean URL.
func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	queryToken := r.URL.Query().Get("token")
	if len(queryToken) != len(s.token) || subtle.ConstantTimeCompare([]byte(queryToken), []byte(s.token)) != 1 {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "ws_session",
		Value:    s.token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		// No Secure flag — localhost doesn't use TLS
	})

	// Redirect to clean URL (strips token from browser history and address bar)
	http.Redirect(w, r, "/", http.StatusFound)
}

// auth wraps a handler with session token authentication.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Check HttpOnly cookie (preferred — set by /auth exchange)
		if cookie, err := r.Cookie("ws_session"); err == nil {
			if len(cookie.Value) == len(s.token) && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.token)) == 1 {
				next(w, r)
				return
			}
		}

		// 2. Check Authorization header (for API calls from frontend JS)
		authHeader := r.Header.Get("Authorization")
		expected := "Bearer " + s.token
		if len(authHeader) == len(expected) && subtle.ConstantTimeCompare([]byte(authHeader), []byte(expected)) == 1 {
			next(w, r)
			return
		}

		// 3. Check query parameter (fallback for SSE EventSource)
		queryToken := r.URL.Query().Get("token")
		if len(queryToken) == len(s.token) && subtle.ConstantTimeCompare([]byte(queryToken), []byte(s.token)) == 1 {
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

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.cfg)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var updated config.Config
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&updated); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	// Preserve version and validate bounds
	updated.ConfigVersion = s.cfg.ConfigVersion
	updated = config.ValidateConfig(updated)
	if err := config.Save(s.configDir, updated); err != nil {
		http.Error(w, "failed to save: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.cfg = updated
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.cfg)
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	type modelEntry struct {
		Provider string `json:"provider"`
		ID       string `json:"id"`
		Name     string `json:"name"`
	}

	var allModels []modelEntry

	for name, prov := range s.providers {
		models, err := prov.ListModels(r.Context())
		if err != nil {
			slog.Debug("failed to list models for provider", "provider", name, "error", err)
			continue
		}
		for _, m := range models {
			allModels = append(allModels, modelEntry{
				Provider: name,
				ID:       m.ID,
				Name:     m.Name,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allModels)
}

func (s *Server) handleProjectDetail(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		http.Error(w, "project ID required", http.StatusBadRequest)
		return
	}

	detail, err := s.db.GetProjectDetail(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
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
	bindHost := s.host
	if bindHost == "" {
		bindHost = "127.0.0.1"
	}
	addr := fmt.Sprintf("%s:%d", bindHost, s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// Try to find a free port
		for attempt := s.port + 1; attempt < s.port+100; attempt++ {
			addr = fmt.Sprintf("%s:%d", bindHost, attempt)
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
