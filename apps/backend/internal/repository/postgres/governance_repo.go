package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/atharvix/quanta/apps/backend/internal/money"
)

type OptimizationRunRepository struct {
	db DBTX
}

func NewOptimizationRunRepository(db DBTX) *OptimizationRunRepository {
	return &OptimizationRunRepository{db: db}
}

func (r *OptimizationRunRepository) CreateRun(ctx context.Context, run domain.OptimizationRun) error {
	fxIDs := run.FXSnapshotIDs
	if len(fxIDs) == 0 {
		fxIDs = []byte("[]")
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO optimization_runs (
			id, organization_id, treasury_entity_id, requested_by, correlation_id,
			solver_category, status, feasible, problem_snapshot, raw_response,
			fx_snapshot_ids, runtime_ms
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::JSONB, $10::JSONB, $11::JSONB, $12)
	`,
		run.ID, run.OrganizationID, run.TreasuryEntityID, run.RequestedBy, run.CorrelationID,
		run.SolverCategory, string(run.Status), run.Feasible,
		string(run.ProblemSnapshot), string(run.RawResponse), string(fxIDs), run.RuntimeMs,
	)
	return err
}

func (r *OptimizationRunRepository) GetRun(ctx context.Context, orgID, runID uuid.UUID) (domain.OptimizationRun, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, organization_id, treasury_entity_id, requested_by, correlation_id,
		       solver_category, status, feasible, problem_snapshot, raw_response,
		       fx_snapshot_ids, runtime_ms, created_at
		FROM optimization_runs
		WHERE organization_id = $1 AND id = $2
	`, orgID, runID)
	run, err := scanRunRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OptimizationRun{}, domain.ErrNotFound
	}
	return run, err
}

func (r *OptimizationRunRepository) ListRunsByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.OptimizationRun, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, treasury_entity_id, requested_by, correlation_id,
		       solver_category, status, feasible, problem_snapshot, raw_response,
		       fx_snapshot_ids, runtime_ms, created_at
		FROM optimization_runs
		WHERE organization_id = $1 AND treasury_entity_id = $2
		ORDER BY created_at DESC
	`, orgID, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []domain.OptimizationRun
	for rows.Next() {
		run, err := scanRunRow(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func scanRunRow(row pgx.Row) (domain.OptimizationRun, error) {
	var run domain.OptimizationRun
	var status string
	err := row.Scan(
		&run.ID, &run.OrganizationID, &run.TreasuryEntityID, &run.RequestedBy, &run.CorrelationID,
		&run.SolverCategory, &status, &run.Feasible, &run.ProblemSnapshot, &run.RawResponse,
		&run.FXSnapshotIDs, &run.RuntimeMs, &run.CreatedAt,
	)
	if err != nil {
		return domain.OptimizationRun{}, err
	}
	run.Status = domain.OptimizationStatus(status)
	return run, nil
}

type ProposalRepository struct {
	db DBTX
}

func NewProposalRepository(db DBTX) *ProposalRepository {
	return &ProposalRepository{db: db}
}

func (r *ProposalRepository) CreateProposal(ctx context.Context, p domain.AllocationProposal) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO allocation_proposals (
			id, organization_id, optimization_run_id, treasury_entity_id, supersedes_proposal_id,
			status, investable_capital, total_allocated, residual_cash,
			verified_allocations, snapshot_account_versions, snapshot_policy_version,
			submitted_by, submitted_at, expires_at, version
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7::NUMERIC(20,4), $8::NUMERIC(20,4), $9::NUMERIC(20,4),
			$10::JSONB, $11::JSONB, $12,
			$13, $14, $15, $16
		)
	`,
		p.ID, p.OrganizationID, p.OptimizationRunID, p.TreasuryEntityID, p.SupersedesProposalID,
		string(p.Status), p.InvestableCapital.String(), p.TotalAllocated.String(), p.ResidualCash.String(),
		string(p.VerifiedAllocations), string(p.SnapshotAccountVersions), p.SnapshotPolicyVersion,
		p.SubmittedBy, p.SubmittedAt, p.ExpiresAt, p.Version,
	)
	return err
}

func (r *ProposalRepository) GetProposal(ctx context.Context, orgID, proposalID uuid.UUID) (domain.AllocationProposal, error) {
	return r.scanSingleProposal(ctx, `
		SELECT id, organization_id, optimization_run_id, treasury_entity_id, supersedes_proposal_id,
		       status, investable_capital::TEXT, total_allocated::TEXT, residual_cash::TEXT,
		       verified_allocations, snapshot_account_versions, snapshot_policy_version,
		       submitted_by, submitted_at, expires_at, version, created_at
		FROM allocation_proposals
		WHERE organization_id = $1 AND id = $2
	`, orgID, proposalID)
}

func (r *ProposalRepository) GetProposalForUpdate(ctx context.Context, orgID, proposalID uuid.UUID) (domain.AllocationProposal, error) {
	return r.scanSingleProposal(ctx, `
		SELECT id, organization_id, optimization_run_id, treasury_entity_id, supersedes_proposal_id,
		       status, investable_capital::TEXT, total_allocated::TEXT, residual_cash::TEXT,
		       verified_allocations, snapshot_account_versions, snapshot_policy_version,
		       submitted_by, submitted_at, expires_at, version, created_at
		FROM allocation_proposals
		WHERE organization_id = $1 AND id = $2
		FOR UPDATE
	`, orgID, proposalID)
}

