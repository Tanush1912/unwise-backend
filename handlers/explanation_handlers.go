package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	apperrors "unwise-backend/errors"
)

func (h *Handlers) ExplainTransaction(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	var req struct {
		TransactionID string `json:"transaction_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid request body. Please provide valid JSON."))
		return
	}

	if req.TransactionID == "" {
		handleError(w, apperrors.MissingRequiredField("Transaction ID"))
		return
	}

	log.Printf("[ExplainTransaction] User %s requested explanation for %s", userID, req.TransactionID)
	explanation, err := h.explanationService.ExplainTransaction(r.Context(), req.TransactionID, userID)
	if err != nil {
		log.Printf("[ExplainTransaction] Failed: %v", err)
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, explanation)
}
