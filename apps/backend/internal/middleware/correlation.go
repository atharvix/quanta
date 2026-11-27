package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// CorrelationIDHeader is the HTTP header used for request correlation propagation.
const CorrelationIDHeader = "X-Correlation-ID"

type contextKey string

const correlationIDContextKey contextKey = "correlation_id"

// CorrelationID extracts or generates a correlation ID, stores it on the request context,
// and echoes it in the X-Correlation-ID response header.
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := strings.TrimSpace(r.Header.Get(CorrelationIDHeader))
		if correlationID == "" {
			correlationID = uuid.NewString()
		}

		ctx := context.WithValue(r.Context(), correlationIDContextKey, correlationID)
		w.Header().Set(CorrelationIDHeader, correlationID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CorrelationIDFromContext returns the correlation ID attached to ctx, or an empty string.
func CorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(correlationIDContextKey).(string); ok {
		return id
	}
	return ""
}
