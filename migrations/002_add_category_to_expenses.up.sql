ALTER TABLE expenses ADD COLUMN category VARCHAR(20) DEFAULT 'EXPENSE' NOT NULL;
ALTER TABLE expenses ADD CONSTRAINT expenses_category_check CHECK (category IN ('EXPENSE', 'REPAYMENT', 'PAYMENT'));

CREATE INDEX idx_expenses_category ON expenses(category);
CREATE INDEX idx_expenses_created_at ON expenses(created_at DESC);

