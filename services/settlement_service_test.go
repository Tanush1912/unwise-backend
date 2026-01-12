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
		balances map[string]float64
		expected []models.Settlement
	}{
		{
			name: "Simple Case",
			balances: map[string]float64{
				"A": 10.00,
				"B": -10.00,
			},
			expected: []models.Settlement{
				{FromUserID: "B", ToUserID: "A", Amount: 10.00},
			},
		},
		{
			name: "Three People Equal Split residue",
			balances: map[string]float64{
				"A": 6.67,
				"B": -3.33,
				"C": -3.34,
			},
			expected: []models.Settlement{
				{FromUserID: "C", ToUserID: "A", Amount: 3.34},
				{FromUserID: "B", ToUserID: "A", Amount: 3.33},
			},
		},
		{
			name: "Floating point precision residue",
			balances: map[string]float64{
				"A": 0.0000000000000001,
				"B": -0.0000000000000001,
			},
			expected: []models.Settlement{},
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

			for i, s := range settlements {
				if i >= len(tt.expected) {
					break
				}
				if s.FromUserID != tt.expected[i].FromUserID || s.ToUserID != tt.expected[i].ToUserID || math.Abs(s.Amount-tt.expected[i].Amount) > 0.001 {
					t.Errorf("settlement %d mismatch: got %+v, want %+v", i, s, tt.expected[i])
				}
			}
		})
	}
}
