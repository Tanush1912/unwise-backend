package services

import (
	"context"
	"fmt"
	"math"

	apperrors "unwise-backend/errors"
	"unwise-backend/models"
	"unwise-backend/repository"

	"go.uber.org/zap"
)

type DashboardService interface {
	GetDashboard(ctx context.Context, userID, email, name string) (*models.DashboardResponse, error)
}

type dashboardService struct {
	userRepo    repository.UserRepository
	groupRepo   repository.GroupRepository
	expenseRepo repository.ExpenseRepository
	userService UserService
}

func NewDashboardService(userRepo repository.UserRepository, groupRepo repository.GroupRepository, expenseRepo repository.ExpenseRepository, userService UserService) DashboardService {
	return &dashboardService{
		userRepo:    userRepo,
		groupRepo:   groupRepo,
		expenseRepo: expenseRepo,
		userService: userService,
	}
}

func (s *dashboardService) GetDashboard(ctx context.Context, userID, email, name string) (*models.DashboardResponse, error) {
	zap.L().Debug("Fetching dashboard data", zap.String("user_id", userID))
	user, err := s.userService.EnsureUser(ctx, userID, email, name)
	if err != nil {
		zap.L().Error("Failed to ensure user exists for dashboard", zap.String("user_id", userID), zap.Error(err))
		return nil, apperrors.InternalError(fmt.Errorf("ensuring user exists: %w", err))
	}

	totalNet, totalOwe, totalOwed, err := s.expenseRepo.GetUserTotalBalance(ctx, userID)
	if err != nil {
		zap.L().Error("Failed to get user total balance", zap.String("user_id", userID), zap.Error(err))
		return nil, apperrors.DatabaseError("getting user total balance", err)
	}

	groups, err := s.groupRepo.GetGroupsWithLastActivity(ctx, userID)
	if err != nil {
		zap.L().Error("Failed to get groups with last activity", zap.String("user_id", userID), zap.Error(err))
		return nil, apperrors.DatabaseError("getting groups with last activity", err)
	}

	groupIDs := make([]string, len(groups))
	for i, g := range groups {
		groupIDs[i] = g.ID
	}

	groupBalances, err := s.expenseRepo.GetGroupBalancesByUserID(ctx, userID, groupIDs)
	if err != nil {
		zap.L().Error("Failed to get group balances", zap.String("user_id", userID), zap.Error(err))
		return nil, apperrors.DatabaseError("getting group balances", err)
	}

	if groups == nil {
		groups = []models.DashboardGroup{}
	}

	for i := range groups {
		balance := groupBalances[groups[i].ID]
		groups[i].MyBalanceInGroup = math.Round(balance*RoundingFactor) / RoundingFactor
	}

	recentExpenses, err := s.expenseRepo.GetRecentTransactionsForUser(ctx, userID, RecentTransactionsLimit)
	if err != nil {
		zap.L().Error("Failed to get recent transactions", zap.String("user_id", userID), zap.Error(err))
		return nil, apperrors.DatabaseError("getting recent transactions", err)
	}

	expenseIDs := make([]string, len(recentExpenses))
	for i, e := range recentExpenses {
		expenseIDs[i] = e.ID
	}

	allPayers, err := s.expenseRepo.GetPayersByExpenseIDs(ctx, expenseIDs)
	if err != nil {
		zap.L().Error("Failed to batch get payers for recent activities", zap.String("user_id", userID), zap.Error(err))
		return nil, apperrors.DatabaseError("batch getting payers", err)
	}

	allSplits, err := s.expenseRepo.GetSplitsByExpenseIDs(ctx, expenseIDs)
	if err != nil {
		zap.L().Error("Failed to batch get splits for recent activities", zap.String("user_id", userID), zap.Error(err))
		return nil, apperrors.DatabaseError("batch getting splits", err)
	}

	recentActivity := make([]models.DashboardActivity, 0, len(recentExpenses))
	for _, expense := range recentExpenses {
		payers := allPayers[expense.ID]
		splits := allSplits[expense.ID]
		actionText := s.generateActionTextOptimized(expense, userID, payers, splits)

		recentActivity = append(recentActivity, models.DashboardActivity{
			ID:              expense.ID,
			Description:     expense.Description,
			Amount:          expense.TotalAmount,
			Type:            string(expense.Category),
			ActionText:      actionText,
			ReceiptImageURL: expense.ReceiptImageURL,
			CreatedAt:       expense.CreatedAt,
			Date:            expense.DateISO,
		})
	}

	zap.L().Info("Dashboard data fetched successfully",
		zap.String("user_id", userID),
		zap.Int("num_groups", len(groups)),
		zap.Float64("net_balance", totalNet))

	return &models.DashboardResponse{
		User: models.DashboardUserInfo{
			ID:        user.ID,
			Name:      user.Name,
			AvatarURL: user.AvatarURL,
		},
		Metrics: models.DashboardMetrics{
			TotalNetBalance: math.Round(totalNet*RoundingFactor) / RoundingFactor,
			TotalYouOwe:     math.Round(totalOwe*RoundingFactor) / RoundingFactor,
			TotalYouAreOwed: math.Round(totalOwed*RoundingFactor) / RoundingFactor,
		},
		Groups:         groups,
		RecentActivity: recentActivity,
	}, nil
}

func (s *dashboardService) generateActionTextOptimized(expense models.Expense, userID string, payers []models.ExpensePayer, splits []models.ExpenseSplit) string {
	var userPaidAmount float64
	for _, payer := range payers {
		if payer.UserID == userID {
			userPaidAmount += payer.AmountPaid
		}
	}

	var userShareAmount float64
	for _, split := range splits {
		if split.UserID == userID {
			userShareAmount += split.Amount
		}
	}

	netAmount := userPaidAmount - userShareAmount

	switch expense.Category {
	case models.TransactionCategoryPayment:
		if expense.PaidByUserID != nil && *expense.PaidByUserID == userID {
			return fmt.Sprintf("You paid $%.2f", expense.TotalAmount)
		}
		return fmt.Sprintf("You received $%.2f", expense.TotalAmount)

	case models.TransactionCategoryRepayment:
		if expense.PaidByUserID != nil && *expense.PaidByUserID == userID {
			return fmt.Sprintf("You repaid $%.2f", expense.TotalAmount)
		}
		return fmt.Sprintf("You received repayment of $%.2f", expense.TotalAmount)

	case models.TransactionCategoryExpense:
		if math.Abs(netAmount) < BalanceThreshold {
			return "You are settled"
		} else if netAmount > 0 {
			return fmt.Sprintf("You lent $%.2f", netAmount)
		}
		return fmt.Sprintf("You borrowed $%.2f", math.Abs(netAmount))

	default:
		return expense.Description
	}
}
