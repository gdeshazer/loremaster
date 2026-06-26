// Package embed provides text embedding generation via Ollama.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Client generates embeddings by calling the Ollama HTTP API.
type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

// NewClient creates an embedding client targeting the given Ollama base URL and model.
func NewClient(baseURL, model string) (*Client, error) {
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("invalid Ollama URL %q: %w", baseURL, err)
	}
	if model == "" {
		return nil, fmt.Errorf("model must not be empty")
	}
	return &Client{
		baseURL: baseURL,
		model:   model,
		http:    &http.Client{},
	}, nil
}

type embedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed returns the embedding vector for a single text string.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(embedRequest{Model: c.model, Prompt: text})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned HTTP %d", resp.StatusCode)
	}

	var er embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, fmt.Errorf("decoding ollama response: %w", err)
	}
	if len(er.Embedding) == 0 {
		return nil, fmt.Errorf("ollama returned empty embedding for model %q", c.model)
	}
	return er.Embedding, nil
}

// EmbedBatch returns embeddings for a slice of texts, calling Embed for each.
// It stops and returns on the first error.
func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := c.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embedding text[%d]: %w", i, err)
		}
		results[i] = vec
	}
	return results, nil
}
