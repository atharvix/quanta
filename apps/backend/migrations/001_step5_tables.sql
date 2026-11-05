-- 001_step5_tables.sql: Schema migrations for Step 5 deliverables

-- Ensure schema_migrations table exists to track applied versions
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- a) financial_problems
CREATE TABLE IF NOT EXISTS financial_problems (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    treasury_id UUID NOT NULL REFERENCES corporate_treasuries(id) ON DELETE CASCADE,
    algorithm VARCHAR(50),
    objective VARCHAR(100),
    market_state VARCHAR(50),
    problem_json JSONB NOT NULL,
    strategy_log_id UUID REFERENCES strategy_execution_logs(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- b) orchestration_runs
CREATE TABLE IF NOT EXISTS orchestration_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    treasury_id UUID NOT NULL REFERENCES corporate_treasuries(id) ON DELETE CASCADE,
    status VARCHAR(30) NOT NULL,
    ai_status VARCHAR(30),
    optimization_status VARCHAR(30),
    strategy_log_id UUID REFERENCES strategy_execution_logs(id) ON DELETE SET NULL,
    response_json JSONB,
    error_message TEXT,
    duration_ms NUMERIC(10, 2),
    created_at TIMESTAMPTZ DEFAULT now()
);

-- c) audit_logs
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID,
    actor VARCHAR(100) NOT NULL DEFAULT 'system',
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity_action ON audit_logs(entity_type, action);
CREATE INDEX IF NOT EXISTS idx_financial_problems_treasury_id ON financial_problems(treasury_id);
CREATE INDEX IF NOT EXISTS idx_orchestration_runs_treasury_id ON orchestration_runs(treasury_id);
