package repository

import (
	"context"
	"fmt"

	"unwise-backend/database"
	"unwise-backend/models"
)

type CommentRepository interface {
	CreateComment(ctx context.Context, comment *models.Comment) error
	GetCommentsByExpenseID(ctx context.Context, expenseID string) ([]models.Comment, error)
	DeleteComment(ctx context.Context, commentID string) error
	AddReaction(ctx context.Context, reaction *models.CommentReaction) error
	RemoveReaction(ctx context.Context, commentID, userID, emoji string) error
	GetCommentByID(ctx context.Context, commentID string) (*models.Comment, error)
}

type commentRepository struct {
	db *database.DB
}

func NewCommentRepository(db *database.DB) CommentRepository {
	return &commentRepository{db: db}
}

func (r *commentRepository) CreateComment(ctx context.Context, comment *models.Comment) error {
	query := `
		INSERT INTO comments (id, expense_id, user_id, text, created_at)
		SELECT $1, $2::text, $3::text, $4, NOW()
		WHERE EXISTS (
			SELECT 1 FROM expenses e
			JOIN group_members gm ON gm.group_id = e.group_id
			WHERE e.id = $2 AND gm.user_id = $3
		)
		RETURNING id
	`
	var insertedID string
	err := r.db.Pool.QueryRow(ctx, query, comment.ID, comment.ExpenseID, comment.UserID, comment.Text).Scan(&insertedID)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return fmt.Errorf("user not authorized or expense not found")
		}
		return fmt.Errorf("creating comment: %w", err)
	}
	return nil
}

func (r *commentRepository) GetCommentByID(ctx context.Context, commentID string) (*models.Comment, error) {
	query := `SELECT id, expense_id, user_id, text, created_at FROM comments WHERE id = $1`
	var c models.Comment
	err := r.db.Pool.QueryRow(ctx, query, commentID).Scan(&c.ID, &c.ExpenseID, &c.UserID, &c.Text, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting comment: %w", err)
	}
	return &c, nil
}

func (r *commentRepository) GetCommentsByExpenseID(ctx context.Context, expenseID string) ([]models.Comment, error) {
	query := `
		SELECT c.id, c.expense_id, c.user_id, c.text, c.created_at,
		       u.id, u.name, u.email, u.avatar_url
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.expense_id = $1
		ORDER BY c.created_at ASC
	`
	rows, err := r.db.Pool.Query(ctx, query, expenseID)
	if err != nil {
		return nil, fmt.Errorf("querying comments: %w", err)
	}
	defer rows.Close()

	var comments []models.Comment
	commentMap := make(map[string]*models.Comment)

	for rows.Next() {
		var c models.Comment
		c.User = &models.User{}
		if err := rows.Scan(
			&c.ID, &c.ExpenseID, &c.UserID, &c.Text, &c.CreatedAt,
			&c.User.ID, &c.User.Name, &c.User.Email, &c.User.AvatarURL,
		); err != nil {
			return nil, fmt.Errorf("scanning comment: %w", err)
		}
		c.Reactions = []models.CommentReaction{}
		comments = append(comments, c)
	}

	for i := range comments {
		commentMap[comments[i].ID] = &comments[i]
	}

	if len(comments) == 0 {
		return []models.Comment{}, nil
	}

	commentIDs := make([]string, len(comments))
	for i, c := range comments {
		commentIDs[i] = c.ID
	}

	reactionQuery := `
		SELECT cr.id, cr.comment_id, cr.user_id, cr.emoji, cr.created_at,
		       u.id, u.name, u.email, u.avatar_url
		FROM comment_reactions cr
		JOIN users u ON cr.user_id = u.id
		WHERE cr.comment_id = ANY($1)
		ORDER BY cr.created_at ASC
	`
	rRows, err := r.db.Pool.Query(ctx, reactionQuery, commentIDs)
	if err != nil {
		return nil, fmt.Errorf("querying reactions: %w", err)
	}
	defer rRows.Close()

	for rRows.Next() {
		var r models.CommentReaction
		r.User = &models.User{}
		if err := rRows.Scan(
			&r.ID, &r.CommentID, &r.UserID, &r.Emoji, &r.CreatedAt,
			&r.User.ID, &r.User.Name, &r.User.Email, &r.User.AvatarURL,
		); err != nil {
			return nil, fmt.Errorf("scanning reaction: %w", err)
		}

		if parentComment, exists := commentMap[r.CommentID]; exists {
			parentComment.Reactions = append(parentComment.Reactions, r)
		}
	}

	return comments, nil
}

func (r *commentRepository) DeleteComment(ctx context.Context, commentID string) error {
	query := `DELETE FROM comments WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, commentID)
	if err != nil {
		return fmt.Errorf("deleting comment: %w", err)
	}
	return nil
}

func (r *commentRepository) AddReaction(ctx context.Context, reaction *models.CommentReaction) error {
	query := `INSERT INTO comment_reactions (id, comment_id, user_id, emoji, created_at)
	          VALUES ($1, $2, $3, $4, NOW())`
	_, err := r.db.Pool.Exec(ctx, query, reaction.ID, reaction.CommentID, reaction.UserID, reaction.Emoji)
	if err != nil {
		return fmt.Errorf("adding reaction: %w", err)
	}
	return nil
}

func (r *commentRepository) RemoveReaction(ctx context.Context, commentID, userID, emoji string) error {
	query := `DELETE FROM comment_reactions WHERE comment_id = $1 AND user_id = $2 AND emoji = $3`
	_, err := r.db.Pool.Exec(ctx, query, commentID, userID, emoji)
	if err != nil {
		return fmt.Errorf("removing reaction: %w", err)
	}
	return nil
}
