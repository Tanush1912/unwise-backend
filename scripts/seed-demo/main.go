package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"unwise-backend/config"

	"github.com/google/uuid"
)

const (
	demoEmail    = "demo@unwise.app"
	demoPassword = "UnwiseDemo2026!"
	demoName     = "Demo User"

	demoUserID   = "a1b2c3d4-e5f6-4a7b-8c9d-000000000001"
	alexUserID   = "a1b2c3d4-e5f6-4a7b-8c9d-000000000002"
	priyaUserID  = "a1b2c3d4-e5f6-4a7b-8c9d-000000000003"
	marcusUserID = "a1b2c3d4-e5f6-4a7b-8c9d-000000000004"
	sofiaUserID  = "a1b2c3d4-e5f6-4a7b-8c9d-000000000005"

	flatmatesGroupID = "a1b2c3d4-e5f6-4a7b-8c9d-000000000010"
	barcelonaGroupID = "a1b2c3d4-e5f6-4a7b-8c9d-000000000011"
)

type apiClient struct {
	baseURL    string
	serviceKey string
	httpClient *http.Client
}

func newAPIClient(cfg *config.Config) *apiClient {
	return &apiClient{
		baseURL:    strings.TrimSuffix(cfg.SupabaseURL, "/"),
		serviceKey: cfg.SupabaseServiceRoleKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *apiClient) restDelete(table, filter string) error {
	url := fmt.Sprintf("%s/rest/v1/%s?%s", c.baseURL, table, filter)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DELETE %s: status %d: %s", table, resp.StatusCode, string(body))
	}
	return nil
}

func (c *apiClient) restInsert(table string, rows []map[string]interface{}) error {
	return c.restPost(table, rows, false)
}

func (c *apiClient) restUpsert(table string, rows []map[string]interface{}) error {
	return c.restPost(table, rows, true)
}

func (c *apiClient) restPost(table string, rows []map[string]interface{}, upsert bool) error {
	jsonData, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/rest/v1/%s", c.baseURL, table)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	c.setHeaders(req)
	if upsert {
		req.Header.Set("Prefer", "return=minimal,resolution=merge-duplicates")
	} else {
		req.Header.Set("Prefer", "return=minimal")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("INSERT %s: status %d: %s", table, resp.StatusCode, string(body))
	}
	return nil
}

func (c *apiClient) setHeaders(req *http.Request) {
	req.Header.Set("apikey", c.serviceKey)
	req.Header.Set("Authorization", "Bearer "+c.serviceKey)
	req.Header.Set("Content-Type", "application/json")
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.SupabaseURL == "" || cfg.SupabaseServiceRoleKey == "" {
		log.Fatalf("SUPABASE_URL and SUPABASE_SERVICE_ROLE_KEY must be set")
	}

	client := newAPIClient(cfg)
	ctx := context.Background()

	log.Println("Starting demo data seeding...")

	if err := cleanDemoData(client); err != nil {
		log.Printf("Warning: Failed to clean existing demo data: %v", err)
	}

	if err := ensureDemoAuthUser(ctx, client); err != nil {
		log.Printf("Warning: Supabase Auth user setup issue: %v", err)
		log.Println("Continuing with database seeding...")
	}

	if err := seedDemoUsers(client); err != nil {
		log.Fatalf("Failed to seed demo users: %v", err)
	}
	log.Println("✓ Seeded demo users")

	if err := seedDemoGroups(client); err != nil {
		log.Fatalf("Failed to seed demo groups: %v", err)
	}
	log.Println("✓ Seeded demo groups")

	if err := seedDemoExpenses(client); err != nil {
		log.Fatalf("Failed to seed demo expenses: %v", err)
	}
	log.Println("✓ Seeded demo expenses with splits and receipt items")

	log.Println("")
	log.Println("✓ Demo data seeding completed!")
	log.Printf("  Login: %s / %s", demoEmail, demoPassword)
}

