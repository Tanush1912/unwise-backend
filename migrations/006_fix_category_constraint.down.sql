-- Drop the constraint
ALTER TABLE expenses DROP CONSTRAINT IF EXISTS expenses_category_check;

-- Recreate with original values (if needed, though this migration is just a fix)
ALTER TABLE expenses ADD CONSTRAINT expenses_category_check CHECK (category IN ('EXPENSE', 'REPAYMENT', 'PAYMENT'));

