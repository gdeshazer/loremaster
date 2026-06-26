package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient_validURL(t *testing.T) {
	_, err := NewClient("http://localhost:11434", "nomic-embed-text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewClient_invalidURL(t *testing.T) {
	_, err := NewClient("not-a-url", "nomic-embed-text")
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}

func TestNewClient_emptyModel(t *testing.T) {
	_, err := NewClient("http://localhost:11434", "")
	if err == nil {
		t.Error("expected error for empty model, got nil")
	}
}

func TestEmbed_success(t *testing.T) {
	wantVec := []float32{0.1, 0.2, 0.3}
	srv := mockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify request shape.
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/embeddings" {
			t.Errorf("path: got %s, want /api/embeddings", r.URL.Path)
		}
		var req embedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "nomic-embed-text" {
			t.Errorf("model: got %q", req.Model)
		}
		if req.Prompt != "hello world" {
			t.Errorf("prompt: got %q", req.Prompt)
		}
		json.NewEncoder(w).Encode(embedResponse{Embedding: wantVec})
	})

	client, _ := NewClient(srv.URL, "nomic-embed-text")
	vec, err := client.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(vec) != len(wantVec) {
		t.Fatalf("vec len: got %d, want %d", len(vec), len(wantVec))
	}
	for i, v := range vec {
		if v != wantVec[i] {
			t.Errorf("vec[%d]: got %f, want %f", i, v, wantVec[i])
		}
	}
}

func TestEmbed_serverError(t *testing.T) {
	srv := mockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not loaded", http.StatusInternalServerError)
	})
	client, _ := NewClient(srv.URL, "nomic-embed-text")
	_, err := client.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

func TestEmbed_emptyEmbedding(t *testing.T) {
	srv := mockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(embedResponse{Embedding: nil})
	})
	client, _ := NewClient(srv.URL, "nomic-embed-text")
	_, err := client.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for empty embedding, got nil")
	}
}

func TestEmbedBatch_success(t *testing.T) {
	callCount := 0
	srv := mockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(embedResponse{Embedding: []float32{float32(callCount), 0.0}})
	})

	client, _ := NewClient(srv.URL, "nomic-embed-text")
	vecs, err := client.EmbedBatch(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("EmbedBatch error: %v", err)
	}
	if len(vecs) != 3 {
		t.Errorf("expected 3 vecs, got %d", len(vecs))
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls to Ollama, got %d", callCount)
	}
}

func TestEmbedBatch_stopsOnError(t *testing.T) {
	callCount := 0
	srv := mockOllamaServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 2 {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(embedResponse{Embedding: []float32{1.0}})
	})

	client, _ := NewClient(srv.URL, "nomic-embed-text")
	_, err := client.EmbedBatch(context.Background(), []string{"a", "b", "c"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if callCount != 2 {
		t.Errorf("expected to stop at 2nd call, got %d calls", callCount)
	}
}

func mockOllamaServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}
