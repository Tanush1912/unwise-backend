DROP INDEX IF EXISTS idx_users_claimed_by;
DROP INDEX IF EXISTS idx_users_placeholder_unclaimed;
ALTER TABLE users DROP COLUMN IF EXISTS claimed_at;
ALTER TABLE users DROP COLUMN IF EXISTS claimed_by;
