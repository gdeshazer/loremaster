package ingest

import (
	"strings"
	"testing"
)

func TestChunkText_shortTextSingleChunk(t *testing.T) {
	text := "This is a short piece of text."
	chunks := ChunkText(text, 300, 40)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Index != 0 {
		t.Errorf("chunk index: got %d, want 0", chunks[0].Index)
	}
}

func TestChunkText_emptyText(t *testing.T) {
	chunks := ChunkText("", 300, 40)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(chunks))
	}
}

func TestChunkText_producesMultipleChunks(t *testing.T) {
	// Build a 600-word text.
	words := make([]string, 600)
	for i := range words {
		words[i] = "word"
	}
	text := strings.Join(words, " ")

	chunks := ChunkText(text, 300, 40)
	if len(chunks) < 2 {
		t.Errorf("expected ≥2 chunks for 600 words with chunkSize=300, got %d", len(chunks))
	}
}

func TestChunkText_overlapExists(t *testing.T) {
	// 100 distinct words so we can verify overlap.
	words := make([]string, 200)
	for i := range words {
		words[i] = strings.Repeat("w", i+1) // each word is unique
	}
	text := strings.Join(words, " ")

	chunks := ChunkText(text, 100, 20)
	if len(chunks) < 2 {
		t.Skip("not enough chunks to test overlap")
	}

	// The end of chunk[0] should appear at the start of chunk[1].
	end0Words := strings.Fields(chunks[0].Content)
	start1Words := strings.Fields(chunks[1].Content)

	// Last word of chunk 0 should appear somewhere in first 25 words of chunk 1.
	lastWord0 := end0Words[len(end0Words)-1]
	found := false
	limit := 25
	if limit > len(start1Words) {
		limit = len(start1Words)
	}
	for _, w := range start1Words[:limit] {
		if w == lastWord0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no overlap detected: last word of chunk[0]=%q not in first 25 words of chunk[1]", lastWord0)
	}
}

func TestChunkText_indexesAreSequential(t *testing.T) {
	words := make([]string, 1000)
	for i := range words {
		words[i] = "x"
	}
	chunks := ChunkText(strings.Join(words, " "), 200, 30)
	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunks[%d].Index = %d, want %d", i, c.Index, i)
		}
	}
}

func TestChunkText_noOrphanChunks(t *testing.T) {
	// All words from original text should appear in at least one chunk.
	original := "alpha beta gamma delta epsilon zeta eta theta iota kappa"
	chunks := ChunkText(original, 4, 1)
	allContent := ""
	for _, c := range chunks {
		allContent += " " + c.Content
	}
	for _, word := range strings.Fields(original) {
		if !strings.Contains(allContent, word) {
			t.Errorf("word %q missing from all chunks", word)
		}
	}
}

func TestChunkText_defaultsAppliedOnZero(t *testing.T) {
	words := make([]string, 10)
	for i := range words {
		words[i] = "x"
	}
	// Should not panic with zero chunkSize/overlap.
	chunks := ChunkText(strings.Join(words, " "), 0, -1)
	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
}
