-- Rename 'date' from migration 011 to 'transaction_timestamp' for absolute sorting
ALTER TABLE expenses RENAME COLUMN date TO transaction_timestamp;

-- Add dedicated 'date_only' and 'time_only' columns for "Wall Clock" intent
-- We use TEXT for time to simplify formatting, or TIME type.
ALTER TABLE expenses ADD COLUMN date_only DATE;
ALTER TABLE expenses ADD COLUMN time_only TIME;

-- Update existing records to populate the new columns from the existing timestamp
UPDATE expenses SET 
    date_only = transaction_timestamp::DATE,
    time_only = transaction_timestamp::TIME;

-- Create index for sorting
DROP INDEX IF EXISTS idx_expenses_date;
CREATE INDEX idx_expenses_sorting ON expenses(transaction_timestamp DESC, created_at DESC);
