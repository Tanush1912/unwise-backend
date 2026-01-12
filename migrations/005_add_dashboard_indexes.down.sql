-- Remove dashboard optimization indexes

DROP INDEX IF EXISTS idx_group_members_user_id;
DROP INDEX IF EXISTS idx_group_members_group_id;
DROP INDEX IF EXISTS idx_group_members_user_group;
DROP INDEX IF EXISTS idx_expenses_updated_at;
DROP INDEX IF EXISTS idx_expenses_group_created;
DROP INDEX IF EXISTS idx_expense_payers_user_id;
DROP INDEX IF EXISTS idx_expense_splits_user_id;
DROP INDEX IF EXISTS idx_expense_payers_expense_user;
DROP INDEX IF EXISTS idx_expense_splits_expense_user;

