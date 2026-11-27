package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/middleware"
	"github.com/atharvix/quanta/apps/backend/internal/money"
	"github.com/atharvix/quanta/apps/backend/internal/service"
)

type TreasuryHandler struct {
	accountSvc *service.AccountService
	ledgerSvc  *service.LedgerService
}

func NewTreasuryHandler(accountSvc *service.AccountService, ledgerSvc *service.LedgerService) *TreasuryHandler {
	return &TreasuryHandler{
		accountSvc: accountSvc,
		ledgerSvc:  ledgerSvc,
	}
}

type ledgerMutationRequest struct {
	Amount        string `json:"amount"`
	ReferenceType string `json:"reference_type"`
	ReferenceID   string `json:"reference_id"`
}

type reverseEntryRequest struct {
	Reason string `json:"reason"`
}

func (h *TreasuryHandler) ListEntities(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	entities, err := h.accountSvc.ListTreasuryEntities(r.Context(), orgCtx.OrganizationID)
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"treasuries": entities})
}

func (h *TreasuryHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	entityID, err := uuid.Parse(chi.URLParam(r, "entityId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid treasury entity ID")
		return
	}
	accounts, err := h.accountSvc.ListAccountsByEntity(r.Context(), orgCtx.OrganizationID, entityID)
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"accounts": serializeAccounts(accounts)})
}

func (h *TreasuryHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	accountID, err := uuid.Parse(chi.URLParam(r, "accountId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid account ID")
		return
	}
	acc, err := h.accountSvc.GetAccount(r.Context(), orgCtx.OrganizationID, accountID)
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"account": serializeAccount(acc)})
}

func (h *TreasuryHandler) ListLedgerEntries(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	accountID, err := uuid.Parse(chi.URLParam(r, "accountId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid account ID")
		return
	}
	entries, err := h.accountSvc.ListEntriesByAccount(r.Context(), orgCtx.OrganizationID, accountID)
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"entries": serializeLedgerEntries(entries)})
}

func (h *TreasuryHandler) Credit(w http.ResponseWriter, r *http.Request) {
	h.handleLedgerMutation(w, r, h.ledgerSvc.Credit)
}

func (h *TreasuryHandler) Debit(w http.ResponseWriter, r *http.Request) {
	h.handleLedgerMutation(w, r, h.ledgerSvc.Debit)
}

func (h *TreasuryHandler) PlaceHold(w http.ResponseWriter, r *http.Request) {
	h.handleLedgerMutation(w, r, h.ledgerSvc.PlaceHold)
}

func (h *TreasuryHandler) ReleaseHold(w http.ResponseWriter, r *http.Request) {
	h.handleLedgerMutation(w, r, h.ledgerSvc.ReleaseHold)
}

func (h *TreasuryHandler) SettleHold(w http.ResponseWriter, r *http.Request) {
	h.handleLedgerMutation(w, r, h.ledgerSvc.SettleHold)
}

func (h *TreasuryHandler) ReverseEntry(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	entryID, err := uuid.Parse(chi.URLParam(r, "entryId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid ledger entry ID")
		return
	}
	var req reverseEntryRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}

	entry, acc, err := h.ledgerSvc.ReverseEntry(r.Context(), service.ReverseEntryInput{
		OrganizationID: orgCtx.OrganizationID,
		EntryID:        entryID,
		ActorID:        &userCtx.UserID,
		ActorRole:      string(orgCtx.Role),
		Reason:         req.Reason,
		CorrelationID:  middleware.CorrelationIDFromContext(r.Context()),
	})
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]any{
		"entry":   serializeLedgerEntry(entry),
		"account": serializeAccount(acc),
	})
}

