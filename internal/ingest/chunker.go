package ingest

import (
	"strings"
	"unicode"
)

const (
	// DefaultChunkSize is the approximate number of words per chunk.
	// ~400 tokens ≈ ~300 words for English prose.
	DefaultChunkSize = 300
	// DefaultOverlap is the number of words of overlap between adjacent chunks.
	DefaultOverlap = 40
)

// Chunk holds one indexed piece of a document.
type Chunk struct {
	Index   int
	Content string
}

// ChunkText splits text into overlapping word-window chunks.
// chunkSize and overlap are in word counts (not tokens).
// Sentence boundaries are preferred for split points when possible.
func ChunkText(text string, chunkSize, overlap int) []Chunk {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}

	words := tokenize(text)
	if len(words) == 0 {
		return nil
	}

	// If the entire text fits in one chunk, return it as-is.
	if len(words) <= chunkSize {
		return []Chunk{{Index: 0, Content: strings.Join(words, " ")}}
	}

	var chunks []Chunk
	step := chunkSize - overlap
	if step <= 0 {
		step = 1
	}

	for start := 0; start < len(words); start += step {
		end := start + chunkSize
		if end > len(words) {
			end = len(words)
		}

		// Prefer ending on a sentence boundary (word ending with . ! ?) within
		// the last 20% of the chunk to avoid cutting mid-sentence.
		if end < len(words) {
			end = preferSentenceBoundary(words, end, chunkSize)
		}

		content := strings.Join(words[start:end], " ")
		chunks = append(chunks, Chunk{Index: len(chunks), Content: content})

		if end >= len(words) {
			break
		}
	}
	return chunks
}

// tokenize splits text into words, normalizing whitespace.
func tokenize(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r)
	})
	// Filter out empty strings from FieldsFunc.
	result := fields[:0]
	for _, f := range fields {
		if f != "" {
			result = append(result, f)
		}
	}
	return result
}

// preferSentenceBoundary looks backward from pos within a window (20% of
// chunkSize) for a word ending in sentence-terminal punctuation.
func preferSentenceBoundary(words []string, pos, chunkSize int) int {
	window := chunkSize / 5
	if window < 5 {
		window = 5
	}
	searchFrom := pos - window
	if searchFrom < 0 {
		searchFrom = 0
	}
	// Search backward from pos.
	for i := pos - 1; i >= searchFrom; i-- {
		w := words[i]
		if len(w) == 0 {
			continue
		}
		last := rune(w[len(w)-1])
		if last == '.' || last == '!' || last == '?' || last == '"' || last == '\'' {
			return i + 1
		}
	}
	return pos
}
