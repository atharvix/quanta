-- 003_treasury_owner.sql: Add owner_id to corporate_treasuries for resource-level authorization

ALTER TABLE corporate_treasuries 
ADD COLUMN IF NOT EXISTS owner_id UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_corporate_treasuries_owner_id ON corporate_treasuries(owner_id);

