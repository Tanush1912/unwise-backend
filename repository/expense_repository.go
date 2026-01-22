package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"

	"unwise-backend/database"
	"unwise-backend/models"
)

type ExpenseRepository interface {
	GetByID(ctx context.Context, id string) (*models.Expense, error)
	GetByGroupID(ctx context.Context, groupID string) ([]models.Expense, error)
	GetTransactionsByGroupID(ctx context.Context, groupID string) ([]models.Transaction, error)
	GetRecentTransactionsForUser(ctx context.Context, userID string, limit int) ([]models.Expense, error)
	GetUserBalanceInGroup(ctx context.Context, groupID, userID string) (float64, error)
	GetUserTotalBalance(ctx context.Context, userID string) ([]models.CurrencyAmount, []models.CurrencyAmount, []models.CurrencyAmount, error)
	Create(ctx context.Context, expense *models.Expense) error
	Update(ctx context.Context, expense *models.Expense) error
	UpdateExplanation(ctx context.Context, id string, explanation string) error
	Delete(ctx context.Context, id string) error
	GetSplits(ctx context.Context, expenseID string) ([]models.ExpenseSplit, error)
	CreateSplit(ctx context.Context, split *models.ExpenseSplit) error
	DeleteSplits(ctx context.Context, expenseID string) error
	GetPayers(ctx context.Context, expenseID string) ([]models.ExpensePayer, error)
	CreatePayer(ctx context.Context, payer *models.ExpensePayer) error
	DeletePayers(ctx context.Context, expenseID string) error
	GetReceiptItems(ctx context.Context, expenseID string) ([]models.ReceiptItem, error)
	CreateReceiptItem(ctx context.Context, item *models.ReceiptItem) error
	GetReceiptItemAssignments(ctx context.Context, receiptItemID string) ([]models.ReceiptItemAssignment, error)
	CreateReceiptItemAssignment(ctx context.Context, assignment *models.ReceiptItemAssignment) error
	DeleteReceiptItems(ctx context.Context, expenseID string) error
	GetSplitsByExpenseIDs(ctx context.Context, expenseIDs []string) (map[string][]models.ExpenseSplit, error)
	GetPayersByExpenseIDs(ctx context.Context, expenseIDs []string) (map[string][]models.ExpensePayer, error)
	GetGroupBalancesByUserID(ctx context.Context, userID string, groupIDs []string) (map[string]float64, error)
	GetGroupMemberBalances(ctx context.Context, groupID string) (map[string]map[string]float64, error)
	GetGroupTotalSpend(ctx context.Context, groupID string) (float64, error)
	GetPairwiseBalances(ctx context.Context, userID, friendID string, groupIDs []string) (map[string]float64, error)
	GetPairwiseBalancesAllFriends(ctx context.Context, userID string) (map[string]map[string]float64, error)
	TransferExpenses(ctx context.Context, fromUserID, toUserID string) error
	WithTx(tx database.Querier) ExpenseRepository
}

type expenseRepository struct {
	db *database.DB
	tx database.Querier
}

func NewExpenseRepository(db *database.DB) ExpenseRepository {
	return &expenseRepository{db: db}
}

func (r *expenseRepository) WithTx(tx database.Querier) ExpenseRepository {
	return &expenseRepository{db: r.db, tx: tx}
}

func (r *expenseRepository) getQuerier() database.Querier {
	if r.tx != nil {
		return r.tx
	}
	return r.db.Pool
}

func (r *expenseRepository) GetByID(ctx context.Context, id string) (*models.Expense, error) {
	var expense models.Expense
	query := `SELECT id, group_id, paid_by_user_id, total_amount, currency, description, 
	          receipt_image_url, type, category, tax, cgst, sgst, service_charge, explanation, created_at, updated_at, 
	          transaction_timestamp, date_only::TEXT, time_only::TEXT
	          FROM expenses WHERE id = $1`

	err := r.getQuerier().QueryRow(ctx, query, id).Scan(
		&expense.ID, &expense.GroupID, &expense.PaidByUserID, &expense.TotalAmount, &expense.Currency,
		&expense.Description, &expense.ReceiptImageURL, &expense.Type, &expense.Category,
		&expense.Tax, &expense.CGST, &expense.SGST, &expense.ServiceCharge, &expense.Explanation,
		&expense.CreatedAt, &expense.UpdatedAt, &expense.DateISO, &expense.Date, &expense.Time,
	)
	if err != nil {
		return nil, fmt.Errorf("getting expense by id: %w", err)
	}

	payers, err := r.GetPayers(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting expense payers: %w", err)
	}
	expense.Payers = payers

	splits, err := r.GetSplits(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting expense splits: %w", err)
	}
	expense.Splits = splits
	receiptItems, err := r.GetReceiptItems(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting receipt items: %w", err)
	}
	expense.ReceiptItems = receiptItems

	return &expense, nil
}

