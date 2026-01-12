package services

import (
	"context"
	"fmt"

	apperrors "unwise-backend/errors"
	"unwise-backend/models"
	"unwise-backend/repository"

	"github.com/google/generative-ai-go/genai"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

type ExplanationService interface {
	ExplainTransaction(ctx context.Context, transactionID, userID string) (*models.DebtExplanation, error)
}

type explanationService struct {
	expenseRepo repository.ExpenseRepository
	groupRepo   repository.GroupRepository
	userRepo    repository.UserRepository
	apiKey      string
	client      *genai.Client
}

func NewExplanationService(apiKey string, expenseRepo repository.ExpenseRepository, groupRepo repository.GroupRepository, userRepo repository.UserRepository) (ExplanationService, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("creating genai client: %w", err)
	}

	return &explanationService{
		expenseRepo: expenseRepo,
		groupRepo:   groupRepo,
		userRepo:    userRepo,
		apiKey:      apiKey,
		client:      client,
	}, nil
}

func (s *explanationService) ExplainTransaction(ctx context.Context, transactionID, userID string) (*models.DebtExplanation, error) {
	expense, err := s.expenseRepo.GetByID(ctx, transactionID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			return nil, apperrors.ExpenseNotFound()
		}
		return nil, apperrors.DatabaseError("getting expense", err)
	}

	if expense.Explanation != nil && *expense.Explanation != "" {
		return &models.DebtExplanation{
			TransactionID: transactionID,
			Explanation:   *expense.Explanation,
		}, nil
	}

	if err := RequireGroupMembership(ctx, s.groupRepo, expense.GroupID, userID); err != nil {
		return nil, err
	}

	allExpenses, err := s.expenseRepo.GetByGroupID(ctx, expense.GroupID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting group expenses", err)
	}

	expenseIDs := make([]string, len(allExpenses))
	for i, e := range allExpenses {
		expenseIDs[i] = e.ID
	}

	allSplits, err := s.expenseRepo.GetSplitsByExpenseIDs(ctx, expenseIDs)
	if err != nil {
		return nil, apperrors.DatabaseError("batch getting splits", err)
	}

	allPayers, err := s.expenseRepo.GetPayersByExpenseIDs(ctx, expenseIDs)
	if err != nil {
		return nil, apperrors.DatabaseError("batch getting payers", err)
	}

	beforeBalances := make(map[string]float64)
	afterBalances := make(map[string]float64)
	members, err := s.groupRepo.GetMembers(ctx, expense.GroupID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting group members", err)
	}
	userMap := make(map[string]string)
	for _, m := range members {
		userMap[m.ID] = m.Name
	}

	for _, e := range allExpenses {
		isTarget := e.ID == transactionID

		for _, payer := range allPayers[e.ID] {
			afterBalances[payer.UserID] += payer.AmountPaid
			if !isTarget {
				beforeBalances[payer.UserID] += payer.AmountPaid
			}
		}

		for _, split := range allSplits[e.ID] {
			afterBalances[split.UserID] -= split.Amount
			if !isTarget {
				beforeBalances[split.UserID] -= split.Amount
			}
		}
	}

	beforeDebts := s.getSimplifiedDebts(beforeBalances, userMap)
	afterDebts := s.getSimplifiedDebts(afterBalances, userMap)
	targetPayers := allPayers[transactionID]
	targetSplits := allSplits[transactionID]

	prompt := s.buildPrompt(expense, targetPayers, targetSplits, beforeDebts, afterDebts, userMap)

	model := s.client.GenerativeModel("gemini-2.0-flash")
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, apperrors.AIServiceError(err)
	}

	explanationText := ""
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		if part, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
			explanationText = string(part)
		}
	}

	if explanationText != "" {
		go func() {
			err := s.expenseRepo.UpdateExplanation(context.Background(), transactionID, explanationText)
			if err != nil {
				zap.L().Error("Failed to cache explanation", zap.String("transaction_id", transactionID), zap.Error(err))
			}
		}()
	}

	return &models.DebtExplanation{
		TransactionID: transactionID,
		Explanation:   explanationText,
	}, nil
}

func (s *explanationService) getSimplifiedDebts(balances map[string]float64, userMap map[string]string) []string {
	creditors := make([]string, 0)
	debtors := make([]string, 0)
	creditorBal := make(map[string]float64)
	debtorBal := make(map[string]float64)

	for id, bal := range balances {
		if bal > BalanceThreshold {
			creditors = append(creditors, id)
			creditorBal[id] = bal
		} else if bal < -BalanceThreshold {
			debtors = append(debtors, id)
			debtorBal[id] = -bal
		}
	}

	var results []string
	for len(creditors) > 0 && len(debtors) > 0 {
		c := creditors[0]
		d := debtors[0]
		amt := creditorBal[c]
		if debtorBal[d] < amt {
			amt = debtorBal[d]
		}

		results = append(results, fmt.Sprintf("%s owes %s $%.2f", userMap[d], userMap[c], amt))

		creditorBal[c] -= amt
		debtorBal[d] -= amt

		if creditorBal[c] < BalanceThreshold {
			creditors = creditors[1:]
		}
		if debtorBal[d] < BalanceThreshold {
			debtors = debtors[1:]
		}
	}
	return results
}

func (s *explanationService) buildPrompt(target *models.Expense, payers []models.ExpensePayer, splits []models.ExpenseSplit, before, after []string, userMap map[string]string) string {
	beforeList := ""
	for _, d := range before {
		beforeList += "- " + d + "\n"
	}
	if beforeList == "" {
		beforeList = "No outstanding debts.\n"
	}

	afterList := ""
	for _, d := range after {
		afterList += "- " + d + "\n"
	}
	if afterList == "" {
		afterList = "All debts have been completely settled/cancelled out by this transaction.\n"
	}

	participantInfo := "\nPARTICIPANTS:\n"
	participantInfo += "Payers (Who paid):\n"
	for _, p := range payers {
		participantInfo += fmt.Sprintf("- %s: ₹%.2f\n", userMap[p.UserID], p.AmountPaid)
	}
	participantInfo += "\nSplit Participants (Who owes/is involved):\n"
	for _, split := range splits {
		participantInfo += fmt.Sprintf("- %s: ₹%.2f share\n", userMap[split.UserID], split.Amount)
	}

	return fmt.Sprintf(`You are a financial analyst for a debt-splitting app called "Unwise". 
Your job is to explain how a specific transaction changed the debt landscape of a group using a "simplified debt" algorithm.

The algorithm minimizes the number of payments. If A owes B ₹10 and B owes C ₹10, it simplifies to A owes C ₹10.

TRANSACTION DETAILS:
Description: %s
Amount: ₹%.2f
Type: %s
%s
DEBT STATE BEFORE THIS TRANSACTION:
%s
DEBT STATE AFTER THIS TRANSACTION:
%s
Please provide a concise, friendly explanation of what happened. Focus on:
1. Who did the user pay or borrow from effectively?
2. Did this transaction "cancel out" any existing debts? 
3. Why does the 'After' state look the way it does? (e.g., "By paying for dinner, you effectively repaid your debt to Sarah while also putting John in your debt").

Keep it under 3-4 sentences. Use names clearly. Be conversational but accurate. Do NOT start with conversational fillers like "Okay so", "Let's see", or "Here is the breakdown". Get straight to the explanation.`,
		target.Description, target.TotalAmount, target.Category, participantInfo, beforeList, afterList)
}
