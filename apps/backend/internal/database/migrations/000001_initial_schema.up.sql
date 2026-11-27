-- QUANTA 2.0 Milestone 2: Core Multi-Tenancy, Treasury, Ledger, and Governance Schema
-- PostgreSQL-native schema using fixed-point NUMERIC(20, 4) / NUMERIC(20, 8).

CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'SUSPENDED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    full_name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS organization_memberships (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    user_id UUID NOT NULL REFERENCES users(id),
    role VARCHAR(32) NOT NULL CHECK (role IN ('ADMIN', 'TREASURY_APPROVER', 'TREASURY_ANALYST', 'VIEWER')),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'SUSPENDED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_org_membership_org_user UNIQUE (organization_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_org_memberships_org_id ON organization_memberships (organization_id);
CREATE INDEX IF NOT EXISTS idx_org_memberships_user_id ON organization_memberships (user_id);

CREATE TABLE IF NOT EXISTS treasury_entities (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    name TEXT NOT NULL,
    base_currency VARCHAR(10) NOT NULL,
    is_synthetic BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_treasury_entities_id_org UNIQUE (id, organization_id),
    CONSTRAINT uq_treasury_entities_id_currency UNIQUE (id, base_currency),
    CONSTRAINT uq_treasury_entities_org_name UNIQUE (organization_id, name)
);

CREATE INDEX IF NOT EXISTS idx_treasury_entities_org_id ON treasury_entities (organization_id);

CREATE TABLE IF NOT EXISTS treasury_accounts (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    treasury_entity_id UUID NOT NULL REFERENCES treasury_entities(id),
    account_code VARCHAR(64) NOT NULL,
    name TEXT NOT NULL,
    currency_code VARCHAR(10) NOT NULL,
    ledger_balance NUMERIC(20, 4) NOT NULL DEFAULT 0 CHECK (ledger_balance >= 0),
    encumbered_balance NUMERIC(20, 4) NOT NULL DEFAULT 0 CHECK (encumbered_balance >= 0 AND encumbered_balance <= ledger_balance),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_treasury_accounts_entity_code UNIQUE (treasury_entity_id, account_code),
    CONSTRAINT uq_treasury_accounts_id_org_currency UNIQUE (id, organization_id, currency_code),
    CONSTRAINT fk_treasury_accounts_entity_org
        FOREIGN KEY (treasury_entity_id, organization_id)
        REFERENCES treasury_entities (id, organization_id),
    -- Single-currency-per-treasury MVP enforcement at the relational boundary.
    CONSTRAINT fk_treasury_accounts_single_currency
        FOREIGN KEY (treasury_entity_id, currency_code)
        REFERENCES treasury_entities (id, base_currency)
);

