package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pgxpool and verifies connectivity.
// pgvector type registration is not needed here because we never scan vector
// columns — Postgres computes distances in-query and we only pass vectors as
// encoded parameters via pgvector.NewVector().
func Connect(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping DB: %w", err)
	}

	return pool, nil
}
