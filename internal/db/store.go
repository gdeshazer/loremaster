package db

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
)

// Store provides all database operations for loremaster.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a Store backed by the given connection pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// --- Project operations ---

// CreateProject inserts a new project, returning the created record.
// Returns an error if the slug already exists.
func (s *Store) CreateProject(ctx context.Context, slug, name, description string) (*Project, error) {
	var p Project
	err := s.pool.QueryRow(ctx, `
		INSERT INTO projects (slug, name, description)
		VALUES ($1, $2, $3)
		RETURNING id, slug, name, description, created_at, updated_at
	`, slug, name, description).Scan(
		&p.ID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return &p, nil
}

// GetProject fetches a project by slug.
func (s *Store) GetProject(ctx context.Context, slug string) (*Project, error) {
	var p Project
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, name, description, created_at, updated_at
		FROM projects WHERE slug = $1
	`, slug).Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get project %q: %w", slug, err)
	}
	return &p, nil
}

// ListProjects returns all projects with their document counts.
func (s *Store) ListProjects(ctx context.Context) ([]ProjectSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.slug, p.name, p.description, COUNT(d.id) AS doc_count
		FROM projects p
		LEFT JOIN documents d ON d.project_id = p.id
		GROUP BY p.id, p.slug, p.name, p.description
		ORDER BY p.created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var summaries []ProjectSummary
	for rows.Next() {
		var s ProjectSummary
		if err := rows.Scan(&s.Slug, &s.Name, &s.Description, &s.DocCount); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// DeleteProject removes a project and all its documents (via CASCADE).
func (s *Store) DeleteProject(ctx context.Context, slug string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE slug = $1`, slug)
	if err != nil {
		return fmt.Errorf("delete project %q: %w", slug, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("project %q not found", slug)
	}
	return nil
}

// ProjectSummary is a project with its document count.
type ProjectSummary struct {
	Slug        string
	Name        string
	Description string
	DocCount    int64
}

// --- Document upsert ---

// UpsertChunk inserts or updates a document chunk. The UNIQUE constraint on
// (project_id, file_path, chunk_index) drives the conflict target.
func (s *Store) UpsertChunk(ctx context.Context, projectID int64, filePath string, chunkIdx int, title, content string, embedding []float32, metadata map[string]string) error {
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	vec := pgvector.NewVector(embedding)
	_, err = s.pool.Exec(ctx, `
		INSERT INTO documents (project_id, file_path, chunk_index, title, content, embedding, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (project_id, file_path, chunk_index) DO UPDATE
		  SET title      = EXCLUDED.title,
		      content    = EXCLUDED.content,
		      embedding  = EXCLUDED.embedding,
		      metadata   = EXCLUDED.metadata,
		      indexed_at = NOW()
	`, projectID, filePath, chunkIdx, title, content, vec, metaJSON)
	if err != nil {
		return fmt.Errorf("upsert chunk: %w", err)
	}
	return nil
}

// DeleteFileChunks removes all chunks for a specific file within a project.
// Useful when a file is deleted from the source tree.
func (s *Store) DeleteFileChunks(ctx context.Context, projectID int64, filePath string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM documents WHERE project_id = $1 AND file_path = $2
	`, projectID, filePath)
	return err
}

// --- Search ---

// SemanticSearch finds the top-k chunks closest to queryVec using HNSW cosine similarity.
func (s *Store) SemanticSearch(ctx context.Context, projectID int64, queryVec []float32, limit int) ([]SearchResult, error) {
	vec := pgvector.NewVector(queryVec)
	rows, err := s.pool.Query(ctx, `
		SELECT file_path, chunk_index, title, content, metadata,
		       1 - (embedding <=> $1) AS score
		FROM documents
		WHERE project_id = $2 AND embedding IS NOT NULL
		ORDER BY embedding <=> $1
		LIMIT $3
	`, vec, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("semantic search: %w", err)
	}
	defer rows.Close()
	return scanResults(rows)
}

// KeywordSearch finds chunks matching query using PostgreSQL full-text search.
func (s *Store) KeywordSearch(ctx context.Context, projectID int64, query string, limit int) ([]SearchResult, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT file_path, chunk_index, title, content, metadata,
		       ts_rank_cd(fts, websearch_to_tsquery('english', $1)) AS score
		FROM documents
		WHERE project_id = $2
		  AND fts @@ websearch_to_tsquery('english', $1)
		ORDER BY score DESC
		LIMIT $3
	`, query, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("keyword search: %w", err)
	}
	defer rows.Close()
	return scanResults(rows)
}

