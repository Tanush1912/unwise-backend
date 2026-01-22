package handlers

import (
	"net/http"

	"unwise-backend/models"
	"unwise-backend/repository"
)

type CurrencyHandlers struct {
	currencyRepo repository.CurrencyRepository
}

func NewCurrencyHandlers(currencyRepo repository.CurrencyRepository) *CurrencyHandlers {
	return &CurrencyHandlers{
		currencyRepo: currencyRepo,
	}
}

func (h *CurrencyHandlers) GetCurrencies(w http.ResponseWriter, r *http.Request) {
	currencies, err := h.currencyRepo.GetAll(r.Context())
	if err != nil {
		handleError(w, err)
		return
	}

	if currencies == nil {
		currencies = []models.Currency{}
	}

	respondJSON(w, http.StatusOK, currencies)
}
