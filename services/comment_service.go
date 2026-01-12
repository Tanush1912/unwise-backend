package services

import (
	"context"

	apperrors "unwise-backend/errors"
	"unwise-backend/models"
	"unwise-backend/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type CommentService interface {
	AddComment(ctx context.Context, expenseID, userID, text string) (*models.Comment, error)
	GetComments(ctx context.Context, expenseID, userID string) ([]models.Comment, error)
	DeleteComment(ctx context.Context, commentID, userID string) error
	AddReaction(ctx context.Context, commentID, userID, emoji string) error
	RemoveReaction(ctx context.Context, commentID, userID, emoji string) error
}

type commentService struct {
	commentRepo repository.CommentRepository
	expenseRepo repository.ExpenseRepository
	groupRepo   repository.GroupRepository
}

func NewCommentService(
	commentRepo repository.CommentRepository,
	expenseRepo repository.ExpenseRepository,
	groupRepo repository.GroupRepository,
) CommentService {
	return &commentService{
		commentRepo: commentRepo,
		expenseRepo: expenseRepo,
		groupRepo:   groupRepo,
	}
}

func (s *commentService) checkAccess(ctx context.Context, expenseID, userID string) error {
	expense, err := s.expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			return apperrors.ExpenseNotFound()
		}
		return apperrors.DatabaseError("finding expense", err)
	}

	isMember, err := s.groupRepo.IsMember(ctx, expense.GroupID, userID)
	if err != nil {
		return apperrors.DatabaseError("checking membership", err)
	}
	if !isMember {
		return apperrors.NotGroupMember()
	}
	return nil
}

func (s *commentService) AddComment(ctx context.Context, expenseID, userID, text string) (*models.Comment, error) {
	if err := s.checkAccess(ctx, expenseID, userID); err != nil {
		return nil, err
	}

	comment := &models.Comment{
		ID:        uuid.New().String(),
		ExpenseID: expenseID,
		UserID:    userID,
		Text:      text,
	}

	if err := s.commentRepo.CreateComment(ctx, comment); err != nil {
		return nil, apperrors.DatabaseError("creating comment", err)
	}

	return comment, nil
}

func (s *commentService) GetComments(ctx context.Context, expenseID, userID string) ([]models.Comment, error) {
	if err := s.checkAccess(ctx, expenseID, userID); err != nil {
		return nil, err
	}

	comments, err := s.commentRepo.GetCommentsByExpenseID(ctx, expenseID)
	if err != nil {
		return nil, apperrors.DatabaseError("fetching comments", err)
	}

	return comments, nil
}

func (s *commentService) DeleteComment(ctx context.Context, commentID, userID string) error {
	comment, err := s.commentRepo.GetCommentByID(ctx, commentID)
	if err != nil {
		return apperrors.DatabaseError("finding comment", err)
	}

	if comment.UserID != userID {
		return apperrors.Unauthorized("You can only delete your own comments")
	}

	if err := s.commentRepo.DeleteComment(ctx, commentID); err != nil {
		return apperrors.DatabaseError("deleting comment", err)
	}
	return nil
}

func (s *commentService) AddReaction(ctx context.Context, commentID, userID, emoji string) error {
	comment, err := s.commentRepo.GetCommentByID(ctx, commentID)
	if err != nil {
		return apperrors.DatabaseError("finding comment", err)
	}

	if err := s.checkAccess(ctx, comment.ExpenseID, userID); err != nil {
		return err
	}

	reaction := &models.CommentReaction{
		ID:        uuid.New().String(),
		CommentID: commentID,
		UserID:    userID,
		Emoji:     emoji,
	}

	if err := s.commentRepo.AddReaction(ctx, reaction); err != nil {
		if apperrors.IsDuplicateError(err) {
			zap.L().Debug("Duplicate reaction ignored (idempotent)",
				zap.String("comment_id", commentID),
				zap.String("user_id", userID),
				zap.String("emoji", emoji))
			return nil
		}
		return apperrors.DatabaseError("adding reaction", err)
	}
	return nil
}

func (s *commentService) RemoveReaction(ctx context.Context, commentID, userID, emoji string) error {
	if err := s.commentRepo.RemoveReaction(ctx, commentID, userID, emoji); err != nil {
		return apperrors.DatabaseError("removing reaction", err)
	}
	return nil
}