func (r *expenseRepository) GetByGroupID(ctx context.Context, groupID string) ([]models.Expense, error) {
	query := `SELECT id, group_id, paid_by_user_id, total_amount, currency, description,
	          receipt_image_url, type, category, tax, cgst, sgst, service_charge, explanation, created_at, updated_at, 
	          transaction_timestamp, date_only::TEXT, time_only::TEXT
	          FROM expenses WHERE group_id = $1
	          ORDER BY transaction_timestamp DESC, created_at DESC`

	rows, err := r.getQuerier().Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("getting expenses by group id: %w", err)
	}
	defer rows.Close()

	var expenses []models.Expense
	expenseIDs := make([]string, 0)
	for rows.Next() {
		var expense models.Expense
		if err := rows.Scan(
			&expense.ID, &expense.GroupID, &expense.PaidByUserID, &expense.TotalAmount, &expense.Currency,
			&expense.Description, &expense.ReceiptImageURL, &expense.Type, &expense.Category,
			&expense.Tax, &expense.CGST, &expense.SGST, &expense.ServiceCharge, &expense.Explanation,
			&expense.CreatedAt, &expense.UpdatedAt, &expense.DateISO, &expense.Date, &expense.Time,
		); err != nil {
			return nil, fmt.Errorf("scanning expense: %w", err)
		}
		expenses = append(expenses, expense)
		expenseIDs = append(expenseIDs, expense.ID)
	}

	if len(expenseIDs) > 0 {
		allSplits, err := r.GetSplitsByExpenseIDs(ctx, expenseIDs)
		if err != nil {
			return nil, fmt.Errorf("batch getting splits: %w", err)
		}

		allPayers, err := r.GetPayersByExpenseIDs(ctx, expenseIDs)
		if err != nil {
			return nil, fmt.Errorf("batch getting payers: %w", err)
		}

		allReceiptItems := make(map[string][]models.ReceiptItem)
		for _, expenseID := range expenseIDs {
			items, err := r.GetReceiptItems(ctx, expenseID)
			if err != nil {
				return nil, fmt.Errorf("getting receipt items for expense %s: %w", expenseID, err)
			}
			allReceiptItems[expenseID] = items
		}

		for i := range expenses {
			if splits := allSplits[expenses[i].ID]; splits != nil {
				expenses[i].Splits = splits
			} else {
				expenses[i].Splits = []models.ExpenseSplit{}
			}

			if payers := allPayers[expenses[i].ID]; payers != nil {
				expenses[i].Payers = payers
			} else {
				expenses[i].Payers = []models.ExpensePayer{}
			}

			if items := allReceiptItems[expenses[i].ID]; items != nil {
				expenses[i].ReceiptItems = items
			} else {
				expenses[i].ReceiptItems = []models.ReceiptItem{}
			}
		}
	}

	return expenses, nil
}

func (r *expenseRepository) Create(ctx context.Context, expense *models.Expense) error {
	category := expense.Category
	if category == "" {
		category = models.TransactionCategoryExpense
	}

	query := `INSERT INTO expenses (id, group_id, paid_by_user_id, total_amount, currency, description,
	          receipt_image_url, type, category, tax, cgst, sgst, service_charge, created_at, updated_at, transaction_timestamp, date_only, time_only)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW(), NOW(), $14, $15, $16)`

	_, err := r.getQuerier().Exec(ctx, query,
		expense.ID, expense.GroupID, expense.PaidByUserID, expense.TotalAmount, expense.Currency,
		expense.Description, expense.ReceiptImageURL, expense.Type, category,
		expense.Tax, expense.CGST, expense.SGST, expense.ServiceCharge, expense.DateISO, expense.Date, expense.Time,
	)
	if err != nil {
		return fmt.Errorf("creating expense: %w", err)
	}
	return nil
}

func (r *expenseRepository) Update(ctx context.Context, expense *models.Expense) error {
	query := `UPDATE expenses SET total_amount = $1, description = $2, 
	          receipt_image_url = $3, type = $4, category = $5, 
	          tax = $6, cgst = $7, sgst = $8, service_charge = $9, transaction_timestamp = $10, date_only = $11, time_only = $12, updated_at = NOW()
	          WHERE id = $13`

	_, err := r.getQuerier().Exec(ctx, query,
		expense.TotalAmount, expense.Description, expense.ReceiptImageURL,
		expense.Type, expense.Category,
		expense.Tax, expense.CGST, expense.SGST, expense.ServiceCharge, expense.DateISO, expense.Date, expense.Time, expense.ID,
	)
	if err != nil {
		return fmt.Errorf("updating expense: %w", err)
	}
	return nil
}

