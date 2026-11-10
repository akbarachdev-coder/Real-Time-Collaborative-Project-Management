package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"collabpm/config"
	"collabpm/realtime"
)

// TestRouteRegistration builds the full route tree and asserts key routes are
// registered. It mainly guards against gin panics on wildcard conflicts.
func TestRouteRegistration(t *testing.T) {
	cfg := &config.Config{
		AllowedOrigins: []string{"*"},
	}
	hub := realtime.NewHub(cfg.AllowedOrigins, func(_, _ uint) (bool, error) { return true, nil })

	engine := New(cfg, nil, hub)

	want := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/health"},
		{http.MethodPost, "/api/auth/register"},
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPost, "/api/auth/refresh"},
		{http.MethodGet, "/api/ws"},
		{http.MethodGet, "/api/users/me"},
		{http.MethodGet, "/api/users"},
		{http.MethodGet, "/api/projects"},
		{http.MethodPost, "/api/projects"},
		{http.MethodGet, "/api/projects/:id"},
		{http.MethodPut, "/api/projects/:id"},
		{http.MethodDelete, "/api/projects/:id"},
		{http.MethodGet, "/api/projects/:id/members"},
		{http.MethodPost, "/api/projects/:id/members"},
		{http.MethodDelete, "/api/projects/:id/members/:userId"},
		{http.MethodGet, "/api/projects/:id/tasks"},
		{http.MethodPost, "/api/projects/:id/tasks"},
		{http.MethodGet, "/api/tasks/:id"},
		{http.MethodPut, "/api/tasks/:id"},
		{http.MethodDelete, "/api/tasks/:id"},
		{http.MethodGet, "/api/tasks/:id/comments"},
		{http.MethodPost, "/api/tasks/:id/comments"},
		{http.MethodDelete, "/api/comments/:id"},
		{http.MethodGet, "/api/notifications"},
		{http.MethodPatch, "/api/notifications/:id/read"},
		{http.MethodPost, "/api/notifications/read-all"},
	}

	routes := engine.Routes()
	registered := make(map[string]bool, len(routes))
	for _, r := range routes {
		registered[r.Method+" "+r.Path] = true
	}

	for _, w := range want {
		key := w.method + " " + w.path
		if !registered[key] {
			t.Errorf("route not registered: %s", key)
		}
	}
}

// TestHealthEndpoint runs the health handler end-to-end.
func TestHealthEndpoint(t *testing.T) {
	cfg := &config.Config{AllowedOrigins: []string{"*"}}
	hub := realtime.NewHub(cfg.AllowedOrigins, func(_, _ uint) (bool, error) { return true, nil })
	engine := New(cfg, nil, hub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
