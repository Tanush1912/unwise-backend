package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	apperrors "unwise-backend/errors"
	"unwise-backend/models"
	"unwise-backend/services"

	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type CreateExpenseRequest struct {
	GroupID         string                     `json:"group_id"`
	TotalAmount     float64                    `json:"total_amount"`
	Description     string                     `json:"description"`
	ReceiptImageURL *string                    `json:"receipt_image_url,omitempty"`
	Type            models.ExpenseType         `json:"split_method"`
	Category        models.TransactionCategory `json:"type"`
	Tax             float64                    `json:"tax"`
	CGST            float64                    `json:"cgst"`
	SGST            float64                    `json:"sgst"`
	ServiceCharge   float64                    `json:"service_charge"`
	Payers          []models.ExpensePayer      `json:"payers,omitempty"`
	PaidByUserID    *string                    `json:"paid_by_user_id,omitempty"`
	Splits          []models.ExpenseSplit      `json:"splits"`
	ReceiptItems    []ReceiptItemRequest       `json:"receipt_items,omitempty"`
	Date            *time.Time                 `json:"date,omitempty"`
}

type ReceiptItemRequest struct {
	Name       string   `json:"name"`
	Price      float64  `json:"price"`
	AssignedTo []string `json:"assigned_to"`
}

type UpdateExpenseRequest struct {
	TotalAmount     float64                    `json:"total_amount"`
	Description     string                     `json:"description"`
	ReceiptImageURL *string                    `json:"receipt_image_url,omitempty"`
	Type            models.ExpenseType         `json:"split_method"`
	Category        models.TransactionCategory `json:"type"`
	Tax             float64                    `json:"tax"`
	CGST            float64                    `json:"cgst"`
	SGST            float64                    `json:"sgst"`
	ServiceCharge   float64                    `json:"service_charge"`
	Payers          []models.ExpensePayer      `json:"payers,omitempty"`
	PaidByUserID    *string                    `json:"paid_by_user_id,omitempty"`
	Splits          []models.ExpenseSplit      `json:"splits"`
	ReceiptItems    []ReceiptItemRequest       `json:"receipt_items,omitempty"`
	Date            *time.Time                 `json:"date,omitempty"`
}

func (h *Handlers) GetExpenses(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	groupID := chi.URLParam(r, "groupID")
	if groupID == "" {
		handleError(w, apperrors.MissingRequiredField("Group ID"))
		return
	}

	expenses, err := h.expenseService.GetByGroupID(r.Context(), groupID, userID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, expenses)
}

func (h *Handlers) GetExpense(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	expenseID := chi.URLParam(r, "expenseID")
	if expenseID == "" {
		handleError(w, apperrors.MissingRequiredField("Expense ID"))
		return
	}

	expense, err := h.expenseService.GetByID(r.Context(), expenseID, userID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, expense)
}

func (h *Handlers) CreateExpense(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	var req CreateExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid request body. Please provide valid JSON."))
		return
	}

	if _, err := uuid.Parse(req.GroupID); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid Group ID format. Must be a valid UUID."))
		return
	}
	if req.TotalAmount <= 0 {
		handleError(w, apperrors.InvalidAmount("Total amount must be greater than zero."))
		return
	}

	if req.Category != models.TransactionCategoryPayment && req.Category != models.TransactionCategoryRepayment {
		desc := strings.TrimSpace(req.Description)
		if desc == "" {
			handleError(w, apperrors.MissingRequiredField("Description"))
			return
		}
		if len(desc) < services.MinDescriptionLength || len(desc) > services.MaxDescriptionLength {
			handleError(w, apperrors.InvalidRequest(fmt.Sprintf("Description must be between %d and %d characters.", services.MinDescriptionLength, services.MaxDescriptionLength)))
			return
		}
	}

	if req.Category != models.TransactionCategoryPayment && req.Category != models.TransactionCategoryRepayment {
		if len(req.Splits) == 0 {
			handleError(w, apperrors.MissingRequiredField("Splits"))
			return
		}
	}

	expense := &models.Expense{
		GroupID:         req.GroupID,
		TotalAmount:     req.TotalAmount,
		Description:     req.Description,
		ReceiptImageURL: req.ReceiptImageURL,
		Type:            req.Type,
		Tax:             req.Tax,
		CGST:            req.CGST,
		SGST:            req.SGST,
		ServiceCharge:   req.ServiceCharge,
		Payers:          req.Payers,
		PaidByUserID:    req.PaidByUserID,
	}

	if req.Date != nil {
		expense.DateISO = *req.Date
		expense.Date = req.Date.Format("2006-01-02")
		expense.Time = req.Date.Format("15:04")
	}

	if len(req.ReceiptItems) > 0 {
		receiptItems := make([]models.ReceiptItem, 0, len(req.ReceiptItems))
		for _, item := range req.ReceiptItems {
			receiptItem := models.ReceiptItem{
				Name:  item.Name,
				Price: item.Price,
			}
			for _, userID := range item.AssignedTo {
				receiptItem.Assignments = append(receiptItem.Assignments, models.ReceiptItemAssignment{
					UserID: userID,
				})
			}
			receiptItems = append(receiptItems, receiptItem)
		}
		expense.ReceiptItems = receiptItems
	}

	expense, err = h.expenseService.Create(r.Context(), userID, expense, req.Splits)
	if err != nil {
		handleError(w, err)
		return
	}

	zap.L().Info("Expense created",
		zap.String("expense_id", expense.ID),
		zap.String("group_id", expense.GroupID),
		zap.String("user_id", userID),
		zap.Float64("amount", expense.TotalAmount))

	respondJSON(w, http.StatusCreated, expense)
}

