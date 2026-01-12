package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"unwise-backend/config"
	"unwise-backend/database"
	"unwise-backend/models"

	"github.com/google/uuid"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	log.Println("Starting database seeding...")
	log.Println("NOTE: All test users will be created with password: TestPassword123!")

	if err := checkSchema(ctx, db); err != nil {
		log.Fatalf("Schema check failed: %v\nPlease run 'make migrate-up' to apply all migrations first.", err)
	}
	if err := clearDatabase(ctx, db); err != nil {
		log.Printf("Warning: Failed to clear database: %v", err)
		log.Println("Continuing with seeding...")
	}

	users, err := seedUsers(ctx, db, cfg)
	if err != nil {
		log.Fatalf("Failed to seed users: %v", err)
	}
	log.Printf("✓ Seeded %d users", len(users))

	groups, err := seedGroups(ctx, db, users)
	if err != nil {
		log.Fatalf("Failed to seed groups: %v", err)
	}
	log.Printf("✓ Seeded %d groups", len(groups))

	expenses, err := seedExpenses(ctx, db, groups, users)
	if err != nil {
		log.Fatalf("Failed to seed expenses: %v", err)
	}
	log.Printf("✓ Seeded %d expenses", len(expenses))

	receiptItems, err := seedReceiptItems(ctx, db, expenses, users)
	if err != nil {
		log.Fatalf("Failed to seed receipt items: %v", err)
	}
	log.Printf("✓ Seeded %d receipt items", len(receiptItems))

	log.Println("✓ Database seeding completed successfully!")
}

func checkSchema(ctx context.Context, db *database.DB) error {
	var tableExists bool
	checkTableQuery := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'expense_payers'
		)
	`
	if err := db.Pool.QueryRow(ctx, checkTableQuery).Scan(&tableExists); err != nil {
		return fmt.Errorf("failed to check schema: %w", err)
	}

	if !tableExists {
		return fmt.Errorf("expense_payers table does not exist - migration 004 has not been applied")
	}

	var constraintDef string
	checkConstraintQuery := `
		SELECT pg_get_constraintdef(oid) 
		FROM pg_constraint 
		WHERE conname = 'expenses_type_check' 
		AND conrelid = 'expenses'::regclass
	`
	if err := db.Pool.QueryRow(ctx, checkConstraintQuery).Scan(&constraintDef); err != nil {
		return fmt.Errorf("failed to check constraint: %w", err)
	}

	if !strings.Contains(constraintDef, "EXACT_AMOUNT") {
		return fmt.Errorf("expenses_type_check constraint does not include 'EXACT_AMOUNT' - migration 004 has not been applied")
	}

	return nil
}

func clearDatabase(ctx context.Context, db *database.DB) error {
	log.Println("Clearing existing data...")

	queries := []string{
		`DO $$ BEGIN
			IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'receipt_item_assignments') THEN
				DELETE FROM receipt_item_assignments;
			END IF;
		END $$;`,
		`DO $$ BEGIN
			IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'receipt_items') THEN
				DELETE FROM receipt_items;
			END IF;
		END $$;`,
		`DO $$ BEGIN
			IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'expense_payers') THEN
				DELETE FROM expense_payers;
			END IF;
		END $$;`,
		`DO $$ BEGIN
			IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'expense_splits') THEN
				DELETE FROM expense_splits;
			END IF;
		END $$;`,
		`DO $$ BEGIN
			IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'expenses') THEN
				DELETE FROM expenses;
			END IF;
		END $$;`,
		`DO $$ BEGIN
			IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'group_members') THEN
				DELETE FROM group_members;
			END IF;
		END $$;`,
		`DO $$ BEGIN
			IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'groups') THEN
				DELETE FROM groups;
			END IF;
		END $$;`,
		`DO $$ BEGIN
			IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'users') THEN
				DELETE FROM users;
			END IF;
		END $$;`,
	}

	for _, query := range queries {
		if _, err := db.Pool.Exec(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

func seedUsers(ctx context.Context, db *database.DB, cfg *config.Config) ([]models.User, error) {
	const testPassword = "TestPassword123!"

	users := []models.User{
		{
			ID:        uuid.New().String(),
			Email:     "alice@example.com",
			Name:      "Alice Johnson",
			AvatarURL: stringPtr("https://i.pravatar.cc/150?img=1"),
		},
		{
			ID:        uuid.New().String(),
			Email:     "bob@example.com",
			Name:      "Bob Smith",
			AvatarURL: stringPtr("https://i.pravatar.cc/150?img=2"),
		},
		{
			ID:        uuid.New().String(),
			Email:     "charlie@example.com",
			Name:      "Charlie Brown",
			AvatarURL: stringPtr("https://i.pravatar.cc/150?img=3"),
		},
		{
			ID:        uuid.New().String(),
			Email:     "diana@example.com",
			Name:      "Diana Prince",
			AvatarURL: stringPtr("https://i.pravatar.cc/150?img=4"),
		},
		{
			ID:        uuid.New().String(),
			Email:     "eve@example.com",
			Name:      "Eve Williams",
			AvatarURL: nil,
		},
		{
			ID:        uuid.New().String(),
			Email:     "frank@example.com",
			Name:      "Frank Miller",
			AvatarURL: stringPtr("https://i.pravatar.cc/150?img=5"),
		},
	}

	if cfg.SupabaseURL != "" && cfg.SupabaseServiceRoleKey != "" {
		log.Println("Creating users in Supabase Auth...")
		for _, user := range users {
			if err := createSupabaseAuthUser(ctx, cfg, user.ID, user.Email, user.Name, testPassword); err != nil {
				log.Printf("Warning: Failed to create Supabase Auth user for %s: %v", user.Email, err)
				log.Println("Continuing with database user creation...")
			}
		}
	} else {
		log.Println("Warning: Supabase credentials not configured. Skipping Supabase Auth user creation.")
		log.Println("Users will only be created in the database.")
		log.Println("To enable Supabase Auth user creation, set SUPABASE_URL and SUPABASE_SERVICE_ROLE_KEY in your .env file.")
	}

	now := time.Now()
	query := `
		INSERT INTO users (id, email, name, avatar_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	for _, user := range users {
		user.CreatedAt = now
		user.UpdatedAt = now
		if _, err := db.Pool.Exec(ctx, query,
			user.ID, user.Email, user.Name, user.AvatarURL, user.CreatedAt, user.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to insert user %s: %w", user.Email, err)
		}
	}

	log.Printf("✓ All users created with password: %s", testPassword)
	return users, nil
}

