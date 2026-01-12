CREATE TABLE expense_payers (
    id VARCHAR(255) PRIMARY KEY,
    expense_id VARCHAR(255) REFERENCES expenses(id) ON DELETE CASCADE NOT NULL,
    user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE NOT NULL,
    amount_paid DECIMAL(10, 2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (expense_id, user_id)
);

ALTER TABLE expenses ALTER COLUMN paid_by_user_id DROP NOT NULL;
ALTER TABLE expenses ALTER COLUMN type DROP CONSTRAINT expenses_type_check;
ALTER TABLE expenses ADD CONSTRAINT expenses_type_check CHECK (type IN ('EQUAL', 'PERCENTAGE', 'ITEMIZED', 'EXACT_AMOUNT'));

CREATE INDEX idx_expense_payers_expense_id ON expense_payers(expense_id);
CREATE INDEX idx_expense_payers_user_id ON expense_payers(user_id);

