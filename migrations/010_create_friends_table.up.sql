CREATE TABLE friends (
    user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
    friend_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_id, friend_id),
    CONSTRAINT different_users CHECK (user_id != friend_id)
);

CREATE INDEX idx_friends_user_id ON friends(user_id);
CREATE INDEX idx_friends_friend_id ON friends(friend_id);
