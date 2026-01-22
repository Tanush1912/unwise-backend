-- Rollback: Remove multi-currency support

-- Drop indexes first
DROP INDEX IF EXISTS idx_expenses_group_currency;
DROP INDEX IF EXISTS idx_expenses_currency;
DROP INDEX IF EXISTS idx_groups_default_currency;

-- Drop foreign key constraints
ALTER TABLE expenses DROP CONSTRAINT IF EXISTS fk_expenses_currency;
ALTER TABLE groups DROP CONSTRAINT IF EXISTS fk_groups_default_currency;

-- Remove currency columns
ALTER TABLE expenses DROP COLUMN IF EXISTS currency;
ALTER TABLE groups DROP COLUMN IF EXISTS default_currency;

-- Drop currencies table
DROP TABLE IF EXISTS currencies;
