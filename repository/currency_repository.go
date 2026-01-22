package repository

import (
	"context"
	"fmt"

	"unwise-backend/database"
	"unwise-backend/models"
)

type CurrencyRepository interface {
	GetAll(ctx context.Context) ([]models.Currency, error)
	GetByCode(ctx context.Context, code string) (*models.Currency, error)
}

type currencyRepository struct {
	db *database.DB
}

func NewCurrencyRepository(db *database.DB) CurrencyRepository {
	return &currencyRepository{db: db}
}

func (r *currencyRepository) GetAll(ctx context.Context) ([]models.Currency, error) {
	query := `SELECT code, name, symbol FROM currencies ORDER BY code`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("getting all currencies: %w", err)
	}
	defer rows.Close()

	var currencies []models.Currency
	for rows.Next() {
		var c models.Currency
		if err := rows.Scan(&c.Code, &c.Name, &c.Symbol); err != nil {
			return nil, fmt.Errorf("scanning currency: %w", err)
		}
		currencies = append(currencies, c)
	}

	return currencies, nil
}

func (r *currencyRepository) GetByCode(ctx context.Context, code string) (*models.Currency, error) {
	query := `SELECT code, name, symbol FROM currencies WHERE code = $1`

	var c models.Currency
	err := r.db.Pool.QueryRow(ctx, query, code).Scan(&c.Code, &c.Name, &c.Symbol)
	if err != nil {
		return nil, fmt.Errorf("getting currency by code: %w", err)
	}

	return &c, nil
}
