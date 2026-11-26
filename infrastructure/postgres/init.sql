-- QUANTA Database Initialization Schema (PostgreSQL)

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS corporate_treasuries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    base_currency VARCHAR(10) DEFAULT 'INR',
    total_capital NUMERIC(18, 2) NOT NULL,
    risk_tolerance NUMERIC(5, 4) NOT NULL DEFAULT 0.5,
    min_liquidity_ratio NUMERIC(5, 4) NOT NULL DEFAULT 0.2,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS strategy_execution_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    treasury_id UUID REFERENCES corporate_treasuries(id) ON DELETE CASCADE,
    algorithm_used VARCHAR(100) NOT NULL,
    objective_value NUMERIC(12, 6) NOT NULL,
    risk_score NUMERIC(5, 4) NOT NULL,
    expected_return NUMERIC(5, 4) NOT NULL,
    liquidity_ratio NUMERIC(5, 4) NOT NULL,
    allocations_json JSONB NOT NULL,
    constraints_audit_json JSONB NOT NULL,
    execution_time_ms NUMERIC(10, 2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS treasury_assets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    treasury_id UUID NOT NULL REFERENCES corporate_treasuries(id) ON DELETE CASCADE,
    asset_id VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    asset_type VARCHAR(100) NOT NULL,
    cost NUMERIC(18, 6) NOT NULL DEFAULT 0.001,
    expected_return NUMERIC(8, 6) NOT NULL DEFAULT 0.06,
    risk NUMERIC(6, 4) NOT NULL DEFAULT 0.05,
    liquidity NUMERIC(6, 4) NOT NULL DEFAULT 0.80,
    volatility NUMERIC(8, 6) NOT NULL DEFAULT 0.04,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    CONSTRAINT uq_treasury_asset UNIQUE (treasury_id, asset_id)
);

CREATE INDEX IF NOT EXISTS idx_treasury_assets_treasury_id ON treasury_assets(treasury_id);

CREATE TABLE IF NOT EXISTS treasury_liabilities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    treasury_id UUID NOT NULL REFERENCES corporate_treasuries(id) ON DELETE CASCADE,
    liability_id VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    principal NUMERIC(18, 2) NOT NULL DEFAULT 0,
    interest_rate NUMERIC(8, 6) NOT NULL DEFAULT 0.05,
    maturity_months INT DEFAULT 12,
    priority INT NOT NULL DEFAULT 1,
    is_mandatory_reserve BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    CONSTRAINT uq_treasury_liability UNIQUE (treasury_id, liability_id)
);

CREATE INDEX IF NOT EXISTS idx_treasury_liabilities_treasury_id ON treasury_liabilities(treasury_id);
