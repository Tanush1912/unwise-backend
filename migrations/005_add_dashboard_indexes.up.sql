-- Indexes for optimized dashboard queries

-- Index for group_members lookups (used in GetGroupsWithLastActivity)
CREATE INDEX IF NOT EXISTS idx_group_members_user_id ON group_members(user_id);
CREATE INDEX IF NOT EXISTS idx_group_members_group_id ON group_members(group_id);

-- Composite index for group_members queries
CREATE INDEX IF NOT EXISTS idx_group_members_user_group ON group_members(user_id, group_id);

-- Index for expenses updated_at (used in last activity sorting)
CREATE INDEX IF NOT EXISTS idx_expenses_updated_at ON expenses(updated_at DESC);

-- Index for expenses group_id and created_at (used in recent transactions)
CREATE INDEX IF NOT EXISTS idx_expenses_group_created ON expenses(group_id, created_at DESC);

-- Index for expense_payers user_id (used in balance calculations)
CREATE INDEX IF NOT EXISTS idx_expense_payers_user_id ON expense_payers(user_id);

-- Index for expense_splits user_id (used in balance calculations)
CREATE INDEX IF NOT EXISTS idx_expense_splits_user_id ON expense_splits(user_id);

-- Composite indexes for balance calculations
CREATE INDEX IF NOT EXISTS idx_expense_payers_expense_user ON expense_payers(expense_id, user_id);
CREATE INDEX IF NOT EXISTS idx_expense_splits_expense_user ON expense_splits(expense_id, user_id);

