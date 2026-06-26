package db

import "time"

type Project struct {
	ID          int64     `db:"id"`
	Slug        string    `db:"slug"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// Chunk is a single indexed chunk from a markdown file.
type Chunk struct {
	ID         int64             `db:"id"`
	ProjectID  int64             `db:"project_id"`
	FilePath   string            `db:"file_path"`
	ChunkIndex int               `db:"chunk_index"`
	Title      string            `db:"title"`
	Content    string            `db:"content"`
	Embedding  []float32         `db:"embedding"`
	Metadata   map[string]string `db:"metadata"`
	IndexedAt  time.Time         `db:"indexed_at"`
}

// SearchResult is returned by all search methods.
type SearchResult struct {
	FilePath   string            `json:"file_path"`
	ChunkIndex int               `json:"chunk_index"`
	Title      string            `json:"title"`
	Content    string            `json:"content"`
	Score      float64           `json:"score"`
	Metadata   map[string]string `json:"metadata"`
}
