-- scripts/seed_demo.sql
-- Idempotent demo operator accounts and starter portfolios for QUANTA local walkthroughs.
-- Demo password for all three accounts: password123

DO $$
DECLARE
    pw_hash CONSTANT TEXT := '$2a$10$Cd6th0S.ufEexQyCk0NDqejNb0PcwSLm20ReHR4ViaWi2sE3dDauS';
    analyst_uid UUID;
    viewer_uid UUID;
    analyst_treasury_id UUID;
    viewer_treasury_id UUID;
BEGIN
    -- 1. Seed one demo operator per RBAC role
    INSERT INTO users (email, password_hash, full_name, role)
    VALUES
        ('admin@quanta.local', pw_hash, 'Admin Operator', 'admin'),
        ('analyst@quanta.local', pw_hash, 'Treasury Analyst', 'analyst'),
        ('viewer@quanta.local', pw_hash, 'Compliance Auditor', 'viewer')
    ON CONFLICT (email) DO NOTHING;

    SELECT id INTO analyst_uid FROM users WHERE email = 'analyst@quanta.local';
    SELECT id INTO viewer_uid FROM users WHERE email = 'viewer@quanta.local';

    -- 2. Seed Analyst demo portfolio (visible to Admin and Analyst)
    IF analyst_uid IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM corporate_treasuries WHERE name = 'Meridian Global Treasury'
    ) THEN
        INSERT INTO corporate_treasuries (
            name, base_currency, total_capital, risk_tolerance, min_liquidity_ratio, owner_id
        ) VALUES (
            'Meridian Global Treasury', 'USD', 120000000.00, 0.45, 0.25, analyst_uid
        ) RETURNING id INTO analyst_treasury_id;

        INSERT INTO treasury_assets (
            treasury_id, asset_id, name, asset_type, cost, expected_return, risk, liquidity, volatility
        ) VALUES
            (analyst_treasury_id, 'gov_bond', 'US Treasury Sovereign Paper', 'gov_bond', 0.001, 0.052, 0.04, 0.96, 0.025),
            (analyst_treasury_id, 'corp_bond', 'Investment Grade Corporate Debt', 'corp_bond', 0.0015, 0.078, 0.11, 0.80, 0.06),
            (analyst_treasury_id, 'bluechip_equity', 'Global Equity Index Core', 'equity', 0.0025, 0.125, 0.28, 0.85, 0.16),
            (analyst_treasury_id, 'tbill_reserve', '91-Day Treasury Liquidity Pool', 'cash', 0.0005, 0.041, 0.015, 0.99, 0.01)
        ON CONFLICT (treasury_id, asset_id) DO NOTHING;

        INSERT INTO treasury_liabilities (
            treasury_id, liability_id, name, principal, interest_rate, maturity_months, priority, is_mandatory_reserve
        ) VALUES
            (analyst_treasury_id, 'senior_revolver', 'Senior Revolving Credit Facility', 18000000.00, 0.062, 12, 1, TRUE)
        ON CONFLICT (treasury_id, liability_id) DO NOTHING;
    END IF;

    -- 3. Seed Viewer audit portfolio (visible to Admin and Viewer in read-only mode)
    IF viewer_uid IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM corporate_treasuries WHERE name = 'Meridian Audit Reserve'
    ) THEN
        INSERT INTO corporate_treasuries (
            name, base_currency, total_capital, risk_tolerance, min_liquidity_ratio, owner_id
        ) VALUES (
            'Meridian Audit Reserve', 'INR', 100.00, 0.35, 0.25, viewer_uid
        ) RETURNING id INTO viewer_treasury_id;

        INSERT INTO treasury_assets (
            treasury_id, asset_id, name, asset_type, cost, expected_return, risk, liquidity, volatility
        ) VALUES
            (viewer_treasury_id, 'gov_bond', 'Sovereign G-Sec Portfolio', 'gov_bond', 0.001, 0.072, 0.05, 0.95, 0.03),
            (viewer_treasury_id, 'corp_bond', 'AAA Corporate Commercial Paper', 'corp_bond', 0.001, 0.085, 0.09, 0.78, 0.05),
            (viewer_treasury_id, 'bluechip_equity', 'Large-Cap Equity Basket', 'equity', 0.002, 0.132, 0.24, 0.82, 0.14)
        ON CONFLICT (treasury_id, asset_id) DO NOTHING;

        INSERT INTO treasury_liabilities (
            treasury_id, liability_id, name, principal, interest_rate, maturity_months, priority, is_mandatory_reserve
        ) VALUES
            (viewer_treasury_id, 'term_facility', 'Senior Term Obligation', 15.00, 0.068, 12, 1, TRUE)
        ON CONFLICT (treasury_id, liability_id) DO NOTHING;
    END IF;
END
$$;
