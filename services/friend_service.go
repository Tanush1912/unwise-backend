package services

import (
	"context"
	"math"

	apperrors "unwise-backend/errors"
	"unwise-backend/models"
	"unwise-backend/repository"

	"go.uber.org/zap"
)

type FriendService interface {
	AddFriendByEmail(ctx context.Context, userID, email string) error
	GetFriendsWithBalances(ctx context.Context, userID string) ([]models.FriendWithBalance, error)
	RemoveFriend(ctx context.Context, userID, friendID string) error
	SearchPotentialFriends(ctx context.Context, query string) ([]models.User, error)
}

type friendService struct {
	friendRepo        repository.FriendRepository
	userRepo          repository.UserRepository
	groupRepo         repository.GroupRepository
	expenseRepo       repository.ExpenseRepository
	settlementService SettlementService
}

func NewFriendService(friendRepo repository.FriendRepository, userRepo repository.UserRepository, groupRepo repository.GroupRepository, expenseRepo repository.ExpenseRepository, settlementService SettlementService) FriendService {
	return &friendService{
		friendRepo:        friendRepo,
		userRepo:          userRepo,
		groupRepo:         groupRepo,
		expenseRepo:       expenseRepo,
		settlementService: settlementService,
	}
}

func (s *friendService) SearchPotentialFriends(ctx context.Context, query string) ([]models.User, error) {
	if query == "" {
		return []models.User{}, nil
	}
	zap.L().Debug("Searching potential friends", zap.String("query", query))
	users, err := s.userRepo.Search(ctx, query)
	if err != nil {
		zap.L().Error("Failed to search potential friends", zap.String("query", query), zap.Error(err))
		return nil, apperrors.DatabaseError("searching users", err)
	}
	return users, nil
}

func (s *friendService) AddFriendByEmail(ctx context.Context, userID, email string) error {
	zap.L().Info("Adding friend by email", zap.String("user_id", userID), zap.String("friend_email", email))
	friendUser, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			zap.L().Debug("User not found by email while adding friend", zap.String("email", email))
			return apperrors.UserNotFoundByEmail(email)
		}
		zap.L().Error("Failed to find user by email", zap.String("email", email), zap.Error(err))
		return apperrors.DatabaseError("finding user by email", err)
	}
	if friendUser.ID == userID {
		zap.L().Warn("User tried to add themselves as friend", zap.String("user_id", userID))
		return apperrors.CannotAddSelf("add as a friend")
	}

	if err := s.friendRepo.Add(ctx, userID, friendUser.ID); err != nil {
		if apperrors.IsDuplicateError(err) {
			zap.L().Debug("Users are already friends", zap.String("user_id", userID), zap.String("friend_id", friendUser.ID))
			return apperrors.AlreadyFriends()
		}
		zap.L().Error("Failed to add friend mapping", zap.String("user_id", userID), zap.String("friend_id", friendUser.ID), zap.Error(err))
		return apperrors.DatabaseError("adding friend", err)
	}
	zap.L().Info("Friend added successfully", zap.String("user_id", userID), zap.String("friend_id", friendUser.ID))
	return nil
}

func (s *friendService) RemoveFriend(ctx context.Context, userID, friendID string) error {
	zap.L().Info("Removing friend", zap.String("user_id", userID), zap.String("friend_id", friendID))
	if err := s.friendRepo.Remove(ctx, userID, friendID); err != nil {
		zap.L().Error("Failed to remove friend mapping", zap.String("user_id", userID), zap.String("friend_id", friendID), zap.Error(err))
		return apperrors.DatabaseError("removing friend", err)
	}
	zap.L().Info("Friend removed successfully", zap.String("user_id", userID), zap.String("friend_id", friendID))
	return nil
}

