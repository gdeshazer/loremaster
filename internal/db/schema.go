package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate creates all required tables and indexes if they don't exist.
// Safe to call on every startup (idempotent).
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS vector`,

		`CREATE TABLE IF NOT EXISTS projects (
			id          BIGSERIAL PRIMARY KEY,
			slug        TEXT NOT NULL UNIQUE,
			name        TEXT NOT NULL,
			description TEXT,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS documents (
			id          BIGSERIAL PRIMARY KEY,
			project_id  BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			file_path   TEXT NOT NULL,
			chunk_index INT  NOT NULL,
			title       TEXT,
			content     TEXT NOT NULL,
			embedding   VECTOR(768),
			fts         TSVECTOR GENERATED ALWAYS AS (to_tsvector('english', content)) STORED,
			metadata    JSONB NOT NULL DEFAULT '{}',
			indexed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (project_id, file_path, chunk_index)
		)`,

		// HNSW index for fast cosine similarity — created concurrently to avoid locking.
		// Using IF NOT EXISTS requires Postgres 15+; pgvector/pgvector:pg17 satisfies this.
		`CREATE INDEX IF NOT EXISTS documents_embedding_hnsw_idx
			ON documents USING hnsw (embedding vector_cosine_ops)`,

		`CREATE INDEX IF NOT EXISTS documents_fts_gin_idx
			ON documents USING GIN (fts)`,

		`CREATE INDEX IF NOT EXISTS documents_project_id_idx
			ON documents (project_id)`,
	}

	for _, sql := range statements {
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("migration failed on statement %q: %w", sql[:min(60, len(sql))], err)
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
