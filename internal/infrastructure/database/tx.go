package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxFunc — функция, выполняющая операции в транзакции.
type TxFunc func(tx pgx.Tx) error

// WithTx выполняет функцию в транзакции.
// Автоматически начинает транзакцию, фиксирует или откатывает при ошибке.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn TxFunc) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// WithTxOptions выполняет функцию в транзакции с опциями.
func WithTxOptions(ctx context.Context, pool *pgxpool.Pool, opts pgx.TxOptions, fn TxFunc) error {
	tx, err := pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// WithTxReadOnly выполняет функцию в read-only транзакции.
func WithTxReadOnly(ctx context.Context, pool *pgxpool.Pool, fn TxFunc) error {
	return WithTxOptions(ctx, pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, fn)
}

// WithTxReadWrite выполняет функцию в read-write транзакции с изоляцией.Serializable.
func WithTxReadWrite(ctx context.Context, pool *pgxpool.Pool, fn TxFunc) error {
	return WithTxOptions(ctx, pool, pgx.TxOptions{IsoLevel: pgx.Serializable}, fn)
}
