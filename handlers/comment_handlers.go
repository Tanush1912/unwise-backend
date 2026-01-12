package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	apperrors "unwise-backend/errors"

	"github.com/go-chi/chi/v5"
)

type CreateCommentRequest struct {
	Text string `json:"text"`
}

type ReactionRequest struct {
	Emoji string `json:"emoji"`
}

func (h *Handlers) GetComments(w http.ResponseWriter, r *http.Request) {
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

	comments, err := h.commentService.GetComments(r.Context(), expenseID, userID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, comments)
}

func (h *Handlers) CreateComment(w http.ResponseWriter, r *http.Request) {
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

	var req CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid JSON"))
		return
	}

	if strings.TrimSpace(req.Text) == "" {
		handleError(w, apperrors.MissingRequiredField("Text"))
		return
	}

	comment, err := h.commentService.AddComment(r.Context(), expenseID, userID, req.Text)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, comment)
}

func (h *Handlers) DeleteComment(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	commentID := chi.URLParam(r, "commentID")
	if commentID == "" {
		handleError(w, apperrors.MissingRequiredField("Comment ID"))
		return
	}

	if err := h.commentService.DeleteComment(r.Context(), commentID, userID); err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Comment deleted"})
}

func (h *Handlers) AddReaction(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	commentID := chi.URLParam(r, "commentID")
	var req ReactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid JSON"))
		return
	}

	if req.Emoji == "" {
		handleError(w, apperrors.MissingRequiredField("Emoji"))
		return
	}

	if err := h.commentService.AddReaction(r.Context(), commentID, userID, req.Emoji); err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Reaction added"})
}

func (h *Handlers) RemoveReaction(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	commentID := chi.URLParam(r, "commentID")
	emoji := r.URL.Query().Get("emoji") // Pass emoji as query param for delete

	if emoji == "" {
		handleError(w, apperrors.MissingRequiredField("Emoji query param"))
		return
	}

	if err := h.commentService.RemoveReaction(r.Context(), commentID, userID, emoji); err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Reaction removed"})
}
