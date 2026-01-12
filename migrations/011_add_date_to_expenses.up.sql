ALTER TABLE expenses ADD COLUMN date TIMESTAMP WITH TIME ZONE DEFAULT NOW();

-- Also add an index for better sorting by date in timelines
CREATE INDEX idx_expenses_date ON expenses(date DESC);
