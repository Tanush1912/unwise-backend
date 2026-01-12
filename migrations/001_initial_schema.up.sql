CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    avatar_url TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE groups (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE group_members (
    group_id VARCHAR(255) REFERENCES groups(id) ON DELETE CASCADE,
    user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE expenses (
    id VARCHAR(255) PRIMARY KEY,
    group_id VARCHAR(255) REFERENCES groups(id) ON DELETE CASCADE NOT NULL,
    paid_by_user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE NOT NULL,
    total_amount DECIMAL(10, 2) NOT NULL,
    description TEXT NOT NULL,
    receipt_image_url TEXT,
    type VARCHAR(20) NOT NULL CHECK (type IN ('EQUAL', 'PERCENTAGE', 'ITEMIZED')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE expense_splits (
    id VARCHAR(255) PRIMARY KEY,
    expense_id VARCHAR(255) REFERENCES expenses(id) ON DELETE CASCADE NOT NULL,
    user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE NOT NULL,
    amount DECIMAL(10, 2) NOT NULL,
    percentage DECIMAL(5, 2),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE receipt_items (
    id VARCHAR(255) PRIMARY KEY,
    expense_id VARCHAR(255) REFERENCES expenses(id) ON DELETE CASCADE NOT NULL,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE receipt_item_assignments (
    id VARCHAR(255) PRIMARY KEY,
    receipt_item_id VARCHAR(255) REFERENCES receipt_items(id) ON DELETE CASCADE NOT NULL,
    user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (receipt_item_id, user_id)
);

CREATE INDEX idx_expenses_group_id ON expenses(group_id);
CREATE INDEX idx_expenses_paid_by_user_id ON expenses(paid_by_user_id);
CREATE INDEX idx_expense_splits_expense_id ON expense_splits(expense_id);
CREATE INDEX idx_expense_splits_user_id ON expense_splits(user_id);
CREATE INDEX idx_group_members_group_id ON group_members(group_id);
CREATE INDEX idx_group_members_user_id ON group_members(user_id);
CREATE INDEX idx_receipt_items_expense_id ON receipt_items(expense_id);
CREATE INDEX idx_receipt_item_assignments_receipt_item_id ON receipt_item_assignments(receipt_item_id);
CREATE INDEX idx_receipt_item_assignments_user_id ON receipt_item_assignments(user_id);

