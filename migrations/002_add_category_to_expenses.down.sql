DROP INDEX IF EXISTS idx_expenses_created_at;
DROP INDEX IF EXISTS idx_expenses_category;
ALTER TABLE expenses DROP COLUMN category;

