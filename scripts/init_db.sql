-- scripts/init_db.sql
-- Idempotent PostgreSQL initialization & verification script for QUANTA native local setup.
-- Usage (run as postgres superuser):
--   psql -U postgres -h localhost -p 5432 -f scripts/init_db.sql

-- 1. Ensure 'quanta' login role exists with password 'quanta_dev'
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'quanta') THEN
        CREATE ROLE quanta WITH LOGIN PASSWORD 'quanta_dev';
        RAISE NOTICE 'Created PostgreSQL role "quanta".';
    ELSE
        ALTER ROLE quanta WITH LOGIN PASSWORD 'quanta_dev';
        RAISE NOTICE 'PostgreSQL role "quanta" already exists (password synchronized).';
    END IF;
END
$$;

-- 2. Ensure 'quanta_db' database exists (must execute outside a transaction block via \gexec)
SELECT 'CREATE DATABASE quanta_db OWNER quanta'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'quanta_db')\gexec

GRANT ALL PRIVILEGES ON DATABASE quanta_db TO quanta;

-- 3. Switch context to quanta_db and configure schema/extension permissions
\connect quanta_db

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

GRANT USAGE, CREATE ON SCHEMA public TO quanta;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO quanta;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO quanta;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO quanta;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO quanta;

-- 4. Verify existence of core QUANTA tables in public schema
SELECT
    expected.table_name,
    CASE
        WHEN t.table_name IS NOT NULL THEN 'EXISTS'
        ELSE 'MISSING (run backend or migrations 000-005)'
    END AS status
FROM (
    VALUES
        ('users'),
        ('corporate_treasuries'),
        ('treasury_assets'),
        ('treasury_liabilities'),
        ('financial_problems'),
        ('orchestration_runs'),
        ('audit_logs')
) AS expected(table_name)
LEFT JOIN information_schema.tables t
    ON t.table_schema = 'public'
   AND t.table_name = expected.table_name
ORDER BY expected.table_name;
