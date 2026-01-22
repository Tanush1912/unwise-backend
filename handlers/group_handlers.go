package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	apperrors "unwise-backend/errors"
	"unwise-backend/models"
	"unwise-backend/services"

	"encoding/csv"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type CreateGroupRequest struct {
	Name         string           `json:"name"`
	Type         models.GroupType `json:"type"`
	MemberEmails []string         `json:"member_emails"`
}

type UpdateGroupRequest struct {
	Name string `json:"name"`
}

type AddMemberRequest struct {
	Email string `json:"email"`
}

type AddPlaceholderMemberRequest struct {
	Name string `json:"name"`
}

type UpdateDefaultCurrencyRequest struct {
	Currency string `json:"currency"`
}

func (h *Handlers) GetGroups(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	groups, err := h.groupService.GetByUserIDWithBalances(r.Context(), userID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, groups)
}

func (h *Handlers) GetGroup(w http.ResponseWriter, r *http.Request) {
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

	group, err := h.groupService.GetByID(r.Context(), groupID, userID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, group)
}

func (h *Handlers) CreateGroup(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid request body. Please provide valid JSON."))
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		handleError(w, apperrors.MissingRequiredField("Group name"))
		return
	}
	if len(name) < services.MinGroupNameLength || len(name) > services.MaxGroupNameLength {
		handleError(w, apperrors.InvalidRequest(fmt.Sprintf("Group name must be between %d and %d characters.", services.MinGroupNameLength, services.MaxGroupNameLength)))
		return
	}

	groupType := models.GroupType(strings.ToUpper(string(req.Type)))
	switch groupType {
	case models.GroupTypeTrip, models.GroupTypeHome, models.GroupTypeCouple, models.GroupTypeOther:
	default:
		groupType = models.GroupTypeOther
	}

	group, err := h.groupService.Create(r.Context(), userID, name, groupType, req.MemberEmails)
	if err != nil {
		handleError(w, err)
		return
	}

	zap.L().Info("Group created", zap.String("group_id", group.ID), zap.String("creator_id", userID))

	respondJSON(w, http.StatusCreated, group)
}

func (h *Handlers) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	groupID := chi.URLParam(r, "groupID")
	if _, err := uuid.Parse(groupID); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid Group ID format."))
		return
	}

	var req UpdateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid request body. Please provide valid JSON."))
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		handleError(w, apperrors.MissingRequiredField("Group name"))
		return
	}
	if len(name) < services.MinGroupNameLength || len(name) > services.MaxGroupNameLength {
		handleError(w, apperrors.InvalidRequest(fmt.Sprintf("Group name must be between %d and %d characters.", services.MinGroupNameLength, services.MaxGroupNameLength)))
		return
	}

	group, err := h.groupService.Update(r.Context(), groupID, userID, name)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, group)
}

func (h *Handlers) DeleteGroup(w http.ResponseWriter, r *http.Request) {
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

	if err := h.groupService.Delete(r.Context(), groupID, userID); err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Group deleted successfully"})
}

func (h *Handlers) AddMember(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	groupID := chi.URLParam(r, "groupID")
	if _, err := uuid.Parse(groupID); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid Group ID format."))
		return
	}

	var req AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid request body. Please provide valid JSON."))
		return
	}

	if strings.TrimSpace(req.Email) == "" {
		handleError(w, apperrors.MissingRequiredField("Email"))
		return
	}

	if err := h.groupService.AddMember(r.Context(), groupID, userID, req.Email); err != nil {
		handleError(w, err)
		return
	}

	zap.L().Info("Member added to group", zap.String("group_id", groupID), zap.String("email", req.Email))

	respondJSON(w, http.StatusOK, map[string]string{"message": "Member added successfully"})
}

func (h *Handlers) AddPlaceholderMember(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}
	groupID := chi.URLParam(r, "groupID")
	if _, err := uuid.Parse(groupID); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid Group ID format."))
		return
	}

	var req AddPlaceholderMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid request body. Please provide valid JSON."))
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		handleError(w, apperrors.MissingRequiredField("Name"))
		return
	}
	if len(name) < services.MinGroupNameLength || len(name) > services.MaxGroupNameLength {
		handleError(w, apperrors.InvalidRequest(fmt.Sprintf("Name must be between %d and %d characters.", services.MinGroupNameLength, services.MaxGroupNameLength)))
		return
	}

	if err := h.groupService.AddPlaceholderMember(r.Context(), groupID, userID, req.Name); err != nil {
		handleError(w, err)
		return
	}

	zap.L().Info("Placeholder member added to group", zap.String("group_id", groupID), zap.String("name", name))

	respondJSON(w, http.StatusCreated, map[string]string{"message": "Placeholder member added successfully"})
}