func (r *expenseRepository) UpdateExplanation(ctx context.Context, id string, explanation string) error {
	query := `UPDATE expenses SET explanation = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.getQuerier().Exec(ctx, query, explanation, id)
	if err != nil {
		return fmt.Errorf("updating expense explanation: %w", err)
	}
	return nil
}

func (r *expenseRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM expenses WHERE id = $1`

	_, err := r.getQuerier().Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting expense: %w", err)
	}
	return nil
}

func (r *expenseRepository) GetSplits(ctx context.Context, expenseID string) ([]models.ExpenseSplit, error) {
	query := `SELECT id, expense_id, user_id, amount, percentage, created_at, updated_at
	          FROM expense_splits WHERE expense_id = $1`

	rows, err := r.getQuerier().Query(ctx, query, expenseID)
	if err != nil {
		return nil, fmt.Errorf("getting expense splits: %w", err)
	}
	defer rows.Close()

	var splits []models.ExpenseSplit
	for rows.Next() {
		var split models.ExpenseSplit
		if err := rows.Scan(
			&split.ID, &split.ExpenseID, &split.UserID, &split.Amount,
			&split.Percentage, &split.CreatedAt, &split.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning expense split: %w", err)
		}
		splits = append(splits, split)
	}

	return splits, nil
}

func (r *expenseRepository) CreateSplit(ctx context.Context, split *models.ExpenseSplit) error {
	query := `INSERT INTO expense_splits (id, expense_id, user_id, amount, percentage, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, NOW(), NOW())`

	_, err := r.getQuerier().Exec(ctx, query,
		split.ID, split.ExpenseID, split.UserID, split.Amount, split.Percentage,
	)
	if err != nil {
		return fmt.Errorf("creating expense split: %w", err)
	}
	return nil
}

func (r *expenseRepository) DeleteSplits(ctx context.Context, expenseID string) error {
	query := `DELETE FROM expense_splits WHERE expense_id = $1`

	_, err := r.getQuerier().Exec(ctx, query, expenseID)
	if err != nil {
		return fmt.Errorf("deleting expense splits: %w", err)
	}
	return nil
}

func (r *expenseRepository) GetReceiptItems(ctx context.Context, expenseID string) ([]models.ReceiptItem, error) {
	query := `SELECT id, expense_id, name, price, created_at
	          FROM receipt_items WHERE expense_id = $1`

	rows, err := r.getQuerier().Query(ctx, query, expenseID)
	if err != nil {
		return nil, fmt.Errorf("getting receipt items: %w", err)
	}
	defer rows.Close()

	var items []models.ReceiptItem
	itemIDs := make([]string, 0)
	itemMap := make(map[string]*models.ReceiptItem)

	for rows.Next() {
		var item models.ReceiptItem
		if err := rows.Scan(
			&item.ID, &item.ExpenseID, &item.Name, &item.Price, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning receipt item: %w", err)
		}
		item.Assignments = []models.ReceiptItemAssignment{}
		items = append(items, item)
		itemIDs = append(itemIDs, item.ID)
	}

	if len(itemIDs) == 0 {
		return items, nil
	}

	for i := range items {
		itemMap[items[i].ID] = &items[i]
	}

	assignQuery := `SELECT id, receipt_item_id, user_id, created_at
	               FROM receipt_item_assignments WHERE receipt_item_id = ANY($1)`

	aRows, err := r.getQuerier().Query(ctx, assignQuery, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("getting batch receipt assignments: %w", err)
	}
	defer aRows.Close()

	for aRows.Next() {
		var a models.ReceiptItemAssignment
		if err := aRows.Scan(&a.ID, &a.ReceiptItemID, &a.UserID, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning assignment: %w", err)
		}
		if item, ok := itemMap[a.ReceiptItemID]; ok {
			item.Assignments = append(item.Assignments, a)
		}
	}

	return items, nil
}

func (r *expenseRepository) CreateReceiptItem(ctx context.Context, item *models.ReceiptItem) error {
	query := `INSERT INTO receipt_items (id, expense_id, name, price, created_at)
	          VALUES ($1, $2, $3, $4, NOW())`

	_, err := r.getQuerier().Exec(ctx, query, item.ID, item.ExpenseID, item.Name, item.Price)
	if err != nil {
		return fmt.Errorf("creating receipt item: %w", err)
	}
	return nil
}

func (r *expenseRepository) GetReceiptItemAssignments(ctx context.Context, receiptItemID string) ([]models.ReceiptItemAssignment, error) {
	query := `SELECT id, receipt_item_id, user_id, created_at
	          FROM receipt_item_assignments WHERE receipt_item_id = $1`

	rows, err := r.getQuerier().Query(ctx, query, receiptItemID)
	if err != nil {
		return nil, fmt.Errorf("getting receipt item assignments: %w", err)
	}
	defer rows.Close()

	var assignments []models.ReceiptItemAssignment
	for rows.Next() {
		var assignment models.ReceiptItemAssignment
		if err := rows.Scan(
			&assignment.ID, &assignment.ReceiptItemID, &assignment.UserID, &assignment.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning receipt item assignment: %w", err)
		}
		assignments = append(assignments, assignment)
	}

	return assignments, nil
}

