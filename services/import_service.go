package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"unwise-backend/database"
	apperrors "unwise-backend/errors"
	"unwise-backend/models"
	"unwise-backend/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ImportService interface {
	PreviewSplitwiseCSV(ctx context.Context, groupID, userID string, file io.Reader) (*SplitwisePreviewResult, error)
	ImportSplitwiseCSV(ctx context.Context, groupID, userID string, file io.Reader, memberMapping map[string]*string) (*SplitwiseImportResult, error)
}

type importService struct {
	groupRepo   repository.GroupRepository
	userRepo    repository.UserRepository
	expenseRepo repository.ExpenseRepository
	db          *database.DB
}

func NewImportService(
	groupRepo repository.GroupRepository,
	userRepo repository.UserRepository,
	expenseRepo repository.ExpenseRepository,
	db *database.DB,
) ImportService {
	return &importService{
		groupRepo:   groupRepo,
		userRepo:    userRepo,
		expenseRepo: expenseRepo,
		db:          db,
	}
}

func (s *importService) requireMembership(ctx context.Context, groupID, userID string) error {
	return RequireGroupMembership(ctx, s.groupRepo, groupID, userID)
}

type SplitwisePreviewResult struct {
	CSVMembers        []string           `json:"csv_members"`
	GroupMembers      []models.User      `json:"group_members"`
	SuggestedMappings map[string]*string `json:"suggested_mappings"`
	ExpenseCount      int                `json:"expense_count"`
	PaymentCount      int                `json:"payment_count"`
	TotalAmount       float64            `json:"total_amount"`
}

type SplitwiseImportResult struct {
	Success             bool     `json:"success"`
	ImportedExpenses    int      `json:"imported_expenses"`
	ImportedPayments    int      `json:"imported_payments"`
	CreatedPlaceholders []string `json:"created_placeholders"`
	Errors              []string `json:"errors,omitempty"`
}

type SplitwiseRow struct {
	Date        time.Time
	Description string
	Category    string
	Cost        float64
	Currency    string
	Balances    map[string]float64
}

const (
	fixedColumnCount = 5
)

func (s *importService) PreviewSplitwiseCSV(ctx context.Context, groupID, userID string, file io.Reader) (*SplitwisePreviewResult, error) {
	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return nil, err
	}

	reader := csv.NewReader(file)

	header, err := reader.Read()
	if err != nil {
		return nil, apperrors.InvalidRequest("Failed to read CSV header: " + err.Error())
	}

	if len(header) < fixedColumnCount+1 {
		return nil, apperrors.InvalidRequest("CSV must have at least 6 columns (Date, Description, Category, Cost, Currency, and at least one member)")
	}

	csvMembers := header[fixedColumnCount:]

	for i, name := range csvMembers {
		csvMembers[i] = strings.TrimSpace(name)
	}

	expenseCount := 0
	paymentCount := 0
	totalAmount := 0.0

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		if len(row) < fixedColumnCount || strings.TrimSpace(row[0]) == "" {
			continue
		}

		if strings.Contains(strings.ToLower(row[1]), "total balance") {
			continue
		}

		category := strings.TrimSpace(row[2])
		cost, _ := strconv.ParseFloat(strings.TrimSpace(row[3]), 64)

		if strings.ToLower(category) == "payment" {
			paymentCount++
		} else {
			expenseCount++
			totalAmount += cost
		}
	}

	groupMembers, err := s.groupRepo.GetMembers(ctx, groupID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting group members", err)
	}

	suggestedMappings := make(map[string]*string)
	for _, csvMember := range csvMembers {
		suggestedMappings[csvMember] = nil

		csvNameLower := strings.ToLower(strings.TrimSpace(csvMember))
		for _, gm := range groupMembers {
			gmNameLower := strings.ToLower(strings.TrimSpace(gm.Name))
			if csvNameLower == gmNameLower {
				id := gm.ID
				suggestedMappings[csvMember] = &id
				break
			}
		}
	}

	return &SplitwisePreviewResult{
		CSVMembers:        csvMembers,
		GroupMembers:      groupMembers,
		SuggestedMappings: suggestedMappings,
		ExpenseCount:      expenseCount,
		PaymentCount:      paymentCount,
		TotalAmount:       totalAmount,
	}, nil
}

