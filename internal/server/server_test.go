package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/haste-lab/warpspawn/internal/config"
	"github.com/haste-lab/warpspawn/internal/db"
	"github.com/haste-lab/warpspawn/internal/provider"
)

func setupTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	cfg := config.DefaultConfig()
	token := "test-token-12345678901234567890123456789012"
	providers := map[string]provider.Provider{}

	srv := New(9999, "127.0.0.1", token, database, cfg, dir, providers)
	return srv, token
}

func TestHealthEndpoint(t *testing.T) {
	srv, token := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
}

func TestHealthEndpoint_Unauthorized(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHealthEndpoint_WrongToken(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHealthEndpoint_CookieAuth(t *testing.T) {
	srv, token := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.AddCookie(&http.Cookie{Name: "ws_session", Value: token})
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestProjectsEndpoint_Empty(t *testing.T) {
	srv, token := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestSettingsEndpoint(t *testing.T) {
	srv, token := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var cfg config.Config
	json.NewDecoder(w.Body).Decode(&cfg)
	if cfg.Execution.ShellMode != "restricted" {
		t.Errorf("shell mode = %q, want restricted", cfg.Execution.ShellMode)
	}
}

func TestUpdateSettings(t *testing.T) {
	srv, token := setupTestServer(t)

	body := `{"config_version":1,"providers":{},"roles":{},"budget":{"daily_limit_usd":25},"execution":{"max_tool_calls":50,"agent_timeout_s":300,"shell_mode":"restricted","llm_context_size":32768}}`
	req := httptest.NewRequest("PUT", "/api/settings", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var cfg config.Config
	json.NewDecoder(w.Body).Decode(&cfg)
	if cfg.Budget.DailyLimitUSD != 25 {
		t.Errorf("budget = %f, want 25", cfg.Budget.DailyLimitUSD)
	}
}

func TestUpdateSettings_Validation(t *testing.T) {
	srv, token := setupTestServer(t)

	// Try to set dangerous values
	body := `{"config_version":1,"providers":{},"roles":{},"budget":{"daily_limit_usd":99999},"execution":{"max_tool_calls":99999,"agent_timeout_s":0,"shell_mode":"dangerous","llm_context_size":100}}`
	req := httptest.NewRequest("PUT", "/api/settings", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var cfg config.Config
	json.NewDecoder(w.Body).Decode(&cfg)
	// Should be clamped to safe bounds
	if cfg.Execution.MaxToolCalls != 200 {
		t.Errorf("max tools = %d, want 200 (clamped)", cfg.Execution.MaxToolCalls)
	}
	if cfg.Execution.ShellMode != "restricted" {
		t.Errorf("shell mode = %q, want restricted (invalid reset)", cfg.Execution.ShellMode)
	}
	if cfg.Budget.DailyLimitUSD != 1000 {
		t.Errorf("budget = %f, want 1000 (clamped)", cfg.Budget.DailyLimitUSD)
	}
}

func TestBudgetEndpoint(t *testing.T) {
	srv, token := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/budget", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestAuthExchange(t *testing.T) {
	srv, token := setupTestServer(t)

	req := httptest.NewRequest("GET", "/auth?token="+token, nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 302 {
		t.Errorf("status = %d, want 302 redirect", w.Code)
	}

	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "ws_session" && c.Value == token {
			found = true
			if !c.HttpOnly {
				t.Error("cookie should be HttpOnly")
			}
			if c.SameSite != http.SameSiteStrictMode {
				t.Error("cookie should be SameSite=Strict")
			}
		}
	}
	if !found {
		t.Error("expected ws_session cookie")
	}
}

func TestAuthExchange_BadToken(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/auth?token=wrong", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestModelsEndpoint_NoProviders(t *testing.T) {
	srv, token := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/models", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestCSPHeaders(t *testing.T) {
	srv, token := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	// Use securityHeaders wrapper to test middleware
	securityHeaders(srv.mux).ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("missing CSP header: %q", csp)
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options: DENY")
	}
}