func (r *expenseRepository) CreateReceiptItemAssignment(ctx context.Context, assignment *models.ReceiptItemAssignment) error {
	query := `INSERT INTO receipt_item_assignments (id, receipt_item_id, user_id, created_at)
	          VALUES ($1, $2, $3, NOW())`

	_, err := r.getQuerier().Exec(ctx, query, assignment.ID, assignment.ReceiptItemID, assignment.UserID)
	if err != nil {
		return fmt.Errorf("creating receipt item assignment: %w", err)
	}
	return nil
}

func (r *expenseRepository) GetTransactionsByGroupID(ctx context.Context, groupID string) ([]models.Transaction, error) {
	query := `SELECT e.id, e.group_id, e.paid_by_user_id, e.total_amount, e.description,
	          e.receipt_image_url, e.type, e.category, e.tax, e.cgst, e.sgst, e.service_charge, e.explanation,
	          e.created_at, e.updated_at, e.transaction_timestamp, e.date_only::TEXT, e.time_only::TEXT,
	          u.id, u.email, u.name, u.avatar_url, u.created_at, u.updated_at
	          FROM expenses e
	          LEFT JOIN users u ON e.paid_by_user_id = u.id
	          WHERE e.group_id = $1
	          ORDER BY e.transaction_timestamp DESC, e.created_at DESC`

	rows, err := r.getQuerier().Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("getting transactions by group id: %w", err)
	}
	defer rows.Close()

	var transactions []models.Transaction
	transactionIDs := make([]string, 0)

	for rows.Next() {
		var t models.Transaction
		var userID, userEmail, userName sql.NullString
		var userAvatarURL sql.NullString
		var userCreatedAt, userUpdatedAt sql.NullTime

		err := rows.Scan(
			&t.ID, &t.GroupID, &t.PaidByUserID, &t.TotalAmount,
			&t.Expense.Description, &t.ReceiptImageURL, &t.Expense.Type, &t.Category,
			&t.Tax, &t.CGST, &t.SGST, &t.ServiceCharge, &t.Explanation,
			&t.CreatedAt, &t.UpdatedAt, &t.DateISO, &t.Date, &t.Time,
			&userID, &userEmail, &userName, &userAvatarURL,
			&userCreatedAt, &userUpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning transaction: %w", err)
		}

		t.Type = string(t.Category)

		if userID.Valid {
			var avatarURL *string
			if userAvatarURL.Valid {
				avatarURL = &userAvatarURL.String
			}
			t.PaidByUser = &models.User{
				ID:        userID.String,
				Email:     userEmail.String,
				Name:      userName.String,
				AvatarURL: avatarURL,
				CreatedAt: userCreatedAt.Time,
				UpdatedAt: userUpdatedAt.Time,
			}
		}

		transactions = append(transactions, t)
		transactionIDs = append(transactionIDs, t.ID)
	}

	if len(transactionIDs) == 0 {
		return []models.Transaction{}, nil
	}

	allSplits, err := r.GetSplitsByExpenseIDs(ctx, transactionIDs)
	if err != nil {
		return nil, fmt.Errorf("batch getting splits: %w", err)
	}

	allPayers, err := r.GetPayersByExpenseIDs(ctx, transactionIDs)
	if err != nil {
		return nil, fmt.Errorf("batch getting payers: %w", err)
	}

	for i := range transactions {
		splits := allSplits[transactions[i].ID]
		if splits == nil {
			splits = []models.ExpenseSplit{}
		}
		transactions[i].Splits = splits

		payers := allPayers[transactions[i].ID]
		if payers == nil {
			payers = []models.ExpensePayer{}
		}
		transactions[i].Payers = payers
	}

	return transactions, nil
}

func (r *expenseRepository) GetPayers(ctx context.Context, expenseID string) ([]models.ExpensePayer, error) {
	query := `SELECT id, expense_id, user_id, amount_paid, created_at
	          FROM expense_payers WHERE expense_id = $1`

	rows, err := r.getQuerier().Query(ctx, query, expenseID)
	if err != nil {
		return nil, fmt.Errorf("getting expense payers: %w", err)
	}
	defer rows.Close()

	var payers []models.ExpensePayer
	for rows.Next() {
		var payer models.ExpensePayer
		if err := rows.Scan(
			&payer.ID, &payer.ExpenseID, &payer.UserID, &payer.AmountPaid, &payer.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning expense payer: %w", err)
		}
		payers = append(payers, payer)
	}

	return payers, nil
}

