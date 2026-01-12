package repository

import (
	"context"
	"fmt"

	"unwise-backend/database"
	"unwise-backend/models"
)

type UserRepository interface {
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Create(ctx context.Context, user *models.User) error
	Update(ctx context.Context, user *models.User) error
	UpdateAvatarURL(ctx context.Context, userID string, avatarURL string) error
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, query string) ([]models.User, error)
	GetUnclaimedPlaceholders(ctx context.Context) ([]models.User, error)
	ClaimPlaceholder(ctx context.Context, placeholderID, claimerID string) error
	WithTx(tx database.Querier) UserRepository
}

type userRepository struct {
	db *database.DB
	tx database.Querier
}

func NewUserRepository(db *database.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) WithTx(tx database.Querier) UserRepository {
	return &userRepository{db: r.db, tx: tx}
}

func (r *userRepository) getQuerier() database.Querier {
	if r.tx != nil {
		return r.tx
	}
	return r.db.Pool
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	var user models.User
	query := `SELECT id, COALESCE(email, ''), name, avatar_url, is_placeholder, claimed_by, claimed_at, created_at, updated_at 
	          FROM users WHERE id = $1`

	err := r.getQuerier().QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.IsPlaceholder,
		&user.ClaimedBy, &user.ClaimedAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return &user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	query := `SELECT id, COALESCE(email, ''), name, avatar_url, is_placeholder, claimed_by, claimed_at, created_at, updated_at 
	          FROM users WHERE email = $1`

	err := r.getQuerier().QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.IsPlaceholder,
		&user.ClaimedBy, &user.ClaimedAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting user by email: %w", err)
	}
	return &user, nil
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	query := `INSERT INTO users (id, email, name, avatar_url, is_placeholder, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
	          ON CONFLICT (id) DO UPDATE SET
	              email = EXCLUDED.email,
	              name = EXCLUDED.name,
	              avatar_url = EXCLUDED.avatar_url,
	              updated_at = NOW()`

	var email interface{} = user.Email
	if user.Email == "" {
		email = nil
	}

	_, err := r.getQuerier().Exec(ctx, query, user.ID, email, user.Name, user.AvatarURL, user.IsPlaceholder)
	if err != nil {
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	query := `UPDATE users SET name = $1, avatar_url = $2, updated_at = NOW()
	          WHERE id = $3`

	_, err := r.getQuerier().Exec(ctx, query, user.Name, user.AvatarURL, user.ID)
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}
	return nil
}

func (r *userRepository) UpdateAvatarURL(ctx context.Context, userID string, avatarURL string) error {
	query := `UPDATE users SET avatar_url = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.getQuerier().Exec(ctx, query, avatarURL, userID)
	if err != nil {
		return fmt.Errorf("updating user avatar: %w", err)
	}
	return nil
}

func (r *userRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = $1`
	_, err := r.getQuerier().Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	return nil
}

func (r *userRepository) Search(ctx context.Context, queryStr string) ([]models.User, error) {
	query := `
		SELECT id, COALESCE(email, ''), name, avatar_url, is_placeholder, claimed_by, claimed_at, created_at, updated_at
		FROM users
		WHERE email ILIKE '%' || $1 || '%' OR name ILIKE '%' || $1 || '%'
		LIMIT 10
	`
	rows, err := r.getQuerier().Query(ctx, query, queryStr)
	if err != nil {
		return nil, fmt.Errorf("searching users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.IsPlaceholder,
			&u.ClaimedBy, &u.ClaimedAt, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *userRepository) GetUnclaimedPlaceholders(ctx context.Context) ([]models.User, error) {
	query := `
		SELECT id, COALESCE(email, ''), name, avatar_url, is_placeholder, claimed_by, claimed_at, created_at, updated_at
		FROM users
		WHERE is_placeholder = TRUE AND claimed_by IS NULL
		ORDER BY name
	`
	rows, err := r.getQuerier().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("getting unclaimed placeholders: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.IsPlaceholder,
			&u.ClaimedBy, &u.ClaimedAt, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning placeholder: %w", err)
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *userRepository) ClaimPlaceholder(ctx context.Context, placeholderID, claimerID string) error {
	query := `UPDATE users SET claimed_by = $1, claimed_at = NOW(), updated_at = NOW() WHERE id = $2 AND is_placeholder = TRUE`
	_, err := r.getQuerier().Exec(ctx, query, claimerID, placeholderID)
	if err != nil {
		return fmt.Errorf("claiming placeholder: %w", err)
	}
	return nil
}
