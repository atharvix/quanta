-- 004_treasury_assets_liabilities.sql: Schema for custom financial assets and liabilities per treasury

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

-- Seed default baseline financial entries for all existing corporate treasuries
INSERT INTO treasury_assets (treasury_id, asset_id, name, asset_type, cost, expected_return, risk, liquidity, volatility)
SELECT id, 'gov_bond', 'Government Bonds', 'gov_bond', 0.001, 0.07, 0.05, 0.95, 0.03 FROM corporate_treasuries
ON CONFLICT (treasury_id, asset_id) DO NOTHING;

INSERT INTO treasury_assets (treasury_id, asset_id, name, asset_type, cost, expected_return, risk, liquidity, volatility)
SELECT id, 'bluechip_equity', 'Bluechip Equity Index', 'equity', 0.002, 0.12, 0.18, 0.85, 0.15 FROM corporate_treasuries
ON CONFLICT (treasury_id, asset_id) DO NOTHING;

INSERT INTO treasury_assets (treasury_id, asset_id, name, asset_type, cost, expected_return, risk, liquidity, volatility)
SELECT id, 'commercial_paper', 'Corporate Commercial Paper', 'corp_bond', 0.001, 0.08, 0.08, 0.75, 0.06 FROM corporate_treasuries
ON CONFLICT (treasury_id, asset_id) DO NOTHING;

INSERT INTO treasury_liabilities (treasury_id, liability_id, name, principal, interest_rate, maturity_months, priority, is_mandatory_reserve)
SELECT id, 'senior_loan', 'Senior Revolving Facility', 15.0, 0.065, 12, 1, true FROM corporate_treasuries
ON CONFLICT (treasury_id, liability_id) DO NOTHING;