func createSupabaseAuthUser(ctx context.Context, cfg *config.Config, userID, email, name, password string) error {
	url := fmt.Sprintf("%s/auth/v1/admin/users", strings.TrimSuffix(cfg.SupabaseURL, "/"))

	payload := map[string]interface{}{
		"id":            userID,
		"email":         email,
		"password":      password,
		"email_confirm": true,
		"user_metadata": map[string]interface{}{
			"name": name,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", cfg.SupabaseServiceRoleKey)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.SupabaseServiceRoleKey))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errorBody map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorBody); err == nil {
			return fmt.Errorf("supabase API error (status %d): %v", resp.StatusCode, errorBody)
		}
		return fmt.Errorf("supabase API error: status %d", resp.StatusCode)
	}

	return nil
}

func seedGroups(ctx context.Context, db *database.DB, users []models.User) ([]models.Group, error) {
	groups := []models.Group{
		{
			ID:   uuid.New().String(),
			Name: "Summer Trip to Paris",
			Type: models.GroupTypeTrip,
		},
		{
			ID:   uuid.New().String(),
			Name: "Apartment Expenses",
			Type: models.GroupTypeHome,
		},
		{
			ID:   uuid.New().String(),
			Name: "Date Night Fund",
			Type: models.GroupTypeCouple,
		},
		{
			ID:   uuid.New().String(),
			Name: "Weekend Getaway",
			Type: models.GroupTypeOther,
		},
	}

	now := time.Now()
	groupQuery := `
		INSERT INTO groups (id, name, type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	memberQuery := `
		INSERT INTO group_members (group_id, user_id, created_at)
		VALUES ($1, $2, $3)
	`

	groups[0].CreatedAt = now
	groups[0].UpdatedAt = now
	if _, err := db.Pool.Exec(ctx, groupQuery,
		groups[0].ID, groups[0].Name, groups[0].Type, groups[0].CreatedAt, groups[0].UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to insert group: %w", err)
	}
	for _, user := range users {
		if _, err := db.Pool.Exec(ctx, memberQuery, groups[0].ID, user.ID, now); err != nil {
			return nil, fmt.Errorf("failed to add member to group: %w", err)
		}
	}

	groups[1].CreatedAt = now
	groups[1].UpdatedAt = now
	if _, err := db.Pool.Exec(ctx, groupQuery,
		groups[1].ID, groups[1].Name, groups[1].Type, groups[1].CreatedAt, groups[1].UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to insert group: %w", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := db.Pool.Exec(ctx, memberQuery, groups[1].ID, users[i].ID, now); err != nil {
			return nil, fmt.Errorf("failed to add member to group: %w", err)
		}
	}

	groups[2].CreatedAt = now
	groups[2].UpdatedAt = now
	if _, err := db.Pool.Exec(ctx, groupQuery,
		groups[2].ID, groups[2].Name, groups[2].Type, groups[2].CreatedAt, groups[2].UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to insert group: %w", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := db.Pool.Exec(ctx, memberQuery, groups[2].ID, users[i].ID, now); err != nil {
			return nil, fmt.Errorf("failed to add member to group: %w", err)
		}
	}

	groups[3].CreatedAt = now
	groups[3].UpdatedAt = now
	if _, err := db.Pool.Exec(ctx, groupQuery,
		groups[3].ID, groups[3].Name, groups[3].Type, groups[3].CreatedAt, groups[3].UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to insert group: %w", err)
	}
	for i := 2; i < 5; i++ {
		if _, err := db.Pool.Exec(ctx, memberQuery, groups[3].ID, users[i].ID, now); err != nil {
			return nil, fmt.Errorf("failed to add member to group: %w", err)
		}
	}

	return groups, nil
}

func seedExpenses(ctx context.Context, db *database.DB, groups []models.Group, users []models.User) ([]models.Expense, error) {
	now := time.Now()
	expenses := []models.Expense{}

	expense1 := models.Expense{
		ID:           uuid.New().String(),
		GroupID:      groups[0].ID,
		PaidByUserID: stringPtr(users[0].ID),
		TotalAmount:  120.00,
		Description:  "Dinner at Le Bistro",
		Type:         models.ExpenseTypeEqual,
		Category:     models.TransactionCategoryExpense,
		CreatedAt:    now.Add(-5 * 24 * time.Hour),
		UpdatedAt:    now.Add(-5 * 24 * time.Hour),
	}
	expenses = append(expenses, expense1)

	expense2 := models.Expense{
		ID:           uuid.New().String(),
		GroupID:      groups[0].ID,
		PaidByUserID: stringPtr(users[1].ID),
		TotalAmount:  600.00,
		Description:  "Hotel booking for 3 nights",
		Type:         models.ExpenseTypePercentage,
		Category:     models.TransactionCategoryExpense,
		CreatedAt:    now.Add(-10 * 24 * time.Hour),
		UpdatedAt:    now.Add(-10 * 24 * time.Hour),
	}
	expenses = append(expenses, expense2)

	expense3 := models.Expense{
		ID:           uuid.New().String(),
		GroupID:      groups[1].ID,
		PaidByUserID: stringPtr(users[0].ID),
		TotalAmount:  85.50,
		Description:  "Weekly groceries",
		Type:         models.ExpenseTypeItemized,
		Category:     models.TransactionCategoryExpense,
		CreatedAt:    now.Add(-2 * 24 * time.Hour),
		UpdatedAt:    now.Add(-2 * 24 * time.Hour),
	}
	expenses = append(expenses, expense3)

	expense4 := models.Expense{
		ID:           uuid.New().String(),
		GroupID:      groups[1].ID,
		PaidByUserID: stringPtr(users[1].ID),
		TotalAmount:  150.00,
		Description:  "Electricity bill",
		Type:         models.ExpenseTypeExactAmount,
		Category:     models.TransactionCategoryExpense,
		CreatedAt:    now.Add(-7 * 24 * time.Hour),
		UpdatedAt:    now.Add(-7 * 24 * time.Hour),
	}
	expenses = append(expenses, expense4)

	expense5 := models.Expense{
		ID:           uuid.New().String(),
		GroupID:      groups[2].ID,
		PaidByUserID: stringPtr(users[0].ID),
		TotalAmount:  30.00,
		Description:  "Movie tickets",
		Type:         models.ExpenseTypeEqual,
		Category:     models.TransactionCategoryExpense,
		CreatedAt:    now.Add(-1 * 24 * time.Hour),
		UpdatedAt:    now.Add(-1 * 24 * time.Hour),
	}
	expenses = append(expenses, expense5)

	expense6 := models.Expense{
		ID:           uuid.New().String(),
		GroupID:      groups[0].ID,
		PaidByUserID: nil,
		TotalAmount:  45.00,
		Description:  "Taxi to airport",
		Type:         models.ExpenseTypeEqual,
		Category:     models.TransactionCategoryExpense,
		CreatedAt:    now.Add(-3 * 24 * time.Hour),
		UpdatedAt:    now.Add(-3 * 24 * time.Hour),
	}
	expenses = append(expenses, expense6)

	expense7 := models.Expense{
		ID:           uuid.New().String(),
		GroupID:      groups[0].ID,
		PaidByUserID: stringPtr(users[2].ID),
		TotalAmount:  50.00,
		Description:  "Repayment to Alice",
		Type:         models.ExpenseTypeEqual,
		Category:     models.TransactionCategoryRepayment,
		CreatedAt:    now.Add(-4 * 24 * time.Hour),
		UpdatedAt:    now.Add(-4 * 24 * time.Hour),
	}
	expenses = append(expenses, expense7)

	expense8 := models.Expense{
		ID:           uuid.New().String(),
		GroupID:      groups[3].ID,
		PaidByUserID: stringPtr(users[3].ID),
		TotalAmount:  75.00,
		Description:  "Sunday brunch",
		Type:         models.ExpenseTypeEqual,
		Category:     models.TransactionCategoryExpense,
		CreatedAt:    now.Add(-6 * 24 * time.Hour),
		UpdatedAt:    now.Add(-6 * 24 * time.Hour),
	}
	expenses = append(expenses, expense8)

	expenseQuery := `
		INSERT INTO expenses (id, group_id, paid_by_user_id, total_amount, description, receipt_image_url, type, category, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	for _, expense := range expenses {
		if _, err := db.Pool.Exec(ctx, expenseQuery,
			expense.ID, expense.GroupID, expense.PaidByUserID, expense.TotalAmount,
			expense.Description, expense.ReceiptImageURL, expense.Type, expense.Category,
			expense.CreatedAt, expense.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to insert expense: %w", err)
		}
	}

	if err := seedExpenseSplits(ctx, db, expenses, groups, users); err != nil {
		return nil, fmt.Errorf("failed to seed expense splits: %w", err)
	}

	if err := seedExpensePayers(ctx, db, expenses, users); err != nil {
		return nil, fmt.Errorf("failed to seed expense payers: %w", err)
	}

	return expenses, nil
}

func seedExpenseSplits(ctx context.Context, db *database.DB, expenses []models.Expense, groups []models.Group, users []models.User) error {
	splitQuery := `
		INSERT INTO expense_splits (id, expense_id, user_id, amount, percentage, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	for _, user := range users {
		split := models.ExpenseSplit{
			ID:        uuid.New().String(),
			ExpenseID: expenses[0].ID,
			UserID:    user.ID,
			Amount:    20.00,
			CreatedAt: expenses[0].CreatedAt,
			UpdatedAt: expenses[0].UpdatedAt,
		}
		if _, err := db.Pool.Exec(ctx, splitQuery,
			split.ID, split.ExpenseID, split.UserID, split.Amount, split.Percentage,
			split.CreatedAt, split.UpdatedAt,
		); err != nil {
			return fmt.Errorf("failed to insert expense split: %w", err)
		}
	}

	percentages := []float64{30.0, 25.0, 20.0, 15.0, 5.0, 5.0}
	amounts := []float64{180.00, 150.00, 120.00, 90.00, 30.00, 30.00}
	for i, user := range users {
		split := models.ExpenseSplit{
			ID:         uuid.New().String(),
			ExpenseID:  expenses[1].ID,
			UserID:     user.ID,
			Amount:     amounts[i],
			Percentage: floatPtr(percentages[i]),
			CreatedAt:  expenses[1].CreatedAt,
			UpdatedAt:  expenses[1].UpdatedAt,
		}
		if _, err := db.Pool.Exec(ctx, splitQuery,
			split.ID, split.ExpenseID, split.UserID, split.Amount, split.Percentage,
			split.CreatedAt, split.UpdatedAt,
		); err != nil {
			return fmt.Errorf("failed to insert expense split: %w", err)
		}
	}

	homeGroupUsers := users[:3]
	for _, user := range homeGroupUsers {
		split := models.ExpenseSplit{
			ID:        uuid.New().String(),
			ExpenseID: expenses[2].ID,
			UserID:    user.ID,
			Amount:    28.50,
			CreatedAt: expenses[2].CreatedAt,
			UpdatedAt: expenses[2].UpdatedAt,
		}
		if _, err := db.Pool.Exec(ctx, splitQuery,
			split.ID, split.ExpenseID, split.UserID, split.Amount, split.Percentage,
			split.CreatedAt, split.UpdatedAt,
		); err != nil {
			return fmt.Errorf("failed to insert expense split: %w", err)
		}
	}

	exactAmounts := []float64{60.00, 50.00, 40.00}
	for i, user := range homeGroupUsers {
		split := models.ExpenseSplit{
			ID:        uuid.New().String(),
			ExpenseID: expenses[3].ID,
			UserID:    user.ID,
			Amount:    exactAmounts[i],
			CreatedAt: expenses[3].CreatedAt,
			UpdatedAt: expenses[3].UpdatedAt,
		}
		if _, err := db.Pool.Exec(ctx, splitQuery,
			split.ID, split.ExpenseID, split.UserID, split.Amount, split.Percentage,
			split.CreatedAt, split.UpdatedAt,
		); err != nil {
			return fmt.Errorf("failed to insert expense split: %w", err)
		}
	}

	coupleUsers := users[:2]
	for _, user := range coupleUsers {
		split := models.ExpenseSplit{
			ID:        uuid.New().String(),
			ExpenseID: expenses[4].ID,
			UserID:    user.ID,
			Amount:    15.00,
			CreatedAt: expenses[4].CreatedAt,
			UpdatedAt: expenses[4].UpdatedAt,
		}
		if _, err := db.Pool.Exec(ctx, splitQuery,
			split.ID, split.ExpenseID, split.UserID, split.Amount, split.Percentage,
			split.CreatedAt, split.UpdatedAt,
		); err != nil {
			return fmt.Errorf("failed to insert expense split: %w", err)
		}
	}

	for _, user := range users {
		split := models.ExpenseSplit{
			ID:        uuid.New().String(),
			ExpenseID: expenses[5].ID,
			UserID:    user.ID,
			Amount:    7.50,
			CreatedAt: expenses[5].CreatedAt,
			UpdatedAt: expenses[5].UpdatedAt,
		}
		if _, err := db.Pool.Exec(ctx, splitQuery,
			split.ID, split.ExpenseID, split.UserID, split.Amount, split.Percentage,
			split.CreatedAt, split.UpdatedAt,
		); err != nil {
			return fmt.Errorf("failed to insert expense split: %w", err)
		}
	}

	split := models.ExpenseSplit{
		ID:        uuid.New().String(),
		ExpenseID: expenses[6].ID,
		UserID:    users[0].ID,
		Amount:    50.00,
		CreatedAt: expenses[6].CreatedAt,
		UpdatedAt: expenses[6].UpdatedAt,
	}
	if _, err := db.Pool.Exec(ctx, splitQuery,
		split.ID, split.ExpenseID, split.UserID, split.Amount, split.Percentage,
		split.CreatedAt, split.UpdatedAt,
	); err != nil {
		return fmt.Errorf("failed to insert expense split: %w", err)
	}

	otherGroupUsers := users[2:5]
	for _, user := range otherGroupUsers {
		split := models.ExpenseSplit{
			ID:        uuid.New().String(),
			ExpenseID: expenses[7].ID,
			UserID:    user.ID,
			Amount:    25.00,
			CreatedAt: expenses[7].CreatedAt,
			UpdatedAt: expenses[7].UpdatedAt,
		}
		if _, err := db.Pool.Exec(ctx, splitQuery,
			split.ID, split.ExpenseID, split.UserID, split.Amount, split.Percentage,
			split.CreatedAt, split.UpdatedAt,
		); err != nil {
			return fmt.Errorf("failed to insert expense split: %w", err)
		}
	}

	return nil
}

func seedExpensePayers(ctx context.Context, db *database.DB, expenses []models.Expense, users []models.User) error {
	payerQuery := `
		INSERT INTO expense_payers (id, expense_id, user_id, amount_paid, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	for _, e := range expenses {
		if e.PaidByUserID != nil {
			expensePayer := models.ExpensePayer{
				ID:         uuid.New().String(),
				ExpenseID:  e.ID,
				UserID:     *e.PaidByUserID,
				AmountPaid: e.TotalAmount,
				CreatedAt:  e.CreatedAt,
			}
			if _, err := db.Pool.Exec(ctx, payerQuery,
				expensePayer.ID, expensePayer.ExpenseID, expensePayer.UserID,
				expensePayer.AmountPaid, expensePayer.CreatedAt,
			); err != nil {
				return fmt.Errorf("failed to insert single expense payer: %w", err)
			}
		} else if e.ID == expenses[5].ID {
			expense6Payers := []struct {
				userID     string
				amountPaid float64
			}{
				{users[0].ID, 22.50},
				{users[1].ID, 22.50},
			}

			for _, payer := range expense6Payers {
				expensePayer := models.ExpensePayer{
					ID:         uuid.New().String(),
					ExpenseID:  e.ID,
					UserID:     payer.userID,
					AmountPaid: payer.amountPaid,
					CreatedAt:  e.CreatedAt,
				}
				if _, err := db.Pool.Exec(ctx, payerQuery,
					expensePayer.ID, expensePayer.ExpenseID, expensePayer.UserID,
					expensePayer.AmountPaid, expensePayer.CreatedAt,
				); err != nil {
					return fmt.Errorf("failed to insert multi-payer expense payer: %w", err)
				}
			}
		}
	}

	return nil
}

func seedReceiptItems(ctx context.Context, db *database.DB, expenses []models.Expense, users []models.User) ([]models.ReceiptItem, error) {
	receiptItems := []models.ReceiptItem{}

	itemizedExpense := expenses[2]
	items := []struct {
		name  string
		price float64
	}{
		{"Milk", 4.50},
		{"Bread", 3.00},
		{"Eggs", 5.00},
		{"Chicken", 18.00},
		{"Rice", 8.00},
		{"Vegetables", 12.00},
		{"Fruits", 15.00},
		{"Snacks", 10.00},
		{"Beverages", 10.00},
	}

	itemQuery := `
		INSERT INTO receipt_items (id, expense_id, name, price, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	assignmentQuery := `
		INSERT INTO receipt_item_assignments (id, receipt_item_id, user_id, created_at)
		VALUES ($1, $2, $3, $4)
	`

	now := time.Now()
	totalPrice := 0.0

	for _, item := range items {
		receiptItem := models.ReceiptItem{
			ID:        uuid.New().String(),
			ExpenseID: itemizedExpense.ID,
			Name:      item.name,
			Price:     item.price,
			CreatedAt: itemizedExpense.CreatedAt,
		}
		receiptItems = append(receiptItems, receiptItem)
		totalPrice += item.price

		if _, err := db.Pool.Exec(ctx, itemQuery,
			receiptItem.ID, receiptItem.ExpenseID, receiptItem.Name, receiptItem.Price, receiptItem.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to insert receipt item: %w", err)
		}

		userIndex := len(receiptItems) % 3
		assignment := models.ReceiptItemAssignment{
			ID:            uuid.New().String(),
			ReceiptItemID: receiptItem.ID,
			UserID:        users[userIndex].ID,
			CreatedAt:     now,
		}

		if _, err := db.Pool.Exec(ctx, assignmentQuery,
			assignment.ID, assignment.ReceiptItemID, assignment.UserID, assignment.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to insert receipt item assignment: %w", err)
		}
	}

	log.Printf("  Created receipt items totaling $%.2f for itemized expense", totalPrice)

	return receiptItems, nil
}

func stringPtr(s string) *string {
	return &s
}

func floatPtr(f float64) *float64 {
	return &f
}