func (h *Handlers) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	groupID := chi.URLParam(r, "groupID")
	memberID := chi.URLParam(r, "userID")

	if groupID == "" {
		handleError(w, apperrors.MissingRequiredField("Group ID"))
		return
	}
	if memberID == "" {
		handleError(w, apperrors.MissingRequiredField("Member ID"))
		return
	}

	if err := h.groupService.RemoveMember(r.Context(), groupID, userID, memberID); err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Member removed successfully"})
}

func (h *Handlers) GetTransactions(w http.ResponseWriter, r *http.Request) {
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

	transactions, err := h.groupService.GetTransactions(r.Context(), groupID, userID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, transactions)
}

type SettleUpRequest struct {
	PayerID    string  `json:"payer_id"`
	ReceiverID string  `json:"receiver_id"`
	Amount     float64 `json:"amount"`
}

func (h *Handlers) SettleUp(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}
	groupID := chi.URLParam(r, "groupID")
	if _, err := uuid.Parse(groupID); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid Group ID format."))
		return
	}

	var req SettleUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid request body. Please provide valid JSON."))
		return
	}

	if _, err := uuid.Parse(req.PayerID); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid Payer ID format."))
		return
	}
	if _, err := uuid.Parse(req.ReceiverID); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid Receiver ID format."))
		return
	}
	if req.Amount <= 0 {
		handleError(w, apperrors.InvalidAmount("Amount must be greater than zero."))
		return
	}

	expense, err := h.groupService.CreateSettlement(r.Context(), groupID, userID, req.PayerID, req.ReceiverID, req.Amount)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, expense)
}

func (h *Handlers) GetSettlements(w http.ResponseWriter, r *http.Request) {
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

	settlements, err := h.settlementService.CalculateSettlements(r.Context(), groupID, userID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, settlements)
}

func (h *Handlers) GetBalances(w http.ResponseWriter, r *http.Request) {
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

	balances, err := h.groupService.GetBalancesEdgeList(r.Context(), groupID, userID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, balances)
}

func (h *Handlers) ExportGroupCSV(w http.ResponseWriter, r *http.Request) {
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

	transactions, err := h.groupService.GetTransactions(r.Context(), groupID, userID)
	if err != nil {
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=group_export.csv")

	writer := csv.NewWriter(w)
	defer writer.Flush()

	header := []string{"Date", "Description", "Category", "Cost", "Paid By", "Your Share"}
	if err := writer.Write(header); err != nil {
		handleError(w, apperrors.InternalError(err))
		return
	}

	for _, t := range transactions {
		paidBy := "Unknown"
		if t.PaidByUser != nil {
			paidBy = t.PaidByUser.Name
		}

		record := []string{
			t.Date,
			t.Description,
			string(t.Category),
			strconv.FormatFloat(t.TotalAmount, 'f', 2, 64),
			paidBy,
			strconv.FormatFloat(t.UserShare, 'f', 2, 64),
		}
		if err := writer.Write(record); err != nil {
			handleError(w, apperrors.InternalError(err))
			return
		}
	}
}

func (h *Handlers) UpdateDefaultCurrency(w http.ResponseWriter, r *http.Request) {
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

	var req UpdateDefaultCurrencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid request body. Please provide valid JSON."))
		return
	}

	currency := strings.TrimSpace(strings.ToUpper(req.Currency))
	if currency == "" {
		handleError(w, apperrors.MissingRequiredField("Currency"))
		return
	}

	group, err := h.groupService.UpdateDefaultCurrency(r.Context(), groupID, userID, currency)
	if err != nil {
		handleError(w, err)
		return
	}

	zap.L().Info("Group default currency updated", zap.String("group_id", groupID), zap.String("currency", currency))

	respondJSON(w, http.StatusOK, group)
}