// HybridSearch runs semantic and keyword search, then merges results using
// Reciprocal Rank Fusion (RRF) with k=60 (standard constant).
func (s *Store) HybridSearch(ctx context.Context, projectID int64, query string, queryVec []float32, limit int) ([]SearchResult, error) {
	semantic, err := s.SemanticSearch(ctx, projectID, queryVec, limit*2)
	if err != nil {
		return nil, err
	}
	keyword, err := s.KeywordSearch(ctx, projectID, query, limit*2)
	if err != nil {
		return nil, err
	}
	return rrfMerge(semantic, keyword, limit), nil
}

const rrfK = 60.0

type rrfKey struct {
	FilePath   string
	ChunkIndex int
}

// rrfMerge implements Reciprocal Rank Fusion over two ranked result lists.
func rrfMerge(semantic, keyword []SearchResult, limit int) []SearchResult {
	scores := make(map[rrfKey]float64)
	byKey := make(map[rrfKey]SearchResult)

	for i, r := range semantic {
		k := rrfKey{r.FilePath, r.ChunkIndex}
		scores[k] += 1.0 / (rrfK + float64(i+1))
		byKey[k] = r
	}
	for i, r := range keyword {
		k := rrfKey{r.FilePath, r.ChunkIndex}
		scores[k] += 1.0 / (rrfK + float64(i+1))
		if _, exists := byKey[k]; !exists {
			byKey[k] = r
		}
	}

	type scored struct {
		key   rrfKey
		score float64
	}
	ranked := make([]scored, 0, len(scores))
	for k, s := range scores {
		ranked = append(ranked, scored{k, s})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	if limit > len(ranked) {
		limit = len(ranked)
	}
	results := make([]SearchResult, limit)
	for i, r := range ranked[:limit] {
		res := byKey[r.key]
		res.Score = r.score
		results[i] = res
	}
	return results
}

// GetDocument fetches all chunks for a file path within a project, ordered by chunk_index.
func (s *Store) GetDocument(ctx context.Context, projectID int64, filePath string) ([]SearchResult, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT file_path, chunk_index, title, content, metadata, 1.0 AS score
		FROM documents
		WHERE project_id = $1 AND file_path = $2
		ORDER BY chunk_index
	`, projectID, filePath)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	defer rows.Close()
	return scanResults(rows)
}

// ListDocuments lists distinct file paths in a project.
func (s *Store) ListDocuments(ctx context.Context, projectID int64, limit int) ([]DocumentSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT file_path, title, COUNT(*) AS chunk_count, MAX(indexed_at) AS indexed_at
		FROM documents
		WHERE project_id = $1
		GROUP BY file_path, title
		ORDER BY file_path
		LIMIT $2
	`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	var docs []DocumentSummary
	for rows.Next() {
		var d DocumentSummary
		if err := rows.Scan(&d.FilePath, &d.Title, &d.ChunkCount, &d.IndexedAt); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// DocumentSummary is a per-file summary returned by ListDocuments.
type DocumentSummary struct {
	FilePath   string
	Title      string
	ChunkCount int64
	IndexedAt  interface{}
}

// DocCount returns the total number of chunks in a project.
func (s *Store) DocCount(ctx context.Context, projectID int64) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents WHERE project_id = $1`, projectID).Scan(&n)
	return n, err
}

// scanResults scans pgx rows into []SearchResult.
func scanResults(rows pgx.Rows) ([]SearchResult, error) {
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var metaJSON []byte
		if err := rows.Scan(&r.FilePath, &r.ChunkIndex, &r.Title, &r.Content, &metaJSON, &r.Score); err != nil {
			return nil, err
		}
		if len(metaJSON) > 0 {
			if err := json.Unmarshal(metaJSON, &r.Metadata); err != nil {
				return nil, err
			}
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
