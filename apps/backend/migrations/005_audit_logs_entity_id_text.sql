-- 005_audit_logs_entity_id_text.sql: Alter audit_logs.entity_id from UUID to VARCHAR(255)
-- Enables auditing of custom financial assets and liabilities with business identifiers

ALTER TABLE audit_logs 
    ALTER COLUMN entity_id TYPE VARCHAR(255) USING entity_id::VARCHAR(255);
