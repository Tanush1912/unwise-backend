package repository

import (
	"context"
	"fmt"

	"unwise-backend/database"
	"unwise-backend/models"
)

type FriendRepository interface {
	Add(ctx context.Context, userID, friendID string) error
	Remove(ctx context.Context, userID, friendID string) error
	List(ctx context.Context, userID string) ([]models.User, error)
	IsFriend(ctx context.Context, userID, friendID string) (bool, error)
}

type friendRepository struct {
	db *database.DB
}

func NewFriendRepository(db *database.DB) FriendRepository {
	return &friendRepository{db: db}
}

func (r *friendRepository) Add(ctx context.Context, userID, friendID string) error {
	query := `INSERT INTO friends (user_id, friend_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.db.Pool.Exec(ctx, query, userID, friendID)
	if err != nil {
		return fmt.Errorf("adding friend: %w", err)
	}
	return nil
}

func (r *friendRepository) Remove(ctx context.Context, userID, friendID string) error {
	query := `DELETE FROM friends WHERE user_id = $1 AND friend_id = $2`
	_, err := r.db.Pool.Exec(ctx, query, userID, friendID)
	if err != nil {
		return fmt.Errorf("removing friend: %w", err)
	}
	return nil
}

func (r *friendRepository) List(ctx context.Context, userID string) ([]models.User, error) {
	query := `
		SELECT u.id, u.email, u.name, u.avatar_url, u.created_at, u.updated_at
		FROM users u
		JOIN friends f ON u.id = f.friend_id
		WHERE f.user_id = $1
		ORDER BY u.name ASC
	`
	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing friends: %w", err)
	}
	defer rows.Close()

	var friends []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning friend user: %w", err)
		}
		friends = append(friends, u)
	}
	return friends, nil
}

func (r *friendRepository) IsFriend(ctx context.Context, userID, friendID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM friends WHERE user_id = $1 AND friend_id = $2)`
	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, userID, friendID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking friendship: %w", err)
	}
	return exists, nil
}
