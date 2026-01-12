package handlers

import (
	"net/http"
)

func (h *Handlers) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	if err := h.userService.DeleteAccount(r.Context(), userID); err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Account deleted successfully"})
}
