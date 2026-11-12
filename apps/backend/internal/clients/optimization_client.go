// Package clients provides HTTP client wrappers for external AI and Optimization engines.
package clients

import (
	"context"
)

// OptimizationClient defines outbound operations to Member 2's Quantum/Classical Optimization Engine.
// NOTE: services/optimization-engine is currently a specification module without live endpoints.
// Real HTTP proxy integration will be implemented in Step 5 once Member 2 delivers their solver service.
type OptimizationClient interface {
	// TODO: Implement ClassicalMarkowitz(ctx, problem) when Member 2 delivers /api/v1/classical/optimize
	// TODO: Implement QuantumQAOA(ctx, problem) when Member 2 delivers /api/v1/quantum/qaoa
	Health(ctx context.Context) error
}
