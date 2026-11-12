package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddleware(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000", "https://quanta.finance"}
	corsMW := CORS(allowedOrigins)

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	handler := corsMW(dummyHandler)

	t.Run("Allowed origin sets correct CORS headers without wildcard", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rec.Code)
		}

		originHeader := rec.Header().Get("Access-Control-Allow-Origin")
		if originHeader != "http://localhost:3000" {
			t.Errorf("expected 'http://localhost:3000', got '%s'", originHeader)
		}
		if originHeader == "*" {
			t.Errorf("wildcard '*' must never be used for Access-Control-Allow-Origin")
		}

		methodsHeader := rec.Header().Get("Access-Control-Allow-Methods")
		if methodsHeader != "GET, POST, PATCH, OPTIONS" {
			t.Errorf("expected 'GET, POST, PATCH, OPTIONS', got '%s'", methodsHeader)
		}

		headersHeader := rec.Header().Get("Access-Control-Allow-Headers")
		if headersHeader != "Authorization, Content-Type" {
			t.Errorf("expected 'Authorization, Content-Type', got '%s'", headersHeader)
		}
	})

	t.Run("Disallowed origin does NOT set Access-Control-Allow-Origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/treasuries", nil)
		req.Header.Set("Origin", "http://malicious-attacker.com")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		originHeader := rec.Header().Get("Access-Control-Allow-Origin")
		if originHeader != "" {
			t.Errorf("expected empty Access-Control-Allow-Origin for untrusted origin, got '%s'", originHeader)
		}
	})

	t.Run("Preflight OPTIONS request returns 204 No Content", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/treasuries", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204 No Content for OPTIONS preflight, got %d", rec.Code)
		}

		originHeader := rec.Header().Get("Access-Control-Allow-Origin")
		if originHeader != "http://localhost:3000" {
			t.Errorf("expected 'http://localhost:3000' in preflight response, got '%s'", originHeader)
		}
	})
}
