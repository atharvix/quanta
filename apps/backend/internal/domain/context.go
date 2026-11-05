package domain

import "context"

type contextKey string

const (
	contextKeyUserID contextKey = "quanta.user_id"
	contextKeyEmail  contextKey = "quanta.user_email"
	contextKeyRole   contextKey = "quanta.user_role"
)

// WithUserContext stores authenticated user information in the request context.
func WithUserContext(ctx context.Context, userID, email string, role Role) context.Context {
	ctx = context.WithValue(ctx, contextKeyUserID, userID)
	ctx = context.WithValue(ctx, contextKeyEmail, email)
	ctx = context.WithValue(ctx, contextKeyRole, role)
	return ctx
}

// UserIDFromContext retrieves the authenticated user ID from context, or empty string.
func UserIDFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(contextKeyUserID).(string); ok {
		return val
	}
	return ""
}

// EmailFromContext retrieves the authenticated user email from context, or empty string.
func EmailFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(contextKeyEmail).(string); ok {
		return val
	}
	return ""
}

// RoleFromContext retrieves the authenticated user role from context, or empty Role.
func RoleFromContext(ctx context.Context) Role {
	if val, ok := ctx.Value(contextKeyRole).(Role); ok {
		return val
	}
	return ""
}

// ActorFromContext returns the authenticated email, or "anonymous" if unauthenticated.
func ActorFromContext(ctx context.Context) string {
	email := EmailFromContext(ctx)
	if email != "" {
		return email
	}
	return "anonymous"
}
