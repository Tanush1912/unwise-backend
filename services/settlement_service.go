package services

import (
	"container/heap"
	"context"
	"math"

	apperrors "unwise-backend/errors"
	"unwise-backend/models"
	"unwise-backend/repository"
)

type SettlementService interface {
	CalculateSettlements(ctx context.Context, groupID, userID string) ([]models.Settlement, error)
}

type settlementService struct {
	expenseRepo repository.ExpenseRepository
	groupRepo   repository.GroupRepository
}

func NewSettlementService(expenseRepo repository.ExpenseRepository, groupRepo repository.GroupRepository) SettlementService {
	return &settlementService{
		expenseRepo: expenseRepo,
		groupRepo:   groupRepo,
	}
}

func (s *settlementService) requireMembership(ctx context.Context, groupID, userID string) error {
	return RequireGroupMembership(ctx, s.groupRepo, groupID, userID)
}

type personBalance struct {
	userID  string
	balance float64
}

type balanceHeap []personBalance

func (h balanceHeap) Len() int           { return len(h) }
func (h balanceHeap) Less(i, j int) bool { return h[i].balance > h[j].balance }
func (h balanceHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *balanceHeap) Push(x interface{}) {
	*h = append(*h, x.(personBalance))
}
func (h *balanceHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (s *settlementService) CalculateSettlements(ctx context.Context, groupID, userID string) ([]models.Settlement, error) {
	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return nil, err
	}

	balancesByCurrency, err := s.expenseRepo.GetGroupMemberBalances(ctx, groupID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting group member balances", err)
	}
	currencyBalances := make(map[string]map[string]float64)
	for userID, currencyMap := range balancesByCurrency {
		for currency, balance := range currencyMap {
			if currencyBalances[currency] == nil {
				currencyBalances[currency] = make(map[string]float64)
			}
			currencyBalances[currency][userID] = balance
		}
	}

	var allSettlements []models.Settlement

	for currency, userBalances := range currencyBalances {
		settlements := s.calculateSettlementsForCurrency(userBalances, currency)
		allSettlements = append(allSettlements, settlements...)
	}

	return allSettlements, nil
}

func (s *settlementService) calculateSettlementsForCurrency(balances map[string]float64, currency string) []models.Settlement {
	creditorHeap := &balanceHeap{}
	debtorHeap := &balanceHeap{}

	for uID, balance := range balances {
		roundedBalance := math.Round(balance*RoundingFactor) / RoundingFactor
		if roundedBalance > BalanceThreshold {
			heap.Push(creditorHeap, personBalance{userID: uID, balance: roundedBalance})
		} else if roundedBalance < -BalanceThreshold {
			heap.Push(debtorHeap, personBalance{userID: uID, balance: math.Abs(roundedBalance)})
		}
	}

	var settlements []models.Settlement
	for creditorHeap.Len() > 0 && debtorHeap.Len() > 0 {
		creditor := heap.Pop(creditorHeap).(personBalance)
		debtor := heap.Pop(debtorHeap).(personBalance)

		amount := math.Min(creditor.balance, debtor.balance)
		roundedAmount := math.Round(amount*RoundingFactor) / RoundingFactor

		if roundedAmount > BalanceThreshold {
			settlements = append(settlements, models.Settlement{
				FromUserID: debtor.userID,
				ToUserID:   creditor.userID,
				Amount:     roundedAmount,
				Currency:   currency,
			})
		}

		creditor.balance = math.Round((creditor.balance-amount)*RoundingFactor) / RoundingFactor
		debtor.balance = math.Round((debtor.balance-amount)*RoundingFactor) / RoundingFactor

		if creditor.balance > BalanceThreshold {
			heap.Push(creditorHeap, creditor)
		}
		if debtor.balance > BalanceThreshold {
			heap.Push(debtorHeap, debtor)
		}
	}

	return settlements
}
