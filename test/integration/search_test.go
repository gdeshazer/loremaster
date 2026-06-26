//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdeshazer/loremaster/internal/db"
	"github.com/gdeshazer/loremaster/internal/embed"
	"github.com/gdeshazer/loremaster/internal/ingest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestSearchEndToEnd(t *testing.T) {
	ctx := context.Background()

	// --- Spin up Postgres with pgvector ---
	pgContainer, err := postgres.Run(ctx,
		"pgvector/pgvector:pg17",
		postgres.WithDatabase("loremaster_test"),
		postgres.WithUsername("loremaster"),
		postgres.WithPassword("loremaster"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := db.Connect(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	err = db.Migrate(ctx, pool)
	require.NoError(t, err)

	// --- Mock Ollama embedding server ---
	// Returns deterministic random-ish vectors so search actually functions.
	ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Prompt string `json:"prompt"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// Seed random with a hash of the prompt so same text = same vector.
		vec := makeVector(req.Prompt, 768)
		json.NewEncoder(w).Encode(map[string]interface{}{"embedding": vec})
	}))
	defer ollamaSrv.Close()

	embedder, err := embed.NewClient(ollamaSrv.URL, "test-model")
	require.NoError(t, err)

	store := db.NewStore(pool)

	// --- Create project ---
	project, err := store.CreateProject(ctx, "test-story", "Test Story", "Integration test project")
	require.NoError(t, err)

	// --- Index fixture files ---
	fixtureDir := filepath.Join("..", "fixtures")
	paths, err := ingest.Walk(fixtureDir, nil)
	require.NoError(t, err)
	require.Greater(t, len(paths), 0, "expected fixture files")

	for _, path := range paths {
		src, err := os.ReadFile(path)
		require.NoError(t, err)

		doc, err := ingest.Parse(src)
		require.NoError(t, err)

		chunks := ingest.ChunkText(doc.Plaintext, ingest.DefaultChunkSize, ingest.DefaultOverlap)
		for _, chunk := range chunks {
			vec, err := embedder.Embed(ctx, chunk.Content)
			require.NoError(t, err)

			err = store.UpsertChunk(ctx, project.ID, path, chunk.Index, doc.Title, chunk.Content, vec, doc.Metadata)
			require.NoError(t, err)
		}
	}

	// --- Verify doc count ---
	count, err := store.DocCount(ctx, project.ID)
	require.NoError(t, err)
	assert.Greater(t, count, int64(0), "expected indexed chunks")
	t.Logf("Indexed %d chunks from %d files", count, len(paths))

	// --- Test keyword search ---
	kwResults, err := store.KeywordSearch(ctx, project.ID, "Roland gunslinger", 5)
	require.NoError(t, err)
	assert.Greater(t, len(kwResults), 0, "keyword search should return results for 'Roland gunslinger'")
	t.Logf("Keyword search returned %d results", len(kwResults))

	// --- Test semantic search ---
	queryVec, err := embedder.Embed(ctx, "magic system and its limitations")
	require.NoError(t, err)

	semResults, err := store.SemanticSearch(ctx, project.ID, queryVec, 5)
	require.NoError(t, err)
	assert.Greater(t, len(semResults), 0, "semantic search should return results")
	t.Logf("Semantic search returned %d results", len(semResults))

	// --- Test hybrid search ---
	hybridResults, err := store.HybridSearch(ctx, project.ID, "magical doorways", queryVec, 5)
	require.NoError(t, err)
	assert.Greater(t, len(hybridResults), 0, "hybrid search should return results")
	t.Logf("Hybrid search returned %d results", len(hybridResults))

	// Hybrid should differ from pure keyword (RRF reranking).
	// We can't assert identical ordering, but we can verify it returns valid results.
	for _, r := range hybridResults {
		assert.NotEmpty(t, r.FilePath)
		assert.NotEmpty(t, r.Content)
		assert.Greater(t, r.Score, float64(0))
	}

	// --- Test GetDocument ---
	docChunks, err := store.GetDocument(ctx, project.ID, paths[0])
	require.NoError(t, err)
	assert.Greater(t, len(docChunks), 0, "GetDocument should return chunks")

	// --- Test ListDocuments ---
	docs, err := store.ListDocuments(ctx, project.ID, 50)
	require.NoError(t, err)
	assert.Equal(t, len(paths), len(docs), "ListDocuments should return one entry per file")

	// --- Test project deletion cascade ---
	err = store.DeleteProject(ctx, "test-story")
	require.NoError(t, err)

	// Chunks should be gone (CASCADE).
	count, err = store.DocCount(ctx, project.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "all chunks should be deleted with project")
}

// makeVector creates a deterministic pseudo-random float32 vector for a given
// string, so the same text always produces the same embedding in tests.
func makeVector(text string, dims int) []float32 {
	seed := int64(0)
	for _, c := range text {
		seed = seed*31 + int64(c)
	}
	rng := rand.New(rand.NewSource(seed))
	vec := make([]float32, dims)
	for i := range vec {
		vec[i] = rng.Float32()*2 - 1
	}
	// Normalize to unit length for cosine similarity.
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	norm := float32(1.0 / (sum + 1e-9))
	for i := range vec {
		vec[i] = vec[i] * norm
	}
	return vec
}

// Satisfy unused import of fmt
var _ = fmt.Sprintf
