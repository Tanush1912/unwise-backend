package services

import (
	"context"
	"math"
	"time"

	"unwise-backend/database"
	apperrors "unwise-backend/errors"
	"unwise-backend/models"
	"unwise-backend/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ExpenseService interface {
	GetByID(ctx context.Context, expenseID, userID string) (*models.Expense, error)
	GetByGroupID(ctx context.Context, groupID, userID string) ([]models.Expense, error)
	Create(ctx context.Context, userID string, expense *models.Expense, splits []models.ExpenseSplit) (*models.Expense, error)
	Update(ctx context.Context, expenseID, userID string, expense *models.Expense, splits []models.ExpenseSplit) (*models.Expense, error)
	Delete(ctx context.Context, expenseID, userID string) error
}

type expenseService struct {
	expenseRepo repository.ExpenseRepository
	groupRepo   repository.GroupRepository
	db          *database.DB
}

func NewExpenseService(expenseRepo repository.ExpenseRepository, groupRepo repository.GroupRepository, db *database.DB) ExpenseService {
	return &expenseService{
		expenseRepo: expenseRepo,
		groupRepo:   groupRepo,
		db:          db,
	}
}

func (s *expenseService) GetByID(ctx context.Context, expenseID, userID string) (*models.Expense, error) {
	zap.L().Debug("Getting expense by ID", zap.String("expense_id", expenseID), zap.String("user_id", userID))
	expense, err := s.expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			zap.L().Debug("Expense not found", zap.String("expense_id", expenseID))
			return nil, apperrors.ExpenseNotFound()
		}
		zap.L().Error("Failed to get expense", zap.String("expense_id", expenseID), zap.Error(err))
		return nil, apperrors.DatabaseError("getting expense", err)
	}

	if err := RequireGroupMembership(ctx, s.groupRepo, expense.GroupID, userID); err != nil {
		return nil, err
	}

	return expense, nil
}

func (s *expenseService) GetByGroupID(ctx context.Context, groupID, userID string) ([]models.Expense, error) {
	zap.L().Debug("Getting expenses by group ID", zap.String("group_id", groupID), zap.String("user_id", userID))
	if err := RequireGroupMembership(ctx, s.groupRepo, groupID, userID); err != nil {
		return nil, err
	}

	expenses, err := s.expenseRepo.GetByGroupID(ctx, groupID)
	if err != nil {
		zap.L().Error("Failed to get group expenses", zap.String("group_id", groupID), zap.Error(err))
		return nil, apperrors.DatabaseError("getting expenses", err)
	}

	if expenses == nil {
		expenses = []models.Expense{}
	}
	return expenses, nil
}

