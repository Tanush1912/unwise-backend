package services

import (
	"testing"
	"unwise-backend/models"
)

func TestExpenseValidation(t *testing.T) {
	tests := []struct {
		name        string
		expense     *models.Expense
		splits      []models.ExpenseSplit
		shouldError bool
	}{
		{
			name: "Valid Equal Split",
			expense: &models.Expense{
				TotalAmount: 10.00,
				Payers: []models.ExpensePayer{
					{UserID: "A", AmountPaid: 10.00},
				},
			},
			splits: []models.ExpenseSplit{
				{UserID: "A", Amount: 3.33},
				{UserID: "B", Amount: 3.33},
				{UserID: "C", Amount: 3.34},
			},
			shouldError: false,
		},
		{
			name: "Invalid Split Sum",
			expense: &models.Expense{
				TotalAmount: 10.00,
				Payers: []models.ExpensePayer{
					{UserID: "A", AmountPaid: 10.00},
				},
			},
			splits: []models.ExpenseSplit{
				{UserID: "A", Amount: 3.33},
				{UserID: "B", Amount: 3.33},
				{UserID: "C", Amount: 3.33}, 
			},
			shouldError: true,
		},
		{
			name: "Invalid Payer Sum",
			expense: &models.Expense{
				TotalAmount: 10.00,
				Payers: []models.ExpensePayer{
					{UserID: "A", AmountPaid: 5.00},
					{UserID: "B", AmountPaid: 4.99}, 
				},
			},
			splits: []models.ExpenseSplit{
				{UserID: "A", Amount: 5.00},
				{UserID: "B", Amount: 5.00},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &expenseService{}
			err := s.validateExpenseAmounts(tt.expense, tt.splits)
			if (err != nil) != tt.shouldError {
				t.Fatalf("expected error: %v, got: %v", tt.shouldError, err)
			}
		})
	}
}
