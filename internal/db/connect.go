package db

import (
	"context"
	"fmt"

	pgxlib "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go/pgx"
)

// Connect opens a pgxpool, registers the pgvector type, and verifies connectivity.
func Connect(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("parse DB URL: %w", err)
	}

	// Register pgvector types after each new connection is established.
	cfg.AfterConnect = func(ctx context.Context, conn *pgxlib.Conn) error {
		return pgx.RegisterTypes(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping DB: %w", err)
	}

	return pool, nil
}