func (s *expenseService) Create(ctx context.Context, userID string, expense *models.Expense, splits []models.ExpenseSplit) (*models.Expense, error) {
	if err := RequireGroupMembership(ctx, s.groupRepo, expense.GroupID, userID); err != nil {
		return nil, err
	}

	expense.ID = uuid.New().String()

	if expense.DateISO.IsZero() {
		expense.DateISO = time.Now()
		expense.Date = expense.DateISO.Format("2006-01-02")
		expense.Time = expense.DateISO.Format("15:04")
	}

	if expense.Category == "" {
		expense.Category = models.TransactionCategoryExpense
	}

	if expense.Type == "" {
		expense.Type = models.ExpenseTypeEqual
	}

	if len(expense.Payers) == 0 {
		if expense.PaidByUserID == nil {
			expense.PaidByUserID = &userID
		}
		expense.Payers = []models.ExpensePayer{
			{
				ID:         uuid.New().String(),
				ExpenseID:  expense.ID,
				UserID:     *expense.PaidByUserID,
				AmountPaid: expense.TotalAmount,
			},
		}
	}

	if err := s.validateExpenseAmounts(expense, splits); err != nil {
		return nil, err
	}

	err := s.db.WithTx(ctx, func(q database.Querier) error {
		txRepo := s.expenseRepo.WithTx(q)
		if err := txRepo.Create(ctx, expense); err != nil {
			return apperrors.DatabaseError("creating expense", err)
		}

		for i := range expense.Payers {
			expense.Payers[i].ID = uuid.New().String()
			expense.Payers[i].ExpenseID = expense.ID
			if err := txRepo.CreatePayer(ctx, &expense.Payers[i]); err != nil {
				return apperrors.DatabaseError("creating expense payer", err)
			}
		}

		for i := range splits {
			splits[i].ID = uuid.New().String()
			splits[i].ExpenseID = expense.ID
			if err := txRepo.CreateSplit(ctx, &splits[i]); err != nil {
				return apperrors.DatabaseError("creating expense split", err)
			}
		}

		for i := range expense.ReceiptItems {
			expense.ReceiptItems[i].ID = uuid.New().String()
			expense.ReceiptItems[i].ExpenseID = expense.ID
			if err := txRepo.CreateReceiptItem(ctx, &expense.ReceiptItems[i]); err != nil {
				return apperrors.DatabaseError("creating receipt item", err)
			}
			for j := range expense.ReceiptItems[i].Assignments {
				expense.ReceiptItems[i].Assignments[j].ID = uuid.New().String()
				expense.ReceiptItems[i].Assignments[j].ReceiptItemID = expense.ReceiptItems[i].ID
				if err := txRepo.CreateReceiptItemAssignment(ctx, &expense.ReceiptItems[i].Assignments[j]); err != nil {
					return apperrors.DatabaseError("creating receipt item assignment", err)
				}
			}
		}
		return nil
	})

	if err != nil {
		zap.L().Error("Failed to create expense transactionally", zap.String("group_id", expense.GroupID), zap.Error(err))
		return nil, err
	}

	zap.L().Info("Expense created successfully", zap.String("expense_id", expense.ID), zap.String("group_id", expense.GroupID), zap.Float64("amount", expense.TotalAmount))
	return s.expenseRepo.GetByID(ctx, expense.ID)
}

func (s *expenseService) Update(ctx context.Context, expenseID, userID string, expense *models.Expense, splits []models.ExpenseSplit) (*models.Expense, error) {
	zap.L().Info("Updating expense", zap.String("expense_id", expenseID), zap.String("user_id", userID))
	existingExpense, err := s.expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			return nil, apperrors.ExpenseNotFound()
		}
		zap.L().Error("Failed to find existing expense for update", zap.String("expense_id", expenseID), zap.Error(err))
		return nil, apperrors.DatabaseError("getting expense", err)
	}

	if err := RequireGroupMembership(ctx, s.groupRepo, existingExpense.GroupID, userID); err != nil {
		return nil, err
	}
	expense.ID = expenseID
	expense.GroupID = existingExpense.GroupID
	if expense.Category == "" {
		expense.Category = existingExpense.Category
	}

	if expense.Type == "" {
		expense.Type = existingExpense.Type
	}

	if expense.DateISO.IsZero() {
		expense.DateISO = existingExpense.DateISO
		expense.Date = existingExpense.Date
		expense.Time = existingExpense.Time
	}

	if len(expense.Payers) == 0 {
		if expense.PaidByUserID == nil && existingExpense.PaidByUserID != nil {
			expense.PaidByUserID = existingExpense.PaidByUserID
		}
		if expense.PaidByUserID != nil {
			expense.Payers = []models.ExpensePayer{
				{
					ID:         uuid.New().String(),
					ExpenseID:  expense.ID,
					UserID:     *expense.PaidByUserID,
					AmountPaid: expense.TotalAmount,
				},
			}
		}
	}

	if err := s.validateExpenseAmounts(expense, splits); err != nil {
		return nil, err
	}

	err = s.db.WithTx(ctx, func(q database.Querier) error {
		txRepo := s.expenseRepo.WithTx(q)

		if err := txRepo.Update(ctx, expense); err != nil {
			return apperrors.DatabaseError("updating expense", err)
		}

		if err := txRepo.DeletePayers(ctx, expenseID); err != nil {
			return apperrors.DatabaseError("deleting existing payers", err)
		}

		for i := range expense.Payers {
			expense.Payers[i].ID = uuid.New().String()
			expense.Payers[i].ExpenseID = expenseID
			if err := txRepo.CreatePayer(ctx, &expense.Payers[i]); err != nil {
				return apperrors.DatabaseError("creating expense payer", err)
			}
		}

		if err := txRepo.DeleteSplits(ctx, expenseID); err != nil {
			return apperrors.DatabaseError("deleting existing splits", err)
		}

		for i := range splits {
			splits[i].ID = uuid.New().String()
			splits[i].ExpenseID = expenseID
			if err := txRepo.CreateSplit(ctx, &splits[i]); err != nil {
				return apperrors.DatabaseError("creating expense split", err)
			}
		}

		if err := txRepo.DeleteReceiptItems(ctx, expenseID); err != nil {
			return apperrors.DatabaseError("deleting existing receipt items", err)
		}

		for i := range expense.ReceiptItems {
			expense.ReceiptItems[i].ID = uuid.New().String()
			expense.ReceiptItems[i].ExpenseID = expenseID
			if err := txRepo.CreateReceiptItem(ctx, &expense.ReceiptItems[i]); err != nil {
				return apperrors.DatabaseError("creating receipt item", err)
			}
			for j := range expense.ReceiptItems[i].Assignments {
				expense.ReceiptItems[i].Assignments[j].ID = uuid.New().String()
				expense.ReceiptItems[i].Assignments[j].ReceiptItemID = expense.ReceiptItems[i].ID
				if err := txRepo.CreateReceiptItemAssignment(ctx, &expense.ReceiptItems[i].Assignments[j]); err != nil {
					return apperrors.DatabaseError("creating receipt item assignment", err)
				}
			}
		}
		return nil
	})

	if err != nil {
		zap.L().Error("Failed to update expense transactionally", zap.String("expense_id", expenseID), zap.Error(err))
		return nil, err
	}

	zap.L().Info("Expense updated successfully", zap.String("expense_id", expenseID), zap.Float64("new_amount", expense.TotalAmount))
	return s.expenseRepo.GetByID(ctx, expenseID)
}