func (h *TreasuryHandler) ReconcileAccount(w http.ResponseWriter, r *http.Request) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	accountID, err := uuid.Parse(chi.URLParam(r, "accountId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid account ID")
		return
	}

	report, err := h.accountSvc.ReconcileAccount(
		r.Context(),
		orgCtx.OrganizationID,
		accountID,
		&userCtx.UserID,
		string(orgCtx.Role),
		middleware.CorrelationIDFromContext(r.Context()),
	)
	if err != nil {
		WriteDomainError(w, r, err, map[string]any{
			"reconciliation": map[string]any{
				"matched":                     report.Matched,
				"stored_ledger_balance":       report.StoredLedgerBalance.String(),
				"computed_ledger_balance":     report.ComputedLedgerBalance.String(),
				"stored_encumbered_balance":   report.StoredEncumberedBalance.String(),
				"computed_encumbered_balance": report.ComputedEncumberedBalance.String(),
				"account_version":             report.AccountVersion,
			},
		})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"matched":                     report.Matched,
		"stored_ledger_balance":       report.StoredLedgerBalance.String(),
		"computed_ledger_balance":     report.ComputedLedgerBalance.String(),
		"stored_encumbered_balance":   report.StoredEncumberedBalance.String(),
		"computed_encumbered_balance": report.ComputedEncumberedBalance.String(),
		"account_version":             report.AccountVersion,
	})
}

func (h *TreasuryHandler) handleLedgerMutation(
	w http.ResponseWriter,
	r *http.Request,
	fn func(ctx context.Context, in service.LedgerOperationInput) (domain.LedgerEntry, domain.TreasuryAccount, error),
) {
	orgCtx, _ := middleware.OrgContextFromContext(r.Context())
	userCtx, _ := middleware.AuthenticatedUserFromContext(r.Context())
	accountID, err := uuid.Parse(chi.URLParam(r, "accountId"))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid account ID")
		return
	}
	var req ledgerMutationRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request payload")
		return
	}
	amount, err := money.FromString(req.Amount)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_AMOUNT", "invalid decimal amount")
		return
	}
	refID := uuid.New()
	if req.ReferenceID != "" {
		refID, err = uuid.Parse(req.ReferenceID)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "INVALID_ID", "invalid reference_id")
			return
		}
	}
	refType := req.ReferenceType
	if refType == "" {
		refType = "MANUAL_SYNTHETIC"
	}

	entry, acc, err := fn(r.Context(), service.LedgerOperationInput{
		OrganizationID: orgCtx.OrganizationID,
		AccountID:      accountID,
		ActorID:        &userCtx.UserID,
		ActorRole:      string(orgCtx.Role),
		Amount:         amount,
		ReferenceType:  refType,
		ReferenceID:    refID,
		CorrelationID:  middleware.CorrelationIDFromContext(r.Context()),
	})
	if err != nil {
		WriteDomainError(w, r, err)
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]any{
		"entry":   serializeLedgerEntry(entry),
		"account": serializeAccount(acc),
	})
}

func serializeAccount(acc domain.TreasuryAccount) map[string]any {
	return map[string]any{
		"id":                 acc.ID,
		"organization_id":    acc.OrganizationID,
		"treasury_entity_id": acc.TreasuryEntityID,
		"account_code":       acc.AccountCode,
		"name":               acc.Name,
		"currency_code":      acc.CurrencyCode,
		"ledger_balance":     acc.LedgerBalance.String(),
		"encumbered_balance": acc.EncumberedBalance.String(),
		"available_balance":  acc.AvailableBalance().String(),
		"version":            acc.Version,
		"updated_at":         acc.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func serializeAccounts(items []domain.TreasuryAccount) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, serializeAccount(item))
	}
	return out
}

func serializeLedgerEntry(e domain.LedgerEntry) map[string]any {
	return map[string]any{
		"id":                e.ID,
		"organization_id":   e.OrganizationID,
		"account_id":        e.AccountID,
		"entry_type":        e.EntryType,
		"currency_code":     e.CurrencyCode,
		"delta_amount":      e.DeltaAmount.String(),
		"balance_after":     e.BalanceAfter.String(),
		"encumbered_after":  e.EncumberedAfter.String(),
		"account_version":   e.AccountVersion,
		"reversed_entry_id": e.ReversedEntryID,
		"reference_type":    e.ReferenceType,
		"reference_id":      e.ReferenceID,
		"correlation_id":    e.CorrelationID,
		"created_at":        e.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func serializeLedgerEntries(items []domain.LedgerEntry) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, serializeLedgerEntry(item))
	}
	return out
}