func (s *friendService) GetFriendsWithBalances(ctx context.Context, userID string) ([]models.FriendWithBalance, error) {
	zap.L().Debug("Getting friends with balances", zap.String("user_id", userID))
	friends, err := s.friendRepo.List(ctx, userID)
	if err != nil {
		zap.L().Error("Failed to list friends", zap.String("user_id", userID), zap.Error(err))
		return nil, apperrors.DatabaseError("listing friends", err)
	}

	if len(friends) == 0 {
		return []models.FriendWithBalance{}, nil
	}

	friendSet := make(map[string]bool)
	for _, f := range friends {
		friendSet[f.ID] = true
	}

	userGroups, err := s.groupRepo.GetByUserID(ctx, userID)
	if err != nil {
		zap.L().Error("Failed to get user groups for friend balance calculation", zap.String("user_id", userID), zap.Error(err))
		return nil, apperrors.DatabaseError("getting user groups", err)
	}

	pairwiseBalances := make(map[string]map[string]map[string]float64)

	for _, group := range userGroups {
		settlements, err := s.settlementService.CalculateSettlements(ctx, group.ID, userID)
		if err != nil {
			zap.L().Warn("Failed to calculate settlements for group", zap.String("group_id", group.ID), zap.Error(err))
			continue
		}

		for _, settlement := range settlements {
			if settlement.ToUserID == userID && friendSet[settlement.FromUserID] {
				friendID := settlement.FromUserID
				if pairwiseBalances[friendID] == nil {
					pairwiseBalances[friendID] = make(map[string]map[string]float64)
				}
				if pairwiseBalances[friendID][group.ID] == nil {
					pairwiseBalances[friendID][group.ID] = make(map[string]float64)
				}
				pairwiseBalances[friendID][group.ID][settlement.Currency] += settlement.Amount
			}
			if settlement.FromUserID == userID && friendSet[settlement.ToUserID] {
				friendID := settlement.ToUserID
				if pairwiseBalances[friendID] == nil {
					pairwiseBalances[friendID] = make(map[string]map[string]float64)
				}
				if pairwiseBalances[friendID][group.ID] == nil {
					pairwiseBalances[friendID][group.ID] = make(map[string]float64)
				}
				pairwiseBalances[friendID][group.ID][settlement.Currency] -= settlement.Amount
			}
		}
	}

	results := make([]models.FriendWithBalance, 0, len(friends))

	for _, friend := range friends {
		friendGroupBalances := pairwiseBalances[friend.ID]
		commonGroups := make([]models.DashboardGroup, 0)
		groupBalances := make([]models.FriendGroupBalance, 0)

		currencyTotals := make(map[string]float64)

		for _, group := range userGroups {
			isMember := false
			for _, m := range group.Members {
				if m.ID == friend.ID {
					isMember = true
					break
				}
			}

			if isMember {
				commonGroups = append(commonGroups, models.DashboardGroup{
					ID:        group.ID,
					Name:      group.Name,
					AvatarURL: group.AvatarURL,
				})

				if groupCurrencyBalances, exists := friendGroupBalances[group.ID]; exists {
					for currency, balance := range groupCurrencyBalances {
						groupBalances = append(groupBalances, models.FriendGroupBalance{
							GroupID:   group.ID,
							GroupName: group.Name,
							Currency:  currency,
							Amount:    math.Round(balance*RoundingFactor) / RoundingFactor,
						})
						currencyTotals[currency] += balance
					}
				}
			}
		}

		balances := make([]models.CurrencyAmount, 0)
		var legacyNetBalance float64
		for currency, amount := range currencyTotals {
			roundedAmount := math.Round(amount*RoundingFactor) / RoundingFactor
			if math.Abs(roundedAmount) > BalanceThreshold {
				balances = append(balances, models.CurrencyAmount{
					Currency: currency,
					Amount:   roundedAmount,
				})
			}
			if currency == "INR" {
				legacyNetBalance = roundedAmount
			}
		}

		results = append(results, models.FriendWithBalance{
			UserInfo: models.UserInfo{
				ID:        friend.ID,
				Name:      friend.Name,
				AvatarURL: friend.AvatarURL,
			},
			Email:         friend.Email,
			NetBalance:    legacyNetBalance,
			Balances:      balances,
			Groups:        commonGroups,
			GroupBalances: groupBalances,
		})
	}

	return results, nil
}
