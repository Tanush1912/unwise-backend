package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	apperrors "unwise-backend/errors"
	"unwise-backend/models"
	"unwise-backend/repository"

	"go.uber.org/zap"
)

type UserService interface {
	DeleteAccount(ctx context.Context, userID string) error
	EnsureUser(ctx context.Context, userID, email, name string) (*models.User, error)
	UpdateAvatar(ctx context.Context, userID, avatarURL string) (*models.User, error)
	GetUser(ctx context.Context, userID string) (*models.User, error)
	GetClaimablePlaceholders(ctx context.Context, userID string) ([]models.User, error)
	ClaimPlaceholder(ctx context.Context, userID, placeholderID string) error
	AssignPlaceholder(ctx context.Context, placeholderID, targetUserID string) error
}

type userService struct {
	userRepo       repository.UserRepository
	expenseRepo    repository.ExpenseRepository
	supabaseURL    string
	serviceRoleKey string
}

func NewUserService(userRepo repository.UserRepository, expenseRepo repository.ExpenseRepository, supabaseURL, serviceRoleKey string) UserService {
	return &userService{
		userRepo:       userRepo,
		expenseRepo:    expenseRepo,
		supabaseURL:    supabaseURL,
		serviceRoleKey: serviceRoleKey,
	}
}

func (s *userService) GetUser(ctx context.Context, userID string) (*models.User, error) {
	zap.L().Debug("Getting user", zap.String("user_id", userID))
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			zap.L().Debug("User not found", zap.String("user_id", userID))
			return nil, apperrors.UserNotFound()
		}
		zap.L().Error("Failed to get user", zap.String("user_id", userID), zap.Error(err))
		return nil, apperrors.DatabaseError("getting user", err)
	}
	return user, nil
}

func (s *userService) UpdateAvatar(ctx context.Context, userID, avatarURL string) (*models.User, error) {
	zap.L().Info("Updating user avatar", zap.String("user_id", userID), zap.String("avatar_url", avatarURL))
	if err := s.userRepo.UpdateAvatarURL(ctx, userID, avatarURL); err != nil {
		zap.L().Error("Failed to update user avatar in DB", zap.String("user_id", userID), zap.Error(err))
		return nil, apperrors.DatabaseError("updating user avatar", err)
	}

	// Update Supabase Auth metadata
	if s.supabaseURL != "" && s.serviceRoleKey != "" {
		go func() {
			err := s.updateSupabaseMetadata(userID, avatarURL)
			if err != nil {
				zap.L().Error("Failed to update Supabase metadata", zap.String("user_id", userID), zap.Error(err))
			} else {
				zap.L().Info("Successfully updated Supabase metadata", zap.String("user_id", userID))
			}
		}()
	}

	return s.userRepo.GetByID(ctx, userID)
}

