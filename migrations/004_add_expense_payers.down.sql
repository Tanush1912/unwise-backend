DROP INDEX IF EXISTS idx_expense_payers_user_id;
DROP INDEX IF EXISTS idx_expense_payers_expense_id;
ALTER TABLE expenses ALTER COLUMN paid_by_user_id SET NOT NULL;
ALTER TABLE expenses ALTER COLUMN type DROP CONSTRAINT expenses_type_check;
ALTER TABLE expenses ADD CONSTRAINT expenses_type_check CHECK (type IN ('EQUAL', 'PERCENTAGE', 'ITEMIZED'));
DROP TABLE IF EXISTS expense_payers;