func (h *Handlers) UpdateExpense(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	expenseID := chi.URLParam(r, "expenseID")
	if _, err := uuid.Parse(expenseID); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid Expense ID format."))
		return
	}

	var req UpdateExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid request body. Please provide valid JSON."))
		return
	}

	if req.TotalAmount <= 0 {
		handleError(w, apperrors.InvalidAmount("Total amount must be greater than zero."))
		return
	}

	if req.Category != models.TransactionCategoryPayment && req.Category != models.TransactionCategoryRepayment {
		desc := strings.TrimSpace(req.Description)
		if desc == "" {
			handleError(w, apperrors.MissingRequiredField("Description"))
			return
		}
		if len(desc) < services.MinDescriptionLength || len(desc) > services.MaxDescriptionLength {
			handleError(w, apperrors.InvalidRequest(fmt.Sprintf("Description must be between %d and %d characters.", services.MinDescriptionLength, services.MaxDescriptionLength)))
			return
		}
	}

	if req.Category != models.TransactionCategoryPayment && req.Category != models.TransactionCategoryRepayment {
		if len(req.Splits) == 0 {
			handleError(w, apperrors.MissingRequiredField("Splits"))
			return
		}
	}

	expense := &models.Expense{
		TotalAmount:     req.TotalAmount,
		Description:     req.Description,
		ReceiptImageURL: req.ReceiptImageURL,
		Type:            req.Type,
		Tax:             req.Tax,
		CGST:            req.CGST,
		SGST:            req.SGST,
		ServiceCharge:   req.ServiceCharge,
		Payers:          req.Payers,
		PaidByUserID:    req.PaidByUserID,
	}

	if req.Date != nil {
		expense.DateISO = *req.Date
		expense.Date = req.Date.Format("2006-01-02")
		expense.Time = req.Date.Format("15:04")
	}

	if len(req.ReceiptItems) > 0 {
		receiptItems := make([]models.ReceiptItem, 0, len(req.ReceiptItems))
		for _, item := range req.ReceiptItems {
			receiptItem := models.ReceiptItem{
				Name:  item.Name,
				Price: item.Price,
			}
			for _, userID := range item.AssignedTo {
				receiptItem.Assignments = append(receiptItem.Assignments, models.ReceiptItemAssignment{
					UserID: userID,
				})
			}
			receiptItems = append(receiptItems, receiptItem)
		}
		expense.ReceiptItems = receiptItems
	}

	expense, err = h.expenseService.Update(r.Context(), expenseID, userID, expense, req.Splits)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, expense)
}

func (h *Handlers) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	expenseID := chi.URLParam(r, "expenseID")
	if expenseID == "" {
		handleError(w, apperrors.MissingRequiredField("Expense ID"))
		return
	}

	if err := h.expenseService.Delete(r.Context(), expenseID, userID); err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Expense deleted successfully"})
}