func (r *expenseRepository) CreatePayer(ctx context.Context, payer *models.ExpensePayer) error {
	query := `INSERT INTO expense_payers (id, expense_id, user_id, amount_paid, created_at)
	          VALUES ($1, $2, $3, $4, NOW())
	          ON CONFLICT (expense_id, user_id) DO UPDATE SET amount_paid = $4`

	_, err := r.getQuerier().Exec(ctx, query, payer.ID, payer.ExpenseID, payer.UserID, payer.AmountPaid)
	if err != nil {
		return fmt.Errorf("creating expense payer: %w", err)
	}
	return nil
}

func (r *expenseRepository) DeletePayers(ctx context.Context, expenseID string) error {
	query := `DELETE FROM expense_payers WHERE expense_id = $1`

	_, err := r.getQuerier().Exec(ctx, query, expenseID)
	if err != nil {
		return fmt.Errorf("deleting expense payers: %w", err)
	}
	return nil
}

func (r *expenseRepository) GetRecentTransactionsForUser(ctx context.Context, userID string, limit int) ([]models.Expense, error) {
	query := `SELECT DISTINCT e.id, e.group_id, e.paid_by_user_id, e.total_amount, e.description,
	          e.receipt_image_url, e.type, e.category, e.tax, e.cgst, e.sgst, e.service_charge, e.explanation,
	          e.created_at, e.updated_at, e.transaction_timestamp, e.date_only::TEXT, e.time_only::TEXT
	          FROM expenses e
	          INNER JOIN group_members gm ON e.group_id = gm.group_id
	          WHERE gm.user_id = $1
	          ORDER BY e.transaction_timestamp DESC, e.created_at DESC
	          LIMIT $2`

	rows, err := r.getQuerier().Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("getting recent transactions: %w", err)
	}
	defer rows.Close()

	var expenses []models.Expense
	for rows.Next() {
		var expense models.Expense
		if err := rows.Scan(
			&expense.ID, &expense.GroupID, &expense.PaidByUserID, &expense.TotalAmount,
			&expense.Description, &expense.ReceiptImageURL, &expense.Type, &expense.Category,
			&expense.Tax, &expense.CGST, &expense.SGST, &expense.ServiceCharge, &expense.Explanation,
			&expense.CreatedAt, &expense.UpdatedAt, &expense.DateISO, &expense.Date, &expense.Time,
		); err != nil {
			return nil, fmt.Errorf("scanning expense: %w", err)
		}
		expenses = append(expenses, expense)
	}

	return expenses, nil
}

func (r *expenseRepository) GetUserBalanceInGroup(ctx context.Context, groupID, userID string) (float64, error) {
	query := `SELECT 
	          COALESCE(SUM(p.amount_paid), 0) - COALESCE(SUM(s.amount), 0) as balance
	          FROM expenses e
	          LEFT JOIN expense_payers p ON e.id = p.expense_id AND p.user_id = $2
	          LEFT JOIN expense_splits s ON e.id = s.expense_id AND s.user_id = $2
	          WHERE e.group_id = $1`

	var balance float64
	err := r.getQuerier().QueryRow(ctx, query, groupID, userID).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("getting user balance in group: %w", err)
	}
	return balance, nil
}

func (r *expenseRepository) GetUserTotalBalance(ctx context.Context, userID string) ([]models.CurrencyAmount, []models.CurrencyAmount, []models.CurrencyAmount, error) {
	query := `
		WITH group_currency_nets AS (
			SELECT 
				e.group_id,
				e.currency,
				COALESCE(SUM(p.amount_paid), 0) - COALESCE(SUM(s.amount), 0) as balance
			FROM expenses e
			INNER JOIN group_members gm ON e.group_id = gm.group_id
			LEFT JOIN expense_payers p ON e.id = p.expense_id AND p.user_id = $1
			LEFT JOIN expense_splits s ON e.id = s.expense_id AND s.user_id = $1
			WHERE gm.user_id = $1
			GROUP BY e.group_id, e.currency
		)
		SELECT 
			currency,
			COALESCE(SUM(balance), 0) as total_net,
			COALESCE(SUM(CASE WHEN balance < -0.01 THEN ABS(balance) ELSE 0 END), 0) as total_owe,
			COALESCE(SUM(CASE WHEN balance > 0.01 THEN balance ELSE 0 END), 0) as total_owed
		FROM group_currency_nets
		GROUP BY currency
	`

	rows, err := r.getQuerier().Query(ctx, query, userID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("getting user total balance: %w", err)
	}
	defer rows.Close()

	var totalBalances, oweBalances, owedBalances []models.CurrencyAmount
	for rows.Next() {
		var currency string
		var totalNet, totalOwe, totalOwed float64
		if err := rows.Scan(&currency, &totalNet, &totalOwe, &totalOwed); err != nil {
			return nil, nil, nil, fmt.Errorf("scanning balance row: %w", err)
		}
		if math.Abs(totalNet) > 0.01 {
			totalBalances = append(totalBalances, models.CurrencyAmount{Currency: currency, Amount: totalNet})
		}
		if totalOwe > 0.01 {
			oweBalances = append(oweBalances, models.CurrencyAmount{Currency: currency, Amount: totalOwe})
		}
		if totalOwed > 0.01 {
			owedBalances = append(owedBalances, models.CurrencyAmount{Currency: currency, Amount: totalOwed})
		}
	}

	return totalBalances, oweBalances, owedBalances, nil
}

