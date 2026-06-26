package db

import (
	"testing"
)

// rrfMerge is pure logic — test it without a database.

func TestRRFMerge_emptyInputs(t *testing.T) {
	results := rrfMerge(nil, nil, 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRRFMerge_onlySemanticResults(t *testing.T) {
	semantic := []SearchResult{
		{FilePath: "a.md", ChunkIndex: 0, Score: 0.9},
		{FilePath: "b.md", ChunkIndex: 0, Score: 0.8},
	}
	results := rrfMerge(semantic, nil, 5)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestRRFMerge_onlyKeywordResults(t *testing.T) {
	keyword := []SearchResult{
		{FilePath: "c.md", ChunkIndex: 0, Score: 0.7},
	}
	results := rrfMerge(nil, keyword, 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestRRFMerge_overlappingResultsBoosted(t *testing.T) {
	// "shared.md" appears in both lists — should outrank items in only one.
	semantic := []SearchResult{
		{FilePath: "shared.md", ChunkIndex: 0, Score: 0.5},
		{FilePath: "semantic-only.md", ChunkIndex: 0, Score: 0.9},
	}
	keyword := []SearchResult{
		{FilePath: "keyword-only.md", ChunkIndex: 0, Score: 0.9},
		{FilePath: "shared.md", ChunkIndex: 0, Score: 0.5},
	}
	results := rrfMerge(semantic, keyword, 10)

	// shared.md has RRF score from both lists → should appear at rank 1.
	if results[0].FilePath != "shared.md" {
		t.Errorf("expected shared.md at rank 1, got %q", results[0].FilePath)
	}
}

func TestRRFMerge_respectsLimit(t *testing.T) {
	semantic := make([]SearchResult, 10)
	for i := range semantic {
		semantic[i] = SearchResult{FilePath: "a.md", ChunkIndex: i}
	}
	results := rrfMerge(semantic, nil, 3)
	if len(results) != 3 {
		t.Errorf("expected limit=3 results, got %d", len(results))
	}
}

func TestRRFMerge_scoresArePositive(t *testing.T) {
	semantic := []SearchResult{
		{FilePath: "x.md", ChunkIndex: 0, Score: 0.8},
	}
	keyword := []SearchResult{
		{FilePath: "y.md", ChunkIndex: 0, Score: 0.6},
	}
	results := rrfMerge(semantic, keyword, 5)
	for _, r := range results {
		if r.Score <= 0 {
			t.Errorf("expected positive RRF score, got %f for %s", r.Score, r.FilePath)
		}
	}
}

func TestRRFMerge_deduplicatesChunks(t *testing.T) {
	// Same chunk appears multiple times in semantic list — should appear once.
	semantic := []SearchResult{
		{FilePath: "dup.md", ChunkIndex: 0, Score: 0.9},
		{FilePath: "dup.md", ChunkIndex: 0, Score: 0.8},
	}
	results := rrfMerge(semantic, nil, 10)

	count := 0
	for _, r := range results {
		if r.FilePath == "dup.md" && r.ChunkIndex == 0 {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected dedup to 1, got %d copies of dup.md chunk 0", count)
	}
}
