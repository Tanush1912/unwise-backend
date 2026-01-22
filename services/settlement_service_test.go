package services

import (
	"context"
	"math"
	"testing"
	"unwise-backend/models"
)

func TestCalculateSettlements(t *testing.T) {
	tests := []struct {
		name     string
		balances map[string]map[string]float64 
		expected []models.Settlement
	}{
		{
			name: "Simple Case",
			balances: map[string]map[string]float64{
				"A": {"INR": 10.00},
				"B": {"INR": -10.00},
			},
			expected: []models.Settlement{
				{FromUserID: "B", ToUserID: "A", Amount: 10.00, Currency: "INR"},
			},
		},
		{
			name: "Three People Equal Split residue",
			balances: map[string]map[string]float64{
				"A": {"INR": 6.67},
				"B": {"INR": -3.33},
				"C": {"INR": -3.34},
			},
			expected: []models.Settlement{
				{FromUserID: "C", ToUserID: "A", Amount: 3.34, Currency: "INR"},
				{FromUserID: "B", ToUserID: "A", Amount: 3.33, Currency: "INR"},
			},
		},
		{
			name: "Floating point precision residue",
			balances: map[string]map[string]float64{
				"A": {"INR": 0.0000000000000001},
				"B": {"INR": -0.0000000000000001},
			},
			expected: []models.Settlement{},
		},
		{
			name: "Multi-currency settlements",
			balances: map[string]map[string]float64{
				"A": {"INR": 100.00, "USD": 50.00},
				"B": {"INR": -100.00, "USD": -50.00},
			},
			expected: []models.Settlement{
				{FromUserID: "B", ToUserID: "A", Amount: 100.00, Currency: "INR"},
				{FromUserID: "B", ToUserID: "A", Amount: 50.00, Currency: "USD"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockExpenseRepo{balances: tt.balances}
			groupRepo := &mockGroupRepo{}

			s := NewSettlementService(repo, groupRepo)

			settlements, err := s.CalculateSettlements(context.Background(), "group1", "user1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(settlements) != len(tt.expected) {
				t.Errorf("expected %d settlements, got %d", len(tt.expected), len(settlements))
			}

			expectedMap := make(map[string]models.Settlement)
			for _, e := range tt.expected {
				key := e.FromUserID + "->" + e.ToUserID + ":" + e.Currency
				expectedMap[key] = e
			}

			for _, s := range settlements {
				key := s.FromUserID + "->" + s.ToUserID + ":" + s.Currency
				expected, ok := expectedMap[key]
				if !ok {
					t.Errorf("unexpected settlement: %+v", s)
					continue
				}
				if math.Abs(s.Amount-expected.Amount) > 0.001 {
					t.Errorf("settlement amount mismatch: got %+v, want %+v", s, expected)
				}
				delete(expectedMap, key)
			}

			for _, remaining := range expectedMap {
				t.Errorf("missing expected settlement: %+v", remaining)
			}
		})
	}
}
