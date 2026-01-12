package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	apperrors "unwise-backend/errors"

	"github.com/go-chi/chi/v5"
)

type AddFriendRequest struct {
	Email string `json:"email"`
}

func (h *Handlers) GetFriends(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	friends, err := h.friendService.GetFriendsWithBalances(r.Context(), userID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, friends)
}

func (h *Handlers) AddFriend(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	var req AddFriendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid request body. Please provide valid JSON."))
		return
	}

	if strings.TrimSpace(req.Email) == "" {
		handleError(w, apperrors.MissingRequiredField("Email"))
		return
	}

	if err := h.friendService.AddFriendByEmail(r.Context(), userID, req.Email); err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Friend added successfully"})
}

func (h *Handlers) RemoveFriend(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	friendID := chi.URLParam(r, "friendID")
	if friendID == "" {
		handleError(w, apperrors.MissingRequiredField("Friend ID"))
		return
	}

	if err := h.friendService.RemoveFriend(r.Context(), userID, friendID); err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Friend removed successfully"})
}

func (h *Handlers) SearchPotentialFriends(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		respondJSON(w, http.StatusOK, []interface{}{})
		return
	}

	results, err := h.friendService.SearchPotentialFriends(r.Context(), query)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, results)
}
