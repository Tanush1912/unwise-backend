CREATE TABLE comments (
    id VARCHAR(255) PRIMARY KEY,
    expense_id VARCHAR(255) REFERENCES expenses(id) ON DELETE CASCADE NOT NULL,
    user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE comment_reactions (
    id VARCHAR(255) PRIMARY KEY,
    comment_id VARCHAR(255) REFERENCES comments(id) ON DELETE CASCADE NOT NULL,
    user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE NOT NULL,
    emoji VARCHAR(10) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (comment_id, user_id, emoji)
);

CREATE INDEX idx_comments_expense_id ON comments(expense_id);
CREATE INDEX idx_comment_reactions_comment_id ON comment_reactions(comment_id);

-- Enable Supabase Realtime for these tables
-- We check if the publication exists first to avoid errors if it was already created manually
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'supabase_realtime') THEN
        CREATE PUBLICATION supabase_realtime;
    END IF;
END
$$;

ALTER PUBLICATION supabase_realtime ADD TABLE comments, comment_reactions;
