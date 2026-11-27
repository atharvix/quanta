package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/atharvix/quanta/apps/backend/internal/auth"
	"github.com/atharvix/quanta/apps/backend/internal/config"
	"github.com/atharvix/quanta/apps/backend/internal/handlers"
	"github.com/atharvix/quanta/apps/backend/internal/middleware"
	"github.com/atharvix/quanta/apps/backend/internal/repository"
)

// Dependencies bundles all handlers and security dependencies needed by NewRouter.
type Dependencies struct {
	Health       *handlers.HealthHandler
	Auth         *handlers.AuthHandler
	Treasury     *handlers.TreasuryHandler
	Policy       *handlers.PolicyHandler
	Counterparty *handlers.CounterpartyHandler
	Intelligence *handlers.IntelligenceHandler
	Proposal     *handlers.ProposalHandler
	Audit        *handlers.AuditHandler
	Tokens       *auth.TokenService
	OrgRepo      repository.OrganizationRepository
}

// NewRouter builds the HTTP router with correlation ID, recovery, auth, and org-scoped RBAC routes.
func NewRouter(_ *config.Config, deps Dependencies) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.CorrelationID)
	r.Use(chimiddleware.Recoverer)

	r.Get("/health", deps.Health.Liveness)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", deps.Health.Readiness)

		if deps.Auth != nil {
			r.Post("/auth/login", deps.Auth.Login)
			if deps.Tokens != nil {
				r.With(middleware.RequireAuth(deps.Tokens)).Get("/auth/me", deps.Auth.Me)
			}
		}

		if deps.Tokens != nil && deps.OrgRepo != nil {
			r.Route("/orgs/{orgId}", func(r chi.Router) {
				r.Use(middleware.RequireAuth(deps.Tokens))

				if deps.Treasury != nil {
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermTreasuryRead)).
						Get("/treasuries", deps.Treasury.ListEntities)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermTreasuryRead)).
						Get("/treasuries/{entityId}/accounts", deps.Treasury.ListAccounts)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermTreasuryRead)).
						Get("/accounts/{accountId}", deps.Treasury.GetAccount)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermTreasuryRead)).
						Get("/accounts/{accountId}/ledger", deps.Treasury.ListLedgerEntries)

					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermLedgerMutate)).
						Post("/accounts/{accountId}/credit", deps.Treasury.Credit)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermLedgerMutate)).
						Post("/accounts/{accountId}/debit", deps.Treasury.Debit)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermLedgerMutate)).
						Post("/accounts/{accountId}/holds", deps.Treasury.PlaceHold)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermLedgerMutate)).
						Post("/accounts/{accountId}/holds/release", deps.Treasury.ReleaseHold)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermLedgerMutate)).
						Post("/accounts/{accountId}/holds/settle", deps.Treasury.SettleHold)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermLedgerMutate)).
						Post("/ledger-entries/{entryId}/reverse", deps.Treasury.ReverseEntry)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermLedgerMutate)).
						Post("/accounts/{accountId}/reconcile", deps.Treasury.ReconcileAccount)
				}

				if deps.Policy != nil {
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermPolicyRead)).
						Get("/treasuries/{entityId}/liquidity-policies", deps.Policy.ListPolicies)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermPolicyWrite)).
						Post("/treasuries/{entityId}/liquidity-policies", deps.Policy.CreatePolicy)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermPolicyWrite)).
						Put("/treasuries/{entityId}/liquidity-policies/{policyId}", deps.Policy.UpdatePolicy)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermPolicyRead)).
						Post("/treasuries/{entityId}/liquidity-policies/validate", deps.Policy.ValidateAllocation)
				}

				if deps.Counterparty != nil {
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermCounterpartyRead)).
						Get("/treasuries/{entityId}/counterparties", deps.Counterparty.ListCounterparties)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermCounterpartyWrite)).
						Post("/treasuries/{entityId}/counterparties", deps.Counterparty.CreateCounterparty)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermCounterpartyWrite)).
						Put("/treasuries/{entityId}/counterparties/{counterpartyId}", deps.Counterparty.UpdateCounterparty)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermCounterpartyRead)).
						Post("/treasuries/{entityId}/counterparties/{counterpartyId}/validate-exposure", deps.Counterparty.ValidateExposure)
				}

				if deps.Audit != nil {
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermAuditRead)).
						Get("/audit-events", deps.Audit.ListAuditEvents)
				}

				if deps.Intelligence != nil {
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermProposalSubmit)).
						Post("/treasuries/{entityId}/forecast", deps.Intelligence.Forecast)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermProposalSubmit)).
						Post("/treasuries/{entityId}/risk/evaluate", deps.Intelligence.EvaluateRisk)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermProposalSubmit)).
						Post("/treasuries/{entityId}/optimize", deps.Intelligence.Optimize)
				}

				if deps.Proposal != nil {
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermTreasuryRead)).
						Get("/treasuries/{entityId}/proposals", deps.Proposal.ListProposals)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermProposalSubmit)).
						Post("/proposals", deps.Proposal.CreateProposal)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermProposalSubmit)).
						Post("/proposals/{proposalId}/submit", deps.Proposal.SubmitProposal)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermProposalApprove)).
						Post("/proposals/{proposalId}/approve", deps.Proposal.ApproveProposal)
					r.With(middleware.RequireOrgPermission(deps.OrgRepo, auth.PermProposalApprove)).
						Post("/proposals/{proposalId}/reject", deps.Proposal.RejectProposal)
				}
			})
		}
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		handlers.WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "route not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		handlers.WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})

	return r
}
