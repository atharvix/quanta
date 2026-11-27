package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCorrelationID_PreservesIncomingHeader(t *testing.T) {
	var seenID string
	handler := CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenID = CorrelationIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(CorrelationIDHeader, "req-test-1234")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if seenID != "req-test-1234" {
		t.Fatalf("expected context correlation id req-test-1234, got %q", seenID)
	}
	if got := rec.Header().Get(CorrelationIDHeader); got != "req-test-1234" {
		t.Fatalf("expected response header req-test-1234, got %q", got)
	}
}

func TestCorrelationID_GeneratesWhenMissing(t *testing.T) {
	var seenID string
	handler := CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenID = CorrelationIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if seenID == "" {
		t.Fatal("expected generated correlation id in context")
	}
	if got := rec.Header().Get(CorrelationIDHeader); got != seenID {
		t.Fatalf("expected response header %q to match context %q", got, seenID)
	}
}