func cleanDemoData(c *apiClient) error {
	log.Println("Cleaning existing demo data...")

	allGroupIDs := []string{flatmatesGroupID, barcelonaGroupID}
	allUserIDs := []string{demoUserID, alexUserID, priyaUserID, marcusUserID, sofiaUserID}

	groupFilter := fmt.Sprintf("id=in.(%s)", joinQuoted(allGroupIDs))
	if err := c.restDelete("groups", groupFilter); err != nil {
		return fmt.Errorf("delete groups: %w", err)
	}

	userFilter := fmt.Sprintf("id=in.(%s)", joinQuoted(allUserIDs))
	if err := c.restDelete("users", userFilter); err != nil {
		return fmt.Errorf("delete users: %w", err)
	}

	return nil
}

func joinQuoted(ids []string) string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf("%q", id)
	}
	return strings.Join(quoted, ",")
}

// --- Supabase Auth helpers ---

func ensureDemoAuthUser(ctx context.Context, c *apiClient) error {
	_ = deleteAuthUser(ctx, c, demoUserID)

	existingID, _ := findAuthUserByEmail(ctx, c, demoEmail)
	if existingID != "" && existingID != demoUserID {
		log.Printf("  Found existing auth user with different ID (%s), removing...", existingID)
		_ = deleteAuthUser(ctx, c, existingID)
	}

	if err := createAuthUser(ctx, c); err != nil {
		return fmt.Errorf("failed to create auth user: %w", err)
	}
	log.Println("✓ Created demo user in Supabase Auth")
	return nil
}

