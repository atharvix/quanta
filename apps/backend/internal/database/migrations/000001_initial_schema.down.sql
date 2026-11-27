-- Rollback QUANTA 2.0 Milestone 2 Schema
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS approval_decisions;
DROP TABLE IF EXISTS allocation_proposals;
DROP TABLE IF EXISTS optimization_runs;
DROP TABLE IF EXISTS financial_constraints;
DROP TABLE IF EXISTS counterparties;
DROP TABLE IF EXISTS liquidity_policies;
DROP TABLE IF EXISTS fx_rate_snapshots;
DROP TABLE IF EXISTS ledger_entries;
DROP TABLE IF EXISTS treasury_accounts;
DROP TABLE IF EXISTS treasury_entities;
DROP TABLE IF EXISTS organization_memberships;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;