func (r *ProposalRepository) ListProposalsByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]domain.AllocationProposal, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, optimization_run_id, treasury_entity_id, supersedes_proposal_id,
		       status, investable_capital::TEXT, total_allocated::TEXT, residual_cash::TEXT,
		       verified_allocations, snapshot_account_versions, snapshot_policy_version,
		       submitted_by, submitted_at, expires_at, version, created_at
		FROM allocation_proposals
		WHERE organization_id = $1 AND treasury_entity_id = $2
		ORDER BY created_at DESC
	`, orgID, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AllocationProposal
	for rows.Next() {
		p, err := scanProposalRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *ProposalRepository) UpdateProposalStatus(
	ctx context.Context,
	orgID, proposalID uuid.UUID,
	expectedVersion int64,
	newStatus domain.ProposalStatus,
	submittedBy *uuid.UUID,
	submittedAtUnix, expiresAtUnix *int64,
) (domain.AllocationProposal, error) {
	var submittedAt, expiresAt *time.Time
	if submittedAtUnix != nil {
		t := time.Unix(*submittedAtUnix, 0).UTC()
		submittedAt = &t
	}
	if expiresAtUnix != nil {
		t := time.Unix(*expiresAtUnix, 0).UTC()
		expiresAt = &t
	}

	row := r.db.QueryRow(ctx, `
		UPDATE allocation_proposals
		SET status = $4,
		    submitted_by = COALESCE($5, submitted_by),
		    submitted_at = COALESCE($6, submitted_at),
		    expires_at = COALESCE($7, expires_at),
		    version = version + 1
		WHERE organization_id = $1 AND id = $2 AND version = $3
		RETURNING id, organization_id, optimization_run_id, treasury_entity_id, supersedes_proposal_id,
		          status, investable_capital::TEXT, total_allocated::TEXT, residual_cash::TEXT,
		          verified_allocations, snapshot_account_versions, snapshot_policy_version,
		          submitted_by, submitted_at, expires_at, version, created_at
	`, orgID, proposalID, expectedVersion, string(newStatus), submittedBy, submittedAt, expiresAt)

	p, err := scanProposalRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AllocationProposal{}, domain.ErrVersionConflict
	}
	return p, err
}

func (r *ProposalRepository) scanSingleProposal(ctx context.Context, query string, orgID, proposalID uuid.UUID) (domain.AllocationProposal, error) {
	p, err := scanProposalRow(r.db.QueryRow(ctx, query, orgID, proposalID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AllocationProposal{}, domain.ErrNotFound
	}
	return p, err
}

func scanProposalRow(row pgx.Row) (domain.AllocationProposal, error) {
	var p domain.AllocationProposal
	var status, invRaw, allocRaw, resRaw string
	err := row.Scan(
		&p.ID, &p.OrganizationID, &p.OptimizationRunID, &p.TreasuryEntityID, &p.SupersedesProposalID,
		&status, &invRaw, &allocRaw, &resRaw,
		&p.VerifiedAllocations, &p.SnapshotAccountVersions, &p.SnapshotPolicyVersion,
		&p.SubmittedBy, &p.SubmittedAt, &p.ExpiresAt, &p.Version, &p.CreatedAt,
	)
	if err != nil {
		return domain.AllocationProposal{}, err
	}
	inv, err := money.FromString(invRaw)
	if err != nil {
		return domain.AllocationProposal{}, fmt.Errorf("parse investable_capital: %w", err)
	}
	alloc, err := money.FromString(allocRaw)
	if err != nil {
		return domain.AllocationProposal{}, fmt.Errorf("parse total_allocated: %w", err)
	}
	res, err := money.FromString(resRaw)
	if err != nil {
		return domain.AllocationProposal{}, fmt.Errorf("parse residual_cash: %w", err)
	}
	p.Status = domain.ProposalStatus(status)
	p.InvestableCapital = inv
	p.TotalAllocated = alloc
	p.ResidualCash = res
	return p, nil
}

type ApprovalDecisionRepository struct {
	db DBTX
}

func NewApprovalDecisionRepository(db DBTX) *ApprovalDecisionRepository {
	return &ApprovalDecisionRepository{db: db}
}

func (r *ApprovalDecisionRepository) AppendDecision(ctx context.Context, d domain.ApprovalDecision) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO approval_decisions (
			id, organization_id, proposal_id, approver_id, decision, rationale, live_balance_snapshot
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::JSONB)
	`, d.ID, d.OrganizationID, d.ProposalID, d.ApproverID, string(d.Decision), d.Rationale, string(d.LiveBalanceSnapshot))
	return err
}

func (r *ApprovalDecisionRepository) ListDecisionsByProposal(ctx context.Context, orgID, proposalID uuid.UUID) ([]domain.ApprovalDecision, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, organization_id, proposal_id, approver_id, decision, rationale, live_balance_snapshot, decided_at
		FROM approval_decisions
		WHERE organization_id = $1 AND proposal_id = $2
		ORDER BY decided_at DESC
	`, orgID, proposalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ApprovalDecision
	for rows.Next() {
		var d domain.ApprovalDecision
		var dec string
		if err := rows.Scan(&d.ID, &d.OrganizationID, &d.ProposalID, &d.ApproverID, &dec, &d.Rationale, &d.LiveBalanceSnapshot, &d.DecidedAt); err != nil {
			return nil, err
		}
		d.Decision = domain.ApprovalDecisionType(dec)
		out = append(out, d)
	}
	return out, rows.Err()
}
