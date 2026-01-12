ALTER TABLE groups ADD COLUMN type VARCHAR(20) DEFAULT 'OTHER' NOT NULL;
ALTER TABLE groups ADD CONSTRAINT groups_type_check CHECK (type IN ('TRIP', 'HOME', 'COUPLE', 'OTHER'));

CREATE INDEX idx_groups_type ON groups(type);

