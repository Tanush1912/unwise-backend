package handlers

import (
	"log"
	"net/http"
)

func (h *Handlers) GetDashboard(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}
	email, err := getUserEmail(r)
	if err != nil {
		handleError(w, err)
		return
	}
	name, _ := getUserName(r)

	dashboard, err := h.dashboardService.GetDashboard(r.Context(), userID, email, name)
	if err != nil {
		log.Printf("[Handlers.GetDashboard] Error: %v", err)
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, dashboard)
}
