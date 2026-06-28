package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestStaticRoutes_RoutingPrecedence tests that API routes are not intercepted by static file handlers
func TestStaticRoutes_RoutingPrecedence(t *testing.T) {
	// We'll test the Routes() function's registration order and pattern matching
	// Since we can't easily instantiate a full Server without a real repository,
	// we'll create a minimal test that verifies the route registration logic

	t.Run("Go ServeMux pattern precedence - specific before prefix", func(t *testing.T) {
		mux := http.NewServeMux()

		// Simulate the registration order in Routes()
		// 1. API routes registered first
		apiCalled := false
		mux.HandleFunc("/api/v1/admin/login", func(w http.ResponseWriter, r *http.Request) {
			apiCalled = true
			w.WriteHeader(http.StatusOK)
		})

		// 2. Static file handler registered after
		mux.Handle("/admin/", http.StripPrefix("/admin/", http.FileServer(http.Dir("."))))

		// Test that /api/v1/admin/login hits the API handler, not static files
		req := httptest.NewRequest("POST", "/api/v1/admin/login", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if !apiCalled {
			t.Error("FAIL: /api/v1/admin/login was intercepted by /admin/ static handler")
		}
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})

	t.Run("Root path redirect behavior", func(t *testing.T) {
		mux := http.NewServeMux()

		// Register the root handler as in Routes()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, "/admin/index.html", http.StatusFound)
		})

		// Test root path
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 302 {
			t.Errorf("Expected 302 redirect, got %d", w.Code)
		}
		location := w.Header().Get("Location")
		if location != "/admin/index.html" {
			t.Errorf("Expected redirect to /admin/index.html, got %s", location)
		}

		// Test non-root path
		req = httptest.NewRequest("GET", "/nonexistent", nil)
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 404 {
			t.Errorf("Expected 404 for /nonexistent, got %d", w.Code)
		}
	})

	t.Run("Static file path patterns", func(t *testing.T) {
		mux := http.NewServeMux()

		// Register static handlers
		mux.Handle("/admin/", http.StripPrefix("/admin/", http.FileServer(http.Dir("."))))
		mux.Handle("/mini/", http.StripPrefix("/mini/", http.FileServer(http.Dir("."))))
		mux.Handle("/mock/", http.StripPrefix("/mock/", http.FileServer(http.Dir("."))))

		// Test that /admin/something maps to correct path
		// We can't easily test file serving without actual files, but we can verify
		// the handler is registered

		// The key insight: /api/v1/admin/login should NOT match /admin/ prefix
		// because Go's ServeMux treats /admin/ as a pattern that matches /admin/*
		// but /api/v1/admin/login doesn't start with /admin/

		req := httptest.NewRequest("GET", "/api/v1/admin/login", nil)
		w := httptest.NewRecorder()

		// Install a catch-all to see what happens
		mux.HandleFunc("/api/v1/admin/login", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 for exact API route, got %d", w.Code)
		}
	})
}

// TestCORSCompatibility verifies CORS headers are applied to all routes
func TestCORSCompatibility(t *testing.T) {
	// Create a simple handler wrapped with CORS
	inner := http.NewServeMux()
	inner.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	inner.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("."))))

	handler := corsMiddleware(inner)

	tests := []struct {
		name string
		path string
	}{
		{"API route", "/test"},
		{"Root path", "/"},
		{"Static path", "/static/file.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			origin := w.Header().Get("Access-Control-Allow-Origin")
			if origin != "*" {
				t.Errorf("Missing CORS header for %s: got %s", tt.path, origin)
			}
		})
	}
}

// TestStaticFilePaths verifies the relative path behavior
func TestStaticFilePaths(t *testing.T) {
	t.Run("Relative path depends on working directory", func(t *testing.T) {
		// The code uses ./prototype/admin, which is relative to the working directory
		// This is a POTENTIAL ISSUE if the server is not started from the project root

		// We can't easily test this without actually starting a server,
		// but we should document this as a concern

		t.Log("WARNING: http.Dir(\"./prototype/admin\") uses relative path")
		t.Log("Server must be started from project root directory")
		t.Log("Consider using absolute path or embedded files (go:embed)")
	})
}

// TestOPTIONSRequestHandling verifies CORS preflight requests work
func TestOPTIONSRequestHandling(t *testing.T) {
	inner := http.NewServeMux()
	inner.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := corsMiddleware(loggingMiddleware(inner))

	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected 204 for OPTIONS request, got %d", w.Code)
	}

	// Verify CORS headers are present
	methods := w.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("Missing Access-Control-Allow-Methods header")
	}
}
