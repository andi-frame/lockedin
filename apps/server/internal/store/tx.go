package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps the pool and the generated queries. Use WithTx for anything that
// changes state together with the ledger (AGENTS.md invariant 2).
type Store struct {
	Pool *pgxpool.Pool
	*Queries
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{Pool: pool, Queries: New(pool)} }

// Connect opens a pool and verifies the connection.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("store: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return pool, nil
}

// WithTx runs fn in one transaction: commit on nil, rollback on error or panic.
func (s *Store) WithTx(ctx context.Context, fn func(q *Queries) error) (err error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if err = fn(s.Queries.WithTx(tx)); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}

// IsNoRows reports a query that found nothing (including an idempotent insert that hit its key).
func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