func (r *expenseRepository) DeleteReceiptItems(ctx context.Context, expenseID string) error {
	query := `DELETE FROM receipt_items WHERE expense_id = $1`

	_, err := r.getQuerier().Exec(ctx, query, expenseID)
	if err != nil {
		return fmt.Errorf("deleting receipt items: %w", err)
	}
	return nil
}

func (r *expenseRepository) GetSplitsByExpenseIDs(ctx context.Context, expenseIDs []string) (map[string][]models.ExpenseSplit, error) {
	if len(expenseIDs) == 0 {
		return make(map[string][]models.ExpenseSplit), nil
	}

	query := `SELECT id, expense_id, user_id, amount, percentage, created_at, updated_at
	          FROM expense_splits WHERE expense_id = ANY($1)`

	rows, err := r.getQuerier().Query(ctx, query, expenseIDs)
	if err != nil {
		return nil, fmt.Errorf("batch getting splits: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]models.ExpenseSplit)
	for rows.Next() {
		var split models.ExpenseSplit
		if err := rows.Scan(&split.ID, &split.ExpenseID, &split.UserID, &split.Amount, &split.Percentage, &split.CreatedAt, &split.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning split: %w", err)
		}
		result[split.ExpenseID] = append(result[split.ExpenseID], split)
	}
	return result, nil
}

func (r *expenseRepository) GetPayersByExpenseIDs(ctx context.Context, expenseIDs []string) (map[string][]models.ExpensePayer, error) {
	if len(expenseIDs) == 0 {
		return make(map[string][]models.ExpensePayer), nil
	}

	query := `SELECT id, expense_id, user_id, amount_paid, created_at
	          FROM expense_payers WHERE expense_id = ANY($1)`

	rows, err := r.getQuerier().Query(ctx, query, expenseIDs)
	if err != nil {
		return nil, fmt.Errorf("batch getting payers: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]models.ExpensePayer)
	for rows.Next() {
		var payer models.ExpensePayer
		if err := rows.Scan(&payer.ID, &payer.ExpenseID, &payer.UserID, &payer.AmountPaid, &payer.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning payer: %w", err)
		}
		result[payer.ExpenseID] = append(result[payer.ExpenseID], payer)
	}
	return result, nil
}

func (r *expenseRepository) GetGroupBalancesByUserID(ctx context.Context, userID string, groupIDs []string) (map[string]float64, error) {
	if len(groupIDs) == 0 {
		return make(map[string]float64), nil
	}

	query := `
		WITH user_payments AS (
			SELECT e.group_id, COALESCE(SUM(p.amount_paid), 0) as paid
			FROM expenses e
			JOIN expense_payers p ON e.id = p.expense_id
			WHERE e.group_id = ANY($2) AND p.user_id = $1
			GROUP BY e.group_id
		),
		user_splits AS (
			SELECT e.group_id, COALESCE(SUM(s.amount), 0) as owed
			FROM expenses e
			JOIN expense_splits s ON e.id = s.expense_id
			WHERE e.group_id = ANY($2) AND s.user_id = $1
			GROUP BY e.group_id
		)
		SELECT 
			COALESCE(up.group_id, us.group_id) as group_id,
			COALESCE(up.paid, 0) - COALESCE(us.owed, 0) as balance
		FROM user_payments up
		FULL OUTER JOIN user_splits us ON up.group_id = us.group_id
	`

	rows, err := r.getQuerier().Query(ctx, query, userID, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("batch getting group balances: %w", err)
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var groupID string
		var balance float64
		if err := rows.Scan(&groupID, &balance); err != nil {
			return nil, fmt.Errorf("scanning balance: %w", err)
		}
		result[groupID] = balance
	}
	return result, nil
}

func (r *expenseRepository) GetGroupMemberBalances(ctx context.Context, groupID string) (map[string]map[string]float64, error) {
	query := `
		WITH member_payments AS (
			SELECT e.currency, p.user_id, COALESCE(SUM(p.amount_paid), 0) as paid
			FROM expense_payers p
			JOIN expenses e ON e.id = p.expense_id
			WHERE e.group_id = $1
			GROUP BY e.currency, p.user_id
		),
		member_splits AS (
			SELECT e.currency, s.user_id, COALESCE(SUM(s.amount), 0) as owed
			FROM expense_splits s
			JOIN expenses e ON e.id = s.expense_id
			WHERE e.group_id = $1
			GROUP BY e.currency, s.user_id
		)
		SELECT 
			COALESCE(mp.user_id, ms.user_id) as user_id,
			COALESCE(mp.currency, ms.currency) as currency,
			COALESCE(mp.paid, 0) - COALESCE(ms.owed, 0) as balance
		FROM member_payments mp
		FULL OUTER JOIN member_splits ms ON mp.user_id = ms.user_id AND mp.currency = ms.currency
	`

	rows, err := r.getQuerier().Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("batch getting group member balances: %w", err)
	}
	defer rows.Close()

	result := make(map[string]map[string]float64)
	for rows.Next() {
		var userID, currency string
		var balance float64
		if err := rows.Scan(&userID, &currency, &balance); err != nil {
			return nil, fmt.Errorf("scanning member balance: %w", err)
		}
		if result[userID] == nil {
			result[userID] = make(map[string]float64)
		}
		result[userID][currency] = balance
	}
	return result, nil
}
func (r *expenseRepository) GetPairwiseBalances(ctx context.Context, userID, friendID string, groupIDs []string) (map[string]float64, error) {
	if len(groupIDs) == 0 {
		return make(map[string]float64), nil
	}

	query := `
		WITH group_balances AS (
			SELECT 
				e.group_id,
				u.user_id,
				COALESCE(SUM(p.amount_paid), 0) - COALESCE(SUM(s.amount), 0) as net_balance
			FROM expenses e
			CROSS JOIN (SELECT $1::text as user_id UNION SELECT $2::text as user_id) u
			LEFT JOIN expense_payers p ON e.id = p.expense_id AND p.user_id = u.user_id
			LEFT JOIN expense_splits s ON e.id = s.expense_id AND s.user_id = u.user_id
			WHERE e.group_id = ANY($3)
			GROUP BY e.group_id, u.user_id
		)
		SELECT 
			b1.group_id,
			b1.net_balance,
			b2.net_balance
		FROM group_balances b1
		JOIN group_balances b2 ON b1.group_id = b2.group_id
		WHERE b1.user_id = $1 AND b2.user_id = $2
	`

	rows, err := r.getQuerier().Query(ctx, query, userID, friendID, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("getting pairwise balances: %w", err)
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var groupID string
		var userNet, friendNet float64
		if err := rows.Scan(&groupID, &userNet, &friendNet); err != nil {
			return nil, fmt.Errorf("scanning pairwise balance: %w", err)
		}

		if userNet > 0.01 && friendNet < -0.01 {
			result[groupID] = math.Min(userNet, math.Abs(friendNet))
		} else if userNet < -0.01 && friendNet > 0.01 {
			result[groupID] = -math.Min(math.Abs(userNet), friendNet)
		} else {
			result[groupID] = 0
		}
	}
	return result, nil
}

func (r *expenseRepository) GetPairwiseBalancesAllFriends(ctx context.Context, userID string) (map[string]map[string]float64, error) {
	groupQuery := `SELECT group_id FROM group_members WHERE user_id = $1`
	groupRows, err := r.getQuerier().Query(ctx, groupQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("getting user groups: %w", err)
	}
	defer groupRows.Close()

	var groupIDs []string
	for groupRows.Next() {
		var gid string
		if err := groupRows.Scan(&gid); err != nil {
			return nil, fmt.Errorf("scanning group id: %w", err)
		}
		groupIDs = append(groupIDs, gid)
	}

	if len(groupIDs) == 0 {
		return make(map[string]map[string]float64), nil
	}

	friendQuery := `SELECT friend_id FROM friends WHERE user_id = $1`
	friendRows, err := r.getQuerier().Query(ctx, friendQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("getting friends: %w", err)
	}
	defer friendRows.Close()

	friendSet := make(map[string]bool)
	for friendRows.Next() {
		var fid string
		if err := friendRows.Scan(&fid); err != nil {
			return nil, fmt.Errorf("scanning friend id: %w", err)
		}
		friendSet[fid] = true
	}

	allGroupBalances, err := r.GetGroupMemberBalancesBatch(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("batch getting group member balances: %w", err)
	}

	result := make(map[string]map[string]float64)

	for _, groupID := range groupIDs {
		memberBalances := allGroupBalances[groupID]
		if memberBalances == nil {
			continue
		}

		type personBalance struct {
			userID  string
			balance float64
		}

		var creditors []personBalance
		var debtors []personBalance

		for uid, balance := range memberBalances {
			if balance > 0.01 {
				creditors = append(creditors, personBalance{uid, balance})
			} else if balance < -0.01 {
				debtors = append(debtors, personBalance{uid, math.Abs(balance)})
			}
		}

		sort.Slice(creditors, func(i, j int) bool {
			return creditors[i].userID < creditors[j].userID
		})
		sort.Slice(debtors, func(i, j int) bool {
			return debtors[i].userID < debtors[j].userID
		})

		pairwiseDebts := make(map[string]map[string]float64)

		for len(creditors) > 0 && len(debtors) > 0 {
			c := creditors[0]
			d := debtors[0]

			amount := math.Min(c.balance, d.balance)

			if _, exists := pairwiseDebts[d.userID]; !exists {
				pairwiseDebts[d.userID] = make(map[string]float64)
			}
			pairwiseDebts[d.userID][c.userID] = amount

			creditors[0].balance -= amount
			debtors[0].balance -= amount

			if creditors[0].balance < 0.01 {
				creditors = creditors[1:]
			}
			if debtors[0].balance < 0.01 {
				debtors = debtors[1:]
			}
		}

		for friendID := range friendSet {
			var balanceWithFriend float64

			if debts, ok := pairwiseDebts[friendID]; ok {
				if amount, ok := debts[userID]; ok {
					balanceWithFriend += amount
				}
			}

			if debts, ok := pairwiseDebts[userID]; ok {
				if amount, ok := debts[friendID]; ok {
					balanceWithFriend -= amount
				}
			}

			if math.Abs(balanceWithFriend) > 0.01 {
				if _, exists := result[friendID]; !exists {
					result[friendID] = make(map[string]float64)
				}
				result[friendID][groupID] = balanceWithFriend
			}
		}
	}

	return result, nil
}

func (r *expenseRepository) GetGroupMemberBalancesBatch(ctx context.Context, groupIDs []string) (map[string]map[string]float64, error) {
	if len(groupIDs) == 0 {
		return make(map[string]map[string]float64), nil
	}

	query := `
		WITH member_payments AS (
			SELECT e.group_id, p.user_id, COALESCE(SUM(p.amount_paid), 0) as paid
			FROM expense_payers p
			JOIN expenses e ON e.id = p.expense_id
			WHERE e.group_id = ANY($1)
			GROUP BY e.group_id, p.user_id
		),
		member_splits AS (
			SELECT e.group_id, s.user_id, COALESCE(SUM(s.amount), 0) as owed
			FROM expense_splits s
			JOIN expenses e ON e.id = s.expense_id
			WHERE e.group_id = ANY($1)
			GROUP BY e.group_id, s.user_id
		)
		SELECT 
			COALESCE(mp.group_id, ms.group_id) as group_id,
			COALESCE(mp.user_id, ms.user_id) as user_id,
			COALESCE(mp.paid, 0) - COALESCE(ms.owed, 0) as balance
		FROM member_payments mp
		FULL OUTER JOIN member_splits ms ON mp.group_id = ms.group_id AND mp.user_id = ms.user_id
	`

	rows, err := r.getQuerier().Query(ctx, query, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("batch getting group member balances: %w", err)
	}
	defer rows.Close()

	result := make(map[string]map[string]float64)
	for rows.Next() {
		var groupID, userID string
		var balance float64
		if err := rows.Scan(&groupID, &userID, &balance); err != nil {
			return nil, fmt.Errorf("scanning member balance: %w", err)
		}
		if _, exists := result[groupID]; !exists {
			result[groupID] = make(map[string]float64)
		}
		result[groupID][userID] = balance
	}
	return result, nil
}

func (r *expenseRepository) GetGroupTotalSpend(ctx context.Context, groupID string) (float64, error) {
	query := `SELECT COALESCE(SUM(total_amount), 0) FROM expenses WHERE group_id = $1 AND category = 'EXPENSE'`
	var total float64
	err := r.db.Pool.QueryRow(ctx, query, groupID).Scan(&total)
	return total, err
}

func (r *expenseRepository) TransferExpenses(ctx context.Context, fromUserID, toUserID string) error {
	payerQuery := `UPDATE expense_payers SET user_id = $1 WHERE user_id = $2`
	_, err := r.getQuerier().Exec(ctx, payerQuery, toUserID, fromUserID)
	if err != nil {
		return fmt.Errorf("transferring expense payers: %w", err)
	}

	splitQuery := `UPDATE expense_splits SET user_id = $1 WHERE user_id = $2`
	_, err = r.getQuerier().Exec(ctx, splitQuery, toUserID, fromUserID)
	if err != nil {
		return fmt.Errorf("transferring expense splits: %w", err)
	}

	expenseQuery := `UPDATE expenses SET paid_by_user_id = $1 WHERE paid_by_user_id = $2`
	_, err = r.getQuerier().Exec(ctx, expenseQuery, toUserID, fromUserID)
	if err != nil {
		return fmt.Errorf("transferring expenses paid_by: %w", err)
	}

	return nil
}
