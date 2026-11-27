package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/intelligence"
	"github.com/atharvix/quanta/apps/backend/internal/middleware"
)

// ErrorDetail represents the canonical structured error body defined in API_DESIGN.md.
type ErrorDetail struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	CorrelationID string `json:"correlation_id"`
	Details       any    `json:"details,omitempty"`
}

// ErrorEnvelope wraps ErrorDetail under the top-level "error" field.
type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// WriteJSON serializes payload as UTF-8 JSON with the given HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// WriteError writes the canonical structured JSON error envelope.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details ...any) {
	var detailPayload any
	if len(details) > 0 {
		detailPayload = details[0]
	}
	WriteJSON(w, status, ErrorEnvelope{
		Error: ErrorDetail{
			Code:          code,
			Message:       message,
			CorrelationID: middleware.CorrelationIDFromContext(r.Context()),
			Details:       detailPayload,
		},
	})
}

// DecodeJSON decodes a single JSON request body into dst, rejecting unknown fields or trailing garbage.
func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

// WriteDomainError maps domain sentinel errors to deterministic HTTP status codes and structured error codes.
func WriteDomainError(w http.ResponseWriter, r *http.Request, err error, details ...any) {
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrInactiveUser):
		WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error(), details...)
	case errors.Is(err, domain.ErrForbidden), errors.Is(err, domain.ErrCrossTenantAccess):
		WriteError(w, r, http.StatusForbidden, "FORBIDDEN", err.Error(), details...)
	case errors.Is(err, domain.ErrMakerCheckerViolation):
		WriteError(w, r, http.StatusForbidden, "MAKER_CHECKER_VIOLATION", err.Error(), details...)
	case errors.Is(err, domain.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "NOT_FOUND", err.Error(), details...)
	case errors.Is(err, domain.ErrVersionConflict):
		WriteError(w, r, http.StatusConflict, "VERSION_CONFLICT", err.Error(), details...)
	case errors.Is(err, domain.ErrCurrencyMismatch):
		WriteError(w, r, http.StatusBadRequest, "CURRENCY_MISMATCH", err.Error(), details...)
	case errors.Is(err, domain.ErrInvalidAmount), errors.Is(err, domain.ErrInvalidConfiguration):
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error(), details...)
	case errors.Is(err, domain.ErrInsufficientAvailableBalance):
		WriteError(w, r, http.StatusUnprocessableEntity, "INSUFFICIENT_AVAILABLE_BALANCE", err.Error(), details...)
	case errors.Is(err, domain.ErrInsufficientEncumbered):
		WriteError(w, r, http.StatusUnprocessableEntity, "INSUFFICIENT_ENCUMBERED_BALANCE", err.Error(), details...)
	case errors.Is(err, domain.ErrAlreadyReversed), errors.Is(err, domain.ErrCannotReverseEntryType):
		WriteError(w, r, http.StatusUnprocessableEntity, "REVERSAL_REJECTED", err.Error(), details...)
	case errors.Is(err, domain.ErrPolicyViolation):
		WriteError(w, r, http.StatusUnprocessableEntity, "LIQUIDITY_POLICY_VIOLATION", err.Error(), details...)
	case errors.Is(err, domain.ErrCounterpartyLimitExceeded):
		WriteError(w, r, http.StatusUnprocessableEntity, "COUNTERPARTY_LIMIT_EXCEEDED", err.Error(), details...)
	case errors.Is(err, domain.ErrReconciliationMismatch):
		WriteError(w, r, http.StatusConflict, "RECONCILIATION_MISMATCH", err.Error(), details...)
	case errors.Is(err, domain.ErrProposalNotCompletedRun), errors.Is(err, domain.ErrInvalidProposalTransition):
		WriteError(w, r, http.StatusUnprocessableEntity, "INVALID_PROPOSAL_STATE", err.Error(), details...)
	case errors.Is(err, domain.ErrProposalExpired):
		WriteError(w, r, http.StatusUnprocessableEntity, "PROPOSAL_EXPIRED", err.Error(), details...)
	case errors.Is(err, domain.ErrStaleProposalState):
		WriteError(w, r, http.StatusConflict, "STALE_PROPOSAL_STATE", err.Error(), details...)
	case errors.Is(err, intelligence.ErrUpstreamUnavailable), errors.Is(err, intelligence.ErrUpstreamTimeout):
		WriteError(w, r, http.StatusServiceUnavailable, "UPSTREAM_UNAVAILABLE", err.Error(), details...)
	case errors.Is(err, intelligence.ErrMalformedResponse):
		WriteError(w, r, http.StatusBadGateway, "UPSTREAM_MALFORMED_RESPONSE", err.Error(), details...)
	default:
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