CREATE INDEX IF NOT EXISTS idx_treasury_accounts_org_id ON treasury_accounts (organization_id);
CREATE INDEX IF NOT EXISTS idx_treasury_accounts_entity_id ON treasury_accounts (treasury_entity_id);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    account_id UUID NOT NULL REFERENCES treasury_accounts(id),
    entry_type VARCHAR(24) NOT NULL CHECK (
        entry_type IN ('CREDIT', 'DEBIT', 'REVERSAL', 'HOLD_PLACED', 'HOLD_RELEASED', 'HOLD_SETTLED')
    ),
    currency_code VARCHAR(10) NOT NULL,
    delta_amount NUMERIC(20, 4) NOT NULL CHECK (delta_amount <> 0),
    balance_after NUMERIC(20, 4) NOT NULL CHECK (balance_after >= 0),
    encumbered_after NUMERIC(20, 4) NOT NULL CHECK (encumbered_after >= 0 AND encumbered_after <= balance_after),
    account_version BIGINT NOT NULL CHECK (account_version >= 1),
    reversed_entry_id UUID NULL REFERENCES ledger_entries(id),
    reference_type VARCHAR(64) NOT NULL,
    reference_id UUID NOT NULL,
    correlation_id VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_ledger_entries_account_org_currency
        FOREIGN KEY (account_id, organization_id, currency_code)
        REFERENCES treasury_accounts (id, organization_id, currency_code),
    CONSTRAINT chk_ledger_entry_sign CHECK (
        (entry_type = 'CREDIT' AND delta_amount > 0) OR
        (entry_type = 'DEBIT' AND delta_amount < 0) OR
        (entry_type = 'REVERSAL' AND delta_amount <> 0) OR
        (entry_type = 'HOLD_PLACED' AND delta_amount > 0) OR
        (entry_type = 'HOLD_RELEASED' AND delta_amount < 0) OR
        (entry_type = 'HOLD_SETTLED' AND delta_amount < 0)
    ),
    CONSTRAINT chk_reversal_requires_target CHECK (
        (entry_type = 'REVERSAL' AND reversed_entry_id IS NOT NULL) OR
        (entry_type <> 'REVERSAL' AND reversed_entry_id IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_org_account_created
    ON ledger_entries (organization_id, account_id, created_at, id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_reference
    ON ledger_entries (organization_id, reference_type, reference_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_ledger_entries_reversed_entry_id
    ON ledger_entries (reversed_entry_id)
    WHERE reversed_entry_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS fx_rate_snapshots (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    from_currency VARCHAR(10) NOT NULL,
    to_currency VARCHAR(10) NOT NULL,
    rate NUMERIC(20, 8) NOT NULL CHECK (rate > 0),
    effective_at TIMESTAMPTZ NOT NULL,
    source_provenance VARCHAR(120) NOT NULL,
    is_synthetic BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fx_rate_snapshots_org_pair_time
    ON fx_rate_snapshots (organization_id, from_currency, to_currency, effective_at DESC);

CREATE TABLE IF NOT EXISTS liquidity_policies (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    treasury_entity_id UUID NOT NULL REFERENCES treasury_entities(id),
    version BIGINT NOT NULL CHECK (version >= 1),
    min_operating_liquidity NUMERIC(20, 4) NOT NULL CHECK (min_operating_liquidity >= 0),
    min_emergency_reserve NUMERIC(20, 4) NOT NULL CHECK (min_emergency_reserve >= 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_liquidity_policies_entity_version UNIQUE (treasury_entity_id, version),
    CONSTRAINT fk_liquidity_policies_entity_org
        FOREIGN KEY (treasury_entity_id, organization_id)
        REFERENCES treasury_entities (id, organization_id)
);

CREATE INDEX IF NOT EXISTS idx_liquidity_policies_org_entity
    ON liquidity_policies (organization_id, treasury_entity_id);

CREATE TABLE IF NOT EXISTS counterparties (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    treasury_entity_id UUID NOT NULL REFERENCES treasury_entities(id),
    name TEXT NOT NULL,
    currency_code VARCHAR(10) NOT NULL,
    max_exposure_limit NUMERIC(20, 4) NOT NULL CHECK (max_exposure_limit >= 0),
    current_exposure NUMERIC(20, 4) NOT NULL DEFAULT 0 CHECK (current_exposure >= 0),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_counterparties_entity_name UNIQUE (treasury_entity_id, name),
    CONSTRAINT fk_counterparties_entity_org
        FOREIGN KEY (treasury_entity_id, organization_id)
        REFERENCES treasury_entities (id, organization_id)
);

CREATE INDEX IF NOT EXISTS idx_counterparties_org_entity
    ON counterparties (organization_id, treasury_entity_id);

CREATE TABLE IF NOT EXISTS financial_constraints (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    treasury_entity_id UUID NOT NULL REFERENCES treasury_entities(id),
    version BIGINT NOT NULL CHECK (version >= 1),
    constraint_type VARCHAR(64) NOT NULL,
    severity VARCHAR(16) NOT NULL CHECK (severity IN ('HARD', 'SOFT')),
    parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_financial_constraints_entity_org
        FOREIGN KEY (treasury_entity_id, organization_id)
        REFERENCES treasury_entities (id, organization_id)
);

CREATE INDEX IF NOT EXISTS idx_financial_constraints_org_entity
    ON financial_constraints (organization_id, treasury_entity_id, is_active);

CREATE TABLE IF NOT EXISTS optimization_runs (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    treasury_entity_id UUID NOT NULL REFERENCES treasury_entities(id),
    requested_by UUID NOT NULL REFERENCES users(id),
    correlation_id VARCHAR(128) NOT NULL,
    solver_category VARCHAR(32) NOT NULL CHECK (solver_category IN ('classical', 'quantum_research', 'hybrid_benchmark')),
    status VARCHAR(32) NOT NULL CHECK (status IN ('COMPLETED', 'INFEASIBLE', 'SOLVER_FAILED', 'VERIFICATION_FAILED', 'CANCELLED')),
    feasible BOOLEAN NOT NULL,
    problem_snapshot JSONB NOT NULL,
    raw_response JSONB NOT NULL,
    fx_snapshot_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    runtime_ms INTEGER NOT NULL CHECK (runtime_ms >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_optimization_runs_id_org UNIQUE (id, organization_id),
    CONSTRAINT fk_optimization_runs_entity_org
        FOREIGN KEY (treasury_entity_id, organization_id)
        REFERENCES treasury_entities (id, organization_id)
);

CREATE INDEX IF NOT EXISTS idx_optimization_runs_org_entity_created
    ON optimization_runs (organization_id, treasury_entity_id, created_at DESC);

CREATE TABLE IF NOT EXISTS allocation_proposals (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    optimization_run_id UUID NOT NULL REFERENCES optimization_runs(id),
    treasury_entity_id UUID NOT NULL REFERENCES treasury_entities(id),
    supersedes_proposal_id UUID NULL REFERENCES allocation_proposals(id),
    status VARCHAR(32) NOT NULL CHECK (status IN ('DRAFT', 'PENDING_APPROVAL', 'APPROVED', 'REJECTED', 'EXPIRED')),
    investable_capital NUMERIC(20, 4) NOT NULL CHECK (investable_capital >= 0),
    total_allocated NUMERIC(20, 4) NOT NULL CHECK (total_allocated >= 0),
    residual_cash NUMERIC(20, 4) NOT NULL CHECK (residual_cash >= 0),
    verified_allocations JSONB NOT NULL,
    snapshot_account_versions JSONB NOT NULL,
    snapshot_policy_version BIGINT NOT NULL,
    submitted_by UUID NULL REFERENCES users(id),
    submitted_at TIMESTAMPTZ NULL,
    expires_at TIMESTAMPTZ NULL,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_allocation_proposals_id_org UNIQUE (id, organization_id),
    CONSTRAINT fk_allocation_proposals_run_org
        FOREIGN KEY (optimization_run_id, organization_id)
        REFERENCES optimization_runs (id, organization_id),
    CONSTRAINT fk_allocation_proposals_entity_org
        FOREIGN KEY (treasury_entity_id, organization_id)
        REFERENCES treasury_entities (id, organization_id),
    CONSTRAINT chk_proposal_capital_conservation CHECK (total_allocated + residual_cash = investable_capital)
);

CREATE INDEX IF NOT EXISTS idx_allocation_proposals_org_entity_status
    ON allocation_proposals (organization_id, treasury_entity_id, status);

CREATE TABLE IF NOT EXISTS approval_decisions (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    proposal_id UUID NOT NULL REFERENCES allocation_proposals(id),
    approver_id UUID NOT NULL REFERENCES users(id),
    decision VARCHAR(16) NOT NULL CHECK (decision IN ('APPROVE', 'REJECT')),
    rationale TEXT NOT NULL,
    live_balance_snapshot JSONB NOT NULL,
    decided_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_approval_decisions_proposal_org
        FOREIGN KEY (proposal_id, organization_id)
        REFERENCES allocation_proposals (id, organization_id)
);

CREATE INDEX IF NOT EXISTS idx_approval_decisions_org_proposal
    ON approval_decisions (organization_id, proposal_id, decided_at DESC);

CREATE TABLE IF NOT EXISTS audit_events (
    id UUID PRIMARY KEY,
    organization_id UUID NULL REFERENCES organizations(id),
    actor_id UUID NULL,
    actor_role VARCHAR(32) NOT NULL,
    action VARCHAR(128) NOT NULL,
    entity_type VARCHAR(64) NOT NULL,
    entity_id UUID NULL,
    correlation_id VARCHAR(128) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_events_org_created
    ON audit_events (organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_correlation
    ON audit_events (correlation_id);
