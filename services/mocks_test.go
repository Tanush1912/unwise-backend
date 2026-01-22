package services

import (
	"context"
	"unwise-backend/database"
	"unwise-backend/models"
	"unwise-backend/repository"
)

type mockExpenseRepo struct {
	balances map[string]map[string]float64
}

func (m *mockExpenseRepo) GetByID(ctx context.Context, id string) (*models.Expense, error) {
	return nil, nil
}
func (m *mockExpenseRepo) GetByGroupID(ctx context.Context, groupID string) ([]models.Expense, error) {
	return nil, nil
}
func (m *mockExpenseRepo) GetTransactionsByGroupID(ctx context.Context, groupID string) ([]models.Transaction, error) {
	return nil, nil
}
func (m *mockExpenseRepo) GetRecentTransactionsForUser(ctx context.Context, userID string, limit int) ([]models.Expense, error) {
	return nil, nil
}
func (m *mockExpenseRepo) GetUserBalanceInGroup(ctx context.Context, groupID, userID string) (float64, error) {
	return 0, nil
}
func (m *mockExpenseRepo) GetUserTotalBalance(ctx context.Context, userID string) ([]models.CurrencyAmount, []models.CurrencyAmount, []models.CurrencyAmount, error) {
	return nil, nil, nil, nil
}
func (m *mockExpenseRepo) Create(ctx context.Context, expense *models.Expense) error { return nil }
func (m *mockExpenseRepo) Update(ctx context.Context, expense *models.Expense) error { return nil }
func (m *mockExpenseRepo) UpdateExplanation(ctx context.Context, id string, explanation string) error {
	return nil
}
func (m *mockExpenseRepo) Delete(ctx context.Context, id string) error { return nil }
func (m *mockExpenseRepo) GetSplits(ctx context.Context, expenseID string) ([]models.ExpenseSplit, error) {
	return nil, nil
}
func (m *mockExpenseRepo) CreateSplit(ctx context.Context, split *models.ExpenseSplit) error {
	return nil
}
func (m *mockExpenseRepo) DeleteSplits(ctx context.Context, expenseID string) error { return nil }
func (m *mockExpenseRepo) GetPayers(ctx context.Context, expenseID string) ([]models.ExpensePayer, error) {
	return nil, nil
}
func (m *mockExpenseRepo) CreatePayer(ctx context.Context, payer *models.ExpensePayer) error {
	return nil
}
func (m *mockExpenseRepo) DeletePayers(ctx context.Context, expenseID string) error { return nil }
func (m *mockExpenseRepo) GetReceiptItems(ctx context.Context, expenseID string) ([]models.ReceiptItem, error) {
	return nil, nil
}
func (m *mockExpenseRepo) CreateReceiptItem(ctx context.Context, item *models.ReceiptItem) error {
	return nil
}
func (m *mockExpenseRepo) GetReceiptItemAssignments(ctx context.Context, receiptItemID string) ([]models.ReceiptItemAssignment, error) {
	return nil, nil
}
func (m *mockExpenseRepo) CreateReceiptItemAssignment(ctx context.Context, assignment *models.ReceiptItemAssignment) error {
	return nil
}
func (m *mockExpenseRepo) DeleteReceiptItems(ctx context.Context, expenseID string) error { return nil }
func (m *mockExpenseRepo) GetSplitsByExpenseIDs(ctx context.Context, expenseIDs []string) (map[string][]models.ExpenseSplit, error) {
	return nil, nil
}
func (m *mockExpenseRepo) GetPayersByExpenseIDs(ctx context.Context, expenseIDs []string) (map[string][]models.ExpensePayer, error) {
	return nil, nil
}
func (m *mockExpenseRepo) GetGroupBalancesByUserID(ctx context.Context, userID string, groupIDs []string) (map[string]float64, error) {
	return nil, nil
}
func (m *mockExpenseRepo) GetGroupMemberBalances(ctx context.Context, groupID string) (map[string]map[string]float64, error) {
	return m.balances, nil
}
func (m *mockExpenseRepo) GetGroupTotalSpend(ctx context.Context, groupID string) (float64, error) {
	return 0, nil
}
func (m *mockExpenseRepo) GetPairwiseBalances(ctx context.Context, userID, friendID string, groupIDs []string) (map[string]float64, error) {
	return nil, nil
}
func (m *mockExpenseRepo) GetPairwiseBalancesAllFriends(ctx context.Context, userID string) (map[string]map[string]float64, error) {
	return nil, nil
}
func (m *mockExpenseRepo) TransferExpenses(ctx context.Context, fromUserID, toUserID string) error {
	return nil
}

func (m *mockExpenseRepo) WithTx(tx database.Querier) repository.ExpenseRepository { return m }

type mockGroupRepo struct{}

func (m *mockGroupRepo) IsMember(ctx context.Context, groupID, userID string) (bool, error) {
	return true, nil
}

func (m *mockGroupRepo) GetByID(ctx context.Context, id string) (*models.Group, error) {
	return nil, nil
}
func (m *mockGroupRepo) GetByUserID(ctx context.Context, userID string) ([]models.Group, error) {
	return nil, nil
}
func (m *mockGroupRepo) GetGroupsWithLastActivity(ctx context.Context, userID string) ([]models.DashboardGroup, error) {
	return nil, nil
}
func (m *mockGroupRepo) Create(ctx context.Context, group *models.Group) error { return nil }
func (m *mockGroupRepo) Update(ctx context.Context, group *models.Group) error { return nil }
func (m *mockGroupRepo) UpdateAvatarURL(ctx context.Context, groupID, avatarURL string) error {
	return nil
}
func (m *mockGroupRepo) UpdateDefaultCurrency(ctx context.Context, groupID, currency string) error {
	return nil
}
func (m *mockGroupRepo) Delete(ctx context.Context, id string) error { return nil }
func (m *mockGroupRepo) AddMember(ctx context.Context, groupID, userID string) error {
	return nil
}
func (m *mockGroupRepo) RemoveMember(ctx context.Context, groupID, userID string) error {
	return nil
}
func (m *mockGroupRepo) GetMembers(ctx context.Context, groupID string) ([]models.User, error) {
	return nil, nil
}
func (m *mockGroupRepo) GetCommonGroups(ctx context.Context, userID1, userID2 string) ([]models.Group, error) {
	return nil, nil
}
func (m *mockGroupRepo) GetGroupsDetailedByUserID(ctx context.Context, userID string) ([]models.Group, error) {
	return nil, nil
}
func (m *mockGroupRepo) WithTx(tx database.Querier) repository.GroupRepository { return m }
