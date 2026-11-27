package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/middleware"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

type ProposalHandler struct {
	proposalSvc *service.ProposalService
}

func NewProposalHandler(proposalSvc *service.ProposalService) *ProposalHandler {
	return &ProposalHandler{proposalSvc: proposalSvc}
}

type createProposalHTTPRequest struct {
	OptimizationRunID    string  `json:"optimization_run_id"`
	SupersedesProposalID *string `json:"supersedes_proposal_id,omitempty"`
}

type submitProposalHTTPRequest struct {
	ExpiresInMinutes int `json:"expires_in_minutes"`
}

type decideProposalHTTPRequest struct {
	Rationale string `json:"rationale"`
}

func (h *ProposalHandler) CreateProposal(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	var req createProposalHTTPRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}
	runID, err := uuid.Parse(req.OptimizationRunID)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid optimization_run_id")
		return
	}
	var supersedesID *uuid.UUID
	if req.SupersedesProposalID != nil && *req.SupersedesProposalID != "" {
		parsed, err := uuid.Parse(*req.SupersedesProposalID)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid supersedes_proposal_id")
			return
		}
		supersedesID = &parsed
	}

	proposal, err := h.proposalSvc.CreateProposal(r.Context(), service.CreateProposalInput{
		OrganizationID:       orgCtx.OrganizationID,
		OptimizationRunID:    runID,
		SupersedesProposalID: supersedesID,
		ActorID:              userCtx.UserID,
		ActorRole:            string(orgCtx.Role),
		CorrelationID:        middleware.CorrelationIDFromContext(r.Context()),
	})
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{"proposal": serializeProposal(proposal)})
}

func (h *ProposalHandler) ListProposals(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	entityID, err := uuid.Parse(chi.URLParam(r, "entityId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid treasury entity ID")
		return
	}
	items, err := h.proposalSvc.ListProposals(r.Context(), orgCtx.OrganizationID, entityID)
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, p := range items {
		out = append(out, serializeProposal(p))
	}
	WriteJSON(w, http.StatusOK, map[string]any{"proposals": out})
}

func (h *ProposalHandler) SubmitProposal(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	proposalID, err := uuid.Parse(chi.URLParam(r, "proposalId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid proposal ID")
		return
	}
	var req submitProposalHTTPRequest
	if r.ContentLength > 0 {
		if err := DecodeJSON(r, &req); err != nil {
			WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
			return
		}
	}
	ttl := 24 * time.Hour
	if req.ExpiresInMinutes > 0 {
		ttl = time.Duration(req.ExpiresInMinutes) * time.Minute
	}

	proposal, err := h.proposalSvc.SubmitProposal(r.Context(), service.SubmitProposalInput{
		OrganizationID: orgCtx.OrganizationID,
		ProposalID:     proposalID,
		ActorID:        userCtx.UserID,
		ActorRole:      string(orgCtx.Role),
		ExpiresIn:      ttl,
		CorrelationID:  middleware.CorrelationIDFromContext(r.Context()),
	})
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"proposal": serializeProposal(proposal)})
}

func (h *ProposalHandler) ApproveProposal(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	proposalID, err := uuid.Parse(chi.URLParam(r, "proposalId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid proposal ID")
		return
	}
	var req decideProposalHTTPRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}

	proposal, decision, err := h.proposalSvc.ApproveProposal(r.Context(), service.DecideProposalInput{
		OrganizationID: orgCtx.OrganizationID,
		ProposalID:     proposalID,
		ApproverID:     userCtx.UserID,
		ApproverRole:   string(orgCtx.Role),
		Rationale:      req.Rationale,
		CorrelationID:  middleware.CorrelationIDFromContext(r.Context()),
	})
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"proposal": serializeProposal(proposal),
		"decision": decision,
	})
}

func (h *ProposalHandler) RejectProposal(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	proposalID, err := uuid.Parse(chi.URLParam(r, "proposalId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid proposal ID")
		return
	}
	var req decideProposalHTTPRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}

	proposal, decision, err := h.proposalSvc.RejectProposal(r.Context(), service.DecideProposalInput{
		OrganizationID: orgCtx.OrganizationID,
		ProposalID:     proposalID,
		ApproverID:     userCtx.UserID,
		ApproverRole:   string(orgCtx.Role),
		Rationale:      req.Rationale,
		CorrelationID:  middleware.CorrelationIDFromContext(r.Context()),
	})
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"proposal": serializeProposal(proposal),
		"decision": decision,
	})
}

func serializeProposal(p domain.AllocationProposal) map[string]any {
	return map[string]any{
		"id":                        p.ID,
		"organization_id":           p.OrganizationID,
		"optimization_run_id":       p.OptimizationRunID,
		"treasury_entity_id":        p.TreasuryEntityID,
		"supersedes_proposal_id":    p.SupersedesProposalID,
		"status":                    p.Status,
		"investable_capital":        p.InvestableCapital.String(),
		"total_allocated":           p.TotalAllocated.String(),
		"residual_cash":             p.ResidualCash.String(),
		"verified_allocations":      p.VerifiedAllocations,
		"snapshot_account_versions": p.SnapshotAccountVersions,
		"snapshot_policy_version":   p.SnapshotPolicyVersion,
		"submitted_by":              p.SubmittedBy,
		"submitted_at":              p.SubmittedAt,
		"expires_at":                p.ExpiresAt,
		"version":                   p.Version,
		"created_at":                p.CreatedAt.UTC().Format(time.RFC3339),
	}
}