func (s *userService) updateSupabaseMetadata(userID, avatarURL string) error {
	url := fmt.Sprintf("%s/auth/v1/admin/users/%s", strings.TrimSuffix(s.supabaseURL, "/"), userID)

	data := map[string]interface{}{
		"user_metadata": map[string]interface{}{
			"avatar_url": avatarURL,
		},
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.serviceRoleKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase admin api returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (s *userService) DeleteAccount(ctx context.Context, userID string) error {
	zap.L().Info("Attempting account deletion", zap.String("user_id", userID))
	netBalance, totalOwe, totalOwed, err := s.expenseRepo.GetUserTotalBalance(ctx, userID)
	if err != nil {
		zap.L().Error("Failed to check user balance before deletion", zap.String("user_id", userID), zap.Error(err))
		return apperrors.DatabaseError("checking user balance before deletion", err)
	}

	if math.Abs(totalOwe) > BalanceThreshold || math.Abs(totalOwed) > BalanceThreshold || math.Abs(netBalance) > BalanceThreshold {
		zap.L().Warn("Account deletion rejected: active balance",
			zap.String("user_id", userID),
			zap.Float64("net_balance", netBalance),
			zap.Float64("total_owe", totalOwe),
			zap.Float64("total_owed", totalOwed))
		return apperrors.CannotDeleteAccountWithBalance()
	}

	if err := s.userRepo.Delete(ctx, userID); err != nil {
		zap.L().Error("Failed to delete user record", zap.String("user_id", userID), zap.Error(err))
		return apperrors.DatabaseError("deleting user account", err)
	}

	zap.L().Info("Account deleted successfully", zap.String("user_id", userID))
	return nil
}

func (s *userService) EnsureUser(ctx context.Context, userID, email, name string) (*models.User, error) {
	zap.L().Debug("Ensuring user record exists", zap.String("user_id", userID), zap.String("email", email))
	user, err := s.userRepo.GetByID(ctx, userID)
	if err == nil {
		return user, nil
	}

	zap.L().Info("User record not found, creating new record", zap.String("user_id", userID), zap.String("email", email))
	newUser := &models.User{
		ID:    userID,
		Email: email,
		Name:  name,
	}
	if newUser.Name == "" {
		newUser.Name = email
	}

	if err := s.userRepo.Create(ctx, newUser); err != nil {
		zap.L().Error("Failed to create user record", zap.String("user_id", userID), zap.Error(err))
		return nil, apperrors.DatabaseError("creating user record", err)
	}

	zap.L().Info("User record created successfully", zap.String("user_id", userID))
	return newUser, nil
}

func (s *userService) GetClaimablePlaceholders(ctx context.Context, userID string) ([]models.User, error) {
	zap.L().Debug("Getting claimable placeholders", zap.String("user_id", userID))

	placeholders, err := s.userRepo.GetUnclaimedPlaceholders(ctx)
	if err != nil {
		zap.L().Error("Failed to get unclaimed placeholders", zap.Error(err))
		return nil, apperrors.DatabaseError("getting unclaimed placeholders", err)
	}

	zap.L().Debug("Found claimable placeholders", zap.Int("count", len(placeholders)))
	return placeholders, nil
}

func (s *userService) ClaimPlaceholder(ctx context.Context, userID, placeholderID string) error {
	zap.L().Info("Claiming placeholder", zap.String("user_id", userID), zap.String("placeholder_id", placeholderID))

	placeholder, err := s.userRepo.GetByID(ctx, placeholderID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			return apperrors.UserNotFound()
		}
		return apperrors.DatabaseError("getting placeholder", err)
	}

	if !placeholder.IsPlaceholder {
		return apperrors.InvalidRequest("User is not a placeholder")
	}
	if placeholder.ClaimedBy != nil {
		return apperrors.InvalidRequest("Placeholder has already been claimed")
	}

	if err := s.userRepo.ClaimPlaceholder(ctx, placeholderID, userID); err != nil {
		zap.L().Error("Failed to claim placeholder", zap.String("placeholder_id", placeholderID), zap.Error(err))
		return apperrors.DatabaseError("claiming placeholder", err)
	}
	if err := s.expenseRepo.TransferExpenses(ctx, placeholderID, userID); err != nil {
		zap.L().Error("Failed to transfer expenses", zap.String("from", placeholderID), zap.String("to", userID), zap.Error(err))
		return apperrors.DatabaseError("transferring expenses", err)
	}

	zap.L().Info("Placeholder claimed successfully",
		zap.String("user_id", userID),
		zap.String("placeholder_id", placeholderID),
		zap.String("placeholder_name", placeholder.Name))

	return nil
}

func (s *userService) AssignPlaceholder(ctx context.Context, placeholderID, targetUserID string) error {
	zap.L().Info("Assigning placeholder", zap.String("placeholder_id", placeholderID), zap.String("target_user_id", targetUserID))

	placeholder, err := s.userRepo.GetByID(ctx, placeholderID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			return apperrors.UserNotFound()
		}
		return apperrors.DatabaseError("getting placeholder", err)
	}

	if !placeholder.IsPlaceholder {
		return apperrors.InvalidRequest("User is not a placeholder")
	}
	if placeholder.ClaimedBy != nil {
		return apperrors.InvalidRequest("Placeholder has already been claimed")
	}

	_, err = s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			return apperrors.InvalidRequest("Target user not found")
		}
		return apperrors.DatabaseError("getting target user", err)
	}

	if err := s.userRepo.ClaimPlaceholder(ctx, placeholderID, targetUserID); err != nil {
		zap.L().Error("Failed to assign placeholder", zap.String("placeholder_id", placeholderID), zap.Error(err))
		return apperrors.DatabaseError("assigning placeholder", err)
	}
	if err := s.expenseRepo.TransferExpenses(ctx, placeholderID, targetUserID); err != nil {
		zap.L().Error("Failed to transfer expenses", zap.String("from", placeholderID), zap.String("to", targetUserID), zap.Error(err))
		return apperrors.DatabaseError("transferring expenses", err)
	}

	zap.L().Info("Placeholder assigned successfully",
		zap.String("placeholder_id", placeholderID),
		zap.String("target_user_id", targetUserID),
		zap.String("placeholder_name", placeholder.Name))

	return nil
}
