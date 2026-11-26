-- 000_bootstrap.sql: Base schema initialization for QUANTA corporate balance sheets
-- Guarantees self-contained database migrations on fresh PostgreSQL instances

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

CREATE INDEX IF NOT EXISTS idx_corporate_treasuries_created_at ON corporate_treasuries(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_strategy_execution_logs_treasury_id ON strategy_execution_logs(treasury_id);
