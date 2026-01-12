-- Drop the existing constraint if it exists
ALTER TABLE expenses DROP CONSTRAINT IF EXISTS expenses_category_check;

-- Recreate the constraint with all three categories including PAYMENT
ALTER TABLE expenses ADD CONSTRAINT expenses_category_check CHECK (category IN ('EXPENSE', 'REPAYMENT', 'PAYMENT'));