func (s *importService) ImportSplitwiseCSV(ctx context.Context, groupID, userID string, file io.Reader, memberMapping map[string]*string) (*SplitwiseImportResult, error) {
	zap.L().Info("Starting Splitwise CSV import",
		zap.String("group_id", groupID),
		zap.String("user_id", userID),
		zap.Int("mapping_count", len(memberMapping)))

	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return nil, err
	}

	reader := csv.NewReader(file)

	header, err := reader.Read()
	if err != nil {
		return nil, apperrors.InvalidRequest("Failed to read CSV header: " + err.Error())
	}

	if len(header) < fixedColumnCount+1 {
		return nil, apperrors.InvalidRequest("CSV must have at least 6 columns")
	}

	csvMembers := header[fixedColumnCount:]
	for i, name := range csvMembers {
		csvMembers[i] = strings.TrimSpace(name)
	}
	for _, csvMember := range csvMembers {
		if _, ok := memberMapping[csvMember]; !ok {
			return nil, apperrors.InvalidRequest(fmt.Sprintf("Member '%s' is not mapped", csvMember))
		}
	}

	result := &SplitwiseImportResult{
		Success:             true,
		CreatedPlaceholders: []string{},
		Errors:              []string{},
	}

	var rows []SplitwiseRow
	rowNum := 1
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: Failed to parse - %v", rowNum, err))
			continue
		}

		row, err := s.parseSplitwiseRow(record, csvMembers)
		if err != nil {
			if err.Error() != "skip" {
				result.Errors = append(result.Errors, fmt.Sprintf("Row %d: %v", rowNum, err))
			}
			continue
		}

		rows = append(rows, *row)
	}

	err = s.db.WithTx(ctx, func(q database.Querier) error {
		txGroupRepo := s.groupRepo.WithTx(q)
		txUserRepo := s.userRepo.WithTx(q)
		txExpenseRepo := s.expenseRepo.WithTx(q)
		resolvedMapping := make(map[string]string)

		for csvMember, userIDPtr := range memberMapping {
			if userIDPtr != nil && *userIDPtr != "" {
				resolvedMapping[csvMember] = *userIDPtr
			} else {
				placeholder := &models.User{
					ID:            uuid.New().String(),
					Name:          csvMember,
					IsPlaceholder: true,
				}
				if err := txUserRepo.Create(ctx, placeholder); err != nil {
					return fmt.Errorf("creating placeholder for '%s': %w", csvMember, err)
				}

				if err := txGroupRepo.AddMember(ctx, groupID, placeholder.ID); err != nil {
					return fmt.Errorf("adding placeholder '%s' to group: %w", csvMember, err)
				}

				resolvedMapping[csvMember] = placeholder.ID
				result.CreatedPlaceholders = append(result.CreatedPlaceholders, csvMember)
				zap.L().Info("Created placeholder user", zap.String("name", csvMember), zap.String("id", placeholder.ID))
			}
		}

		for i, row := range rows {
			if strings.ToLower(row.Category) == "payment" {
				err := s.importPaymentRow(ctx, txExpenseRepo, groupID, row, resolvedMapping)
				if err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("Row %d: %v", i+2, err))
					continue
				}
				result.ImportedPayments++
			} else {
				err := s.importExpenseRow(ctx, txExpenseRepo, groupID, row, resolvedMapping)
				if err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("Row %d: %v", i+2, err))
					continue
				}
				result.ImportedExpenses++
			}
		}

		return nil
	})

	if err != nil {
		zap.L().Error("Failed to import Splitwise CSV", zap.Error(err))
		return nil, apperrors.DatabaseError("importing CSV", err)
	}

	zap.L().Info("Splitwise CSV import completed",
		zap.Int("expenses", result.ImportedExpenses),
		zap.Int("payments", result.ImportedPayments),
		zap.Int("placeholders", len(result.CreatedPlaceholders)),
		zap.Int("errors", len(result.Errors)))

	return result, nil
}

func (s *importService) parseSplitwiseRow(record []string, memberNames []string) (*SplitwiseRow, error) {
	if len(record) < fixedColumnCount {
		return nil, fmt.Errorf("row has insufficient columns")
	}

	if strings.TrimSpace(record[0]) == "" {
		return nil, fmt.Errorf("skip")
	}
	if strings.Contains(strings.ToLower(record[1]), "total balance") {
		return nil, fmt.Errorf("skip")
	}

	dateStr := strings.TrimSpace(record[0])
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		date, err = time.Parse("01/02/2006", dateStr)
		if err != nil {
			date, err = time.Parse("2/1/2006", dateStr)
			if err != nil {
				return nil, fmt.Errorf("invalid date format: %s", dateStr)
			}
		}
	}

	costStr := strings.TrimSpace(record[3])
	cost, err := strconv.ParseFloat(costStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid cost: %s", costStr)
	}

	balances := make(map[string]float64)
	for i, memberName := range memberNames {
		colIndex := fixedColumnCount + i
		if colIndex >= len(record) {
			balances[memberName] = 0
			continue
		}

		balanceStr := strings.TrimSpace(record[colIndex])
		if balanceStr == "" {
			balances[memberName] = 0
			continue
		}

		balance, err := strconv.ParseFloat(balanceStr, 64)
		if err != nil {
			balances[memberName] = 0
			continue
		}
		balances[memberName] = balance
	}

	return &SplitwiseRow{
		Date:        date,
		Description: strings.TrimSpace(record[1]),
		Category:    strings.TrimSpace(record[2]),
		Cost:        cost,
		Currency:    strings.TrimSpace(record[4]),
		Balances:    balances,
	}, nil
}

