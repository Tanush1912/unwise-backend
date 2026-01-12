package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

type DB struct {
	Pool *pgxpool.Pool
}

func New(databaseURL string) (*DB, error) {
	zap.L().Info("Initializing database connection pool")
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		zap.L().Error("Failed to create connection pool", zap.Error(err))
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		zap.L().Error("Failed to ping database", zap.Error(err))
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	zap.L().Info("Database connection established successfully")
	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	zap.L().Info("Closing database connection pool")
	db.Pool.Close()
}

func (db *DB) WithTx(ctx context.Context, fn func(Querier) error) error {
	txID := uuid.New().String()
	startTime := time.Now()

	zap.L().Debug("Beginning transaction", zap.String("tx_id", txID))

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		zap.L().Error("Failed to begin transaction", zap.String("tx_id", txID), zap.Error(err))
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			zap.L().Error("Recovered from panic in transaction", zap.String("tx_id", txID), zap.Any("panic", p))
			_ = tx.Rollback(ctx)
			panic(p) 
		} else if err != nil {
			zap.L().Warn("Rolling back transaction due to error", zap.String("tx_id", txID), zap.Error(err))
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				zap.L().Error("Failed to rollback transaction", zap.String("tx_id", txID), zap.Error(rbErr))
			}
		}
	}()

	err = fn(tx)
	if err != nil {
		return err 
	}

	if err := tx.Commit(ctx); err != nil {
		zap.L().Error("Failed to commit transaction", zap.String("tx_id", txID), zap.Error(err))
		return fmt.Errorf("committing transaction: %w", err)
	}

	zap.L().Debug("Transaction committed successfully",
		zap.String("tx_id", txID),
		zap.Duration("duration", time.Since(startTime)))
	return nil
}