func createAuthUser(ctx context.Context, c *apiClient) error {
	url := fmt.Sprintf("%s/auth/v1/admin/users", c.baseURL)

	payload := map[string]interface{}{
		"id":            demoUserID,
		"email":         demoEmail,
		"password":      demoPassword,
		"email_confirm": true,
		"user_metadata": map[string]interface{}{
			"name": demoName,
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func deleteAuthUser(ctx context.Context, c *apiClient, userID string) error {
	url := fmt.Sprintf("%s/auth/v1/admin/users/%s", c.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func findAuthUserByEmail(ctx context.Context, c *apiClient, email string) (string, error) {
	url := fmt.Sprintf("%s/auth/v1/admin/users?per_page=100", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Users []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	for _, u := range result.Users {
		if u.Email == email {
			return u.ID, nil
		}
	}
	return "", nil
}

// --- Data seeding ---

func seedDemoUsers(c *apiClient) error {
	now := time.Now().UTC().Format(time.RFC3339)

	users := []map[string]interface{}{
		{"id": demoUserID, "email": demoEmail, "name": demoName, "is_placeholder": false, "created_at": now, "updated_at": now},
		{"id": alexUserID, "email": "alex.chen@example.com", "name": "Alex Chen", "is_placeholder": true, "created_at": now, "updated_at": now},
		{"id": priyaUserID, "email": "priya.sharma@example.com", "name": "Priya Sharma", "is_placeholder": true, "created_at": now, "updated_at": now},
		{"id": marcusUserID, "email": "marcus.johnson@example.com", "name": "Marcus Johnson", "is_placeholder": true, "created_at": now, "updated_at": now},
		{"id": sofiaUserID, "email": "sofia.garcia@example.com", "name": "Sofia Garcia", "is_placeholder": true, "created_at": now, "updated_at": now},
	}

	return c.restUpsert("users", users)
}

func seedDemoGroups(c *apiClient) error {
	now := time.Now().UTC().Format(time.RFC3339)

	groups := []map[string]interface{}{
		{"id": flatmatesGroupID, "name": "Flatmates", "type": "HOME", "default_currency": "INR", "created_at": now, "updated_at": now},
		{"id": barcelonaGroupID, "name": "Trip to Barcelona", "type": "TRIP", "default_currency": "EUR", "created_at": now, "updated_at": now},
	}
	if err := c.restInsert("groups", groups); err != nil {
		return fmt.Errorf("groups: %w", err)
	}

	members := []map[string]interface{}{
		{"group_id": flatmatesGroupID, "user_id": demoUserID, "created_at": now},
		{"group_id": flatmatesGroupID, "user_id": alexUserID, "created_at": now},
		{"group_id": flatmatesGroupID, "user_id": priyaUserID, "created_at": now},
		{"group_id": flatmatesGroupID, "user_id": marcusUserID, "created_at": now},
		{"group_id": barcelonaGroupID, "user_id": demoUserID, "created_at": now},
		{"group_id": barcelonaGroupID, "user_id": alexUserID, "created_at": now},
		{"group_id": barcelonaGroupID, "user_id": sofiaUserID, "created_at": now},
	}
	if err := c.restInsert("group_members", members); err != nil {
		return fmt.Errorf("group_members: %w", err)
	}

	return nil
}

func seedDemoExpenses(c *apiClient) error {
	now := time.Now()

	type expenseInfo struct {
		id, groupID, paidBy, currency, description, expType string
		amount                                              float64
		daysAgo                                             int
		receiptURL                                          string
	}

	groceryReceiptURL := "https://images.unsplash.com/photo-1604719312566-8912e9227c6a?w=400&h=600&fit=crop"

	expenses := []expenseInfo{
		{uuid.New().String(), flatmatesGroupID, demoUserID, "INR", "Monthly Rent", "EQUAL", 48000, 6, ""},
		{uuid.New().String(), flatmatesGroupID, alexUserID, "INR", "Weekly Groceries", "ITEMIZED", 3500, 4, groceryReceiptURL},
		{uuid.New().String(), flatmatesGroupID, priyaUserID, "INR", "WiFi & Internet", "EQUAL", 1200, 3, ""},
		{uuid.New().String(), flatmatesGroupID, demoUserID, "INR", "Dinner at Barbeque Nation", "PERCENTAGE", 3680, 2, ""},
		{uuid.New().String(), flatmatesGroupID, marcusUserID, "INR", "Cleaning Supplies", "EXACT_AMOUNT", 560, 1, ""},
		{uuid.New().String(), barcelonaGroupID, sofiaUserID, "EUR", "Flights to Barcelona", "EQUAL", 450, 10, ""},
		{uuid.New().String(), barcelonaGroupID, alexUserID, "EUR", "Airbnb (3 nights)", "EQUAL", 540, 8, ""},
		{uuid.New().String(), barcelonaGroupID, demoUserID, "EUR", "Sagrada Familia Tickets", "EQUAL", 75, 7, ""},
		{uuid.New().String(), barcelonaGroupID, demoUserID, "EUR", "Tapas Dinner at El Nacional", "EQUAL", 135, 7, ""},
		{uuid.New().String(), barcelonaGroupID, alexUserID, "EUR", "Beach Bar Drinks", "EQUAL", 60, 6, ""},
	}

	var expenseRows []map[string]interface{}
	var payerRows []map[string]interface{}

	for _, e := range expenses {
		ts := now.Add(-time.Duration(e.daysAgo) * 24 * time.Hour).UTC()
		tsStr := ts.Format(time.RFC3339)
		dateOnly := ts.Format("2006-01-02")
		timeOnly := ts.Format("15:04:05")

		row := map[string]interface{}{
			"id": e.id, "group_id": e.groupID, "paid_by_user_id": e.paidBy,
			"total_amount": e.amount, "currency": e.currency, "description": e.description,
			"type": e.expType, "category": "EXPENSE",
			"transaction_timestamp": tsStr, "date_only": dateOnly, "time_only": timeOnly,
			"created_at": tsStr, "updated_at": tsStr,
		}
		if e.receiptURL != "" {
			row["receipt_image_url"] = e.receiptURL
		}
		expenseRows = append(expenseRows, row)

		payerRows = append(payerRows, map[string]interface{}{
			"id": uuid.New().String(), "expense_id": e.id,
			"user_id": e.paidBy, "amount_paid": e.amount, "created_at": tsStr,
		})
	}

	if err := c.restInsert("expenses", expenseRows); err != nil {
		return fmt.Errorf("expenses: %w", err)
	}
	if err := c.restInsert("expense_payers", payerRows); err != nil {
		return fmt.Errorf("expense_payers: %w", err)
	}

	// --- Splits ---
	var splitRows []map[string]interface{}

	addSplit := func(expIdx int, userID string, amount float64, pct *float64) {
		ts := now.Add(-time.Duration(expenses[expIdx].daysAgo) * 24 * time.Hour).UTC().Format(time.RFC3339)
		row := map[string]interface{}{
			"id": uuid.New().String(), "expense_id": expenses[expIdx].id,
			"user_id": userID, "amount": amount, "percentage": pct,
			"created_at": ts, "updated_at": ts,
		}
		splitRows = append(splitRows, row)
	}

	flatmates := []string{demoUserID, alexUserID, priyaUserID, marcusUserID}
	barcelona := []string{demoUserID, alexUserID, sofiaUserID}

	// 0: Rent ₹48,000 EQUAL among 4
	for _, uid := range flatmates {
		addSplit(0, uid, 12000, nil)
	}

	// 1: Groceries ₹3,500 ITEMIZED
	for _, s := range []struct {
		uid    string
		amount float64
	}{
		{demoUserID, 980}, {alexUserID, 1100}, {priyaUserID, 1050}, {marcusUserID, 370},
	} {
		addSplit(1, s.uid, s.amount, nil)
	}

	// 2: WiFi ₹1,200 EQUAL among 4
	for _, uid := range flatmates {
		addSplit(2, uid, 300, nil)
	}

	// 3: Dinner ₹3,680 PERCENTAGE
	for _, s := range []struct {
		uid    string
		amount float64
		pct    float64
	}{
		{demoUserID, 1472, 40}, {alexUserID, 920, 25}, {priyaUserID, 736, 20}, {marcusUserID, 552, 15},
	} {
		pct := s.pct
		addSplit(3, s.uid, s.amount, &pct)
	}

	// 4: Cleaning ₹560 EXACT_AMOUNT
	for _, s := range []struct {
		uid    string
		amount float64
	}{
		{demoUserID, 200}, {alexUserID, 160}, {priyaUserID, 100}, {marcusUserID, 100},
	} {
		addSplit(4, s.uid, s.amount, nil)
	}

	// 5-9: Barcelona EQUAL among 3
	for i, perPerson := range []float64{150, 180, 25, 45, 20} {
		for _, uid := range barcelona {
			addSplit(5+i, uid, perPerson, nil)
		}
	}

	if err := c.restInsert("expense_splits", splitRows); err != nil {
		return fmt.Errorf("expense_splits: %w", err)
	}

	// --- Receipt items for Groceries (expense 1) ---
	groceryTS := now.Add(-time.Duration(expenses[1].daysAgo) * 24 * time.Hour).UTC().Format(time.RFC3339)
	type receiptItem struct {
		name   string
		price  float64
		userID string
	}
	items := []receiptItem{
		{"Chicken Breast (1kg)", 550, demoUserID},
		{"Biscuits & Snacks", 430, demoUserID},
		{"Basmati Rice (5kg)", 400, alexUserID},
		{"Sunflower Oil (1L)", 700, alexUserID},
		{"Farm Fresh Eggs (12)", 180, priyaUserID},
		{"Mixed Vegetables", 520, priyaUserID},
		{"Moong Dal (1kg)", 350, priyaUserID},
		{"Toned Milk (2L)", 250, marcusUserID},
		{"Whole Wheat Bread", 120, marcusUserID},
	}

	var itemRows []map[string]interface{}
	var assignmentRows []map[string]interface{}

	for _, it := range items {
		itemID := uuid.New().String()
		itemRows = append(itemRows, map[string]interface{}{
			"id": itemID, "expense_id": expenses[1].id,
			"name": it.name, "price": it.price, "created_at": groceryTS,
		})
		assignmentRows = append(assignmentRows, map[string]interface{}{
			"id": uuid.New().String(), "receipt_item_id": itemID,
			"user_id": it.userID, "created_at": groceryTS,
		})
	}

	if err := c.restInsert("receipt_items", itemRows); err != nil {
		return fmt.Errorf("receipt_items: %w", err)
	}
	if err := c.restInsert("receipt_item_assignments", assignmentRows); err != nil {
		return fmt.Errorf("receipt_item_assignments: %w", err)
	}

	return nil
}