func (s *importService) importExpenseRow(ctx context.Context, repo repository.ExpenseRepository, groupID string, row SplitwiseRow, memberMapping map[string]string) error {
	var payers []models.ExpensePayer
	var splits []models.ExpenseSplit

	expenseID := uuid.New().String()
	totalOwed := 0.0
	for _, balance := range row.Balances {
		if balance < -AmountTolerance {
			totalOwed += math.Abs(balance)
		}
	}

	payerShare := row.Cost - totalOwed

	for memberName, balance := range row.Balances {
		userID, ok := memberMapping[memberName]
		if !ok {
			continue
		}

		if balance > AmountTolerance {
			payers = append(payers, models.ExpensePayer{
				ID:         uuid.New().String(),
				ExpenseID:  expenseID,
				UserID:     userID,
				AmountPaid: balance + payerShare,
			})

			if payerShare > AmountTolerance {
				splits = append(splits, models.ExpenseSplit{
					ID:        uuid.New().String(),
					ExpenseID: expenseID,
					UserID:    userID,
					Amount:    payerShare,
				})
			}
		} else if balance < -AmountTolerance {
			splits = append(splits, models.ExpenseSplit{
				ID:        uuid.New().String(),
				ExpenseID: expenseID,
				UserID:    userID,
				Amount:    math.Abs(balance),
			})
		}
	}

	if len(payers) == 0 {
		var maxPayer string
		var maxBalance float64
		for memberName, balance := range row.Balances {
			if balance > maxBalance {
				maxBalance = balance
				maxPayer = memberName
			}
		}
		if maxPayer != "" {
			if userID, ok := memberMapping[maxPayer]; ok {
				payers = append(payers, models.ExpensePayer{
					ID:         uuid.New().String(),
					ExpenseID:  expenseID,
					UserID:     userID,
					AmountPaid: row.Cost,
				})
			}
		}
	}

	expense := &models.Expense{
		ID:          expenseID,
		GroupID:     groupID,
		TotalAmount: row.Cost,
		Description: row.Description,
		Type:        models.ExpenseTypeEqual,
		Category:    models.TransactionCategoryExpense,
		DateISO:     row.Date,
		Date:        row.Date.Format("2006-01-02"),
		Time:        "12:00",
		Payers:      payers,
	}

	if err := repo.Create(ctx, expense); err != nil {
		return fmt.Errorf("creating expense: %w", err)
	}

	for _, payer := range payers {
		if err := repo.CreatePayer(ctx, &payer); err != nil {
			return fmt.Errorf("creating payer: %w", err)
		}
	}

	for _, split := range splits {
		if err := repo.CreateSplit(ctx, &split); err != nil {
			return fmt.Errorf("creating split: %w", err)
		}
	}

	return nil
}

func (s *importService) importPaymentRow(ctx context.Context, repo repository.ExpenseRepository, groupID string, row SplitwiseRow, memberMapping map[string]string) error {
	expenseID := uuid.New().String()

	var payerID, receiverID string

	for memberName, balance := range row.Balances {
		userID, ok := memberMapping[memberName]
		if !ok {
			continue
		}

		if balance > AmountTolerance {
			payerID = userID
		} else if balance < -AmountTolerance {
			receiverID = userID
		}
	}

	if payerID == "" || receiverID == "" {
		return fmt.Errorf("could not determine payer and receiver for payment")
	}

	expense := &models.Expense{
		ID:          expenseID,
		GroupID:     groupID,
		TotalAmount: row.Cost,
		Description: row.Description,
		Type:        models.ExpenseTypeEqual,
		Category:    models.TransactionCategoryPayment,
		DateISO:     row.Date,
		Date:        row.Date.Format("2006-01-02"),
		Time:        "12:00",
	}

	payer := models.ExpensePayer{
		ID:         uuid.New().String(),
		ExpenseID:  expenseID,
		UserID:     payerID,
		AmountPaid: row.Cost,
	}

	split := models.ExpenseSplit{
		ID:        uuid.New().String(),
		ExpenseID: expenseID,
		UserID:    receiverID,
		Amount:    row.Cost,
	}

	if err := repo.Create(ctx, expense); err != nil {
		return fmt.Errorf("creating payment: %w", err)
	}

	if err := repo.CreatePayer(ctx, &payer); err != nil {
		return fmt.Errorf("creating payment payer: %w", err)
	}

	if err := repo.CreateSplit(ctx, &split); err != nil {
		return fmt.Errorf("creating payment split: %w", err)
	}

	return nil
}
