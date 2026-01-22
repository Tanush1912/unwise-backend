-- Migration: Add multi-currency support
-- This migration adds currency tracking at the group and expense level

-- Create currencies reference table
CREATE TABLE currencies (
    code VARCHAR(3) PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    symbol VARCHAR(5) NOT NULL
);

-- Seed supported currencies
INSERT INTO currencies (code, name, symbol) VALUES
    ('INR', 'Indian Rupee', '₹'),
    ('USD', 'US Dollar', '$'),
    ('EUR', 'Euro', '€'),
    ('GBP', 'British Pound', '£'),
    ('JPY', 'Japanese Yen', '¥'),
    ('CAD', 'Canadian Dollar', 'C$'),
    ('AUD', 'Australian Dollar', 'A$'),
    ('CNY', 'Chinese Yuan', '¥'),
    ('THB', 'Thai Baht', '฿'),
    ('SGD', 'Singapore Dollar', 'S$');

-- Add default_currency to groups (existing groups default to INR)
ALTER TABLE groups ADD COLUMN default_currency VARCHAR(3) NOT NULL DEFAULT 'INR';

-- Add currency to expenses (existing expenses default to INR)
ALTER TABLE expenses ADD COLUMN currency VARCHAR(3) NOT NULL DEFAULT 'INR';

-- Add foreign key constraints
ALTER TABLE groups ADD CONSTRAINT fk_groups_default_currency 
    FOREIGN KEY (default_currency) REFERENCES currencies(code);

ALTER TABLE expenses ADD CONSTRAINT fk_expenses_currency 
    FOREIGN KEY (currency) REFERENCES currencies(code);

-- Add indexes for efficient currency-based queries
CREATE INDEX idx_groups_default_currency ON groups(default_currency);
CREATE INDEX idx_expenses_currency ON expenses(currency);
CREATE INDEX idx_expenses_group_currency ON expenses(group_id, currency);
