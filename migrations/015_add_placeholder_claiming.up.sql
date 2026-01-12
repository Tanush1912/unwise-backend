-- Add columns to support placeholder user claiming
-- claimed_by: The real user ID that claimed this placeholder
-- claimed_at: When the placeholder was claimed

ALTER TABLE users ADD COLUMN claimed_by VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE users ADD COLUMN claimed_at TIMESTAMP;

-- Index for finding placeholders that can be claimed
CREATE INDEX idx_users_placeholder_unclaimed ON users(is_placeholder) WHERE is_placeholder = TRUE AND claimed_by IS NULL;

-- Index for finding which placeholders a user has claimed
CREATE INDEX idx_users_claimed_by ON users(claimed_by) WHERE claimed_by IS NOT NULL;
