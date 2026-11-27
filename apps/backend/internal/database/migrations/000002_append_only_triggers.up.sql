-- QUANTA 2.0 Milestone 2: Append-Only Immutability Triggers
-- Enforces that historical ledger, FX snapshot, approval, and audit rows
-- cannot be destructively modified or deleted.

CREATE OR REPLACE FUNCTION quanta_prevent_append_only_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'append-only table "%" does not permit % operations', TG_TABLE_NAME, TG_OP;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_ledger_entries_append_only ON ledger_entries;
CREATE TRIGGER trg_ledger_entries_append_only
    BEFORE UPDATE OR DELETE ON ledger_entries
    FOR EACH ROW EXECUTE FUNCTION quanta_prevent_append_only_mutation();

DROP TRIGGER IF EXISTS trg_fx_rate_snapshots_append_only ON fx_rate_snapshots;
CREATE TRIGGER trg_fx_rate_snapshots_append_only
    BEFORE UPDATE OR DELETE ON fx_rate_snapshots
    FOR EACH ROW EXECUTE FUNCTION quanta_prevent_append_only_mutation();

DROP TRIGGER IF EXISTS trg_approval_decisions_append_only ON approval_decisions;
CREATE TRIGGER trg_approval_decisions_append_only
    BEFORE UPDATE OR DELETE ON approval_decisions
    FOR EACH ROW EXECUTE FUNCTION quanta_prevent_append_only_mutation();

DROP TRIGGER IF EXISTS trg_audit_events_append_only ON audit_events;
CREATE TRIGGER trg_audit_events_append_only
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION quanta_prevent_append_only_mutation();