func (s *expenseService) validateExpenseAmounts(expense *models.Expense, splits []models.ExpenseSplit) error {
	totalPaid := 0.0
	for _, payer := range expense.Payers {
		totalPaid += payer.AmountPaid
	}
	roundedTotalPaid := math.Round(totalPaid*RoundingFactor) / RoundingFactor
	roundedTotalAmount := math.Round(expense.TotalAmount*RoundingFactor) / RoundingFactor

	if math.Abs(roundedTotalPaid-roundedTotalAmount) > AmountTolerance {
		zap.L().Warn("Expense validation failed: amount mismatch (payers)",
			zap.Float64("total_paid", roundedTotalPaid),
			zap.Float64("total_amount", roundedTotalAmount))
		return apperrors.AmountMismatch(roundedTotalPaid, roundedTotalAmount, "payer")
	}

	totalSplit := 0.0
	for _, split := range splits {
		totalSplit += split.Amount
	}
	roundedTotalSplit := math.Round(totalSplit*RoundingFactor) / RoundingFactor

	if math.Abs(roundedTotalSplit-roundedTotalAmount) > AmountTolerance {
		zap.L().Warn("Expense validation failed: amount mismatch (splits)",
			zap.Float64("total_split", roundedTotalSplit),
			zap.Float64("total_amount", roundedTotalAmount))
		return apperrors.AmountMismatch(roundedTotalSplit, roundedTotalAmount, "split")
	}

	return nil
}

func (s *expenseService) Delete(ctx context.Context, expenseID, userID string) error {
	zap.L().Info("Deleting expense", zap.String("expense_id", expenseID), zap.String("user_id", userID))
	expense, err := s.expenseRepo.GetByID(ctx, expenseID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			return apperrors.ExpenseNotFound()
		}
		zap.L().Error("Failed to find expense for deletion", zap.String("expense_id", expenseID), zap.Error(err))
		return apperrors.DatabaseError("getting expense", err)
	}

	if err := RequireGroupMembership(ctx, s.groupRepo, expense.GroupID, userID); err != nil {
		return err
	}

	if err := s.expenseRepo.Delete(ctx, expenseID); err != nil {
		zap.L().Error("Failed to delete expense record", zap.String("expense_id", expenseID), zap.Error(err))
		return apperrors.DatabaseError("deleting expense", err)
	}

	zap.L().Info("Expense deleted successfully", zap.String("expense_id", expenseID))
	return nil
}
