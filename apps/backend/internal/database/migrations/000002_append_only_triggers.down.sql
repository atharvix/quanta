DROP TRIGGER IF EXISTS trg_audit_events_append_only ON audit_events;
DROP TRIGGER IF EXISTS trg_approval_decisions_append_only ON approval_decisions;
DROP TRIGGER IF EXISTS trg_fx_rate_snapshots_append_only ON fx_rate_snapshots;
DROP TRIGGER IF EXISTS trg_ledger_entries_append_only ON ledger_entries;
DROP FUNCTION IF EXISTS quanta_prevent_append_only_mutation();
