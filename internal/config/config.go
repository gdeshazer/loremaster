// Package config loads project-local settings from loremaster.json.
// It walks up from the current working directory looking for the file,
// mirroring how git finds .git. Global settings (DB URL, Ollama URL)
// live in env vars / viper and are not handled here.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// ErrNotFound is returned when no loremaster.json is found in the directory tree.
var ErrNotFound = errors.New("loremaster.json not found in current directory or any parent")

// ProjectConfig holds settings from loremaster.json.
type ProjectConfig struct {
	// Project is the slug that maps to a projects.slug in the database.
	Project string `json:"project"`
	// EmbeddingModel overrides the global default Ollama model for this project.
	EmbeddingModel string `json:"embedding_model,omitempty"`
	// Exclude is a list of glob patterns (relative to the project root) to skip during indexing.
	Exclude []string `json:"exclude,omitempty"`

	// Dir is the directory where loremaster.json was found. Not serialized.
	Dir string `json:"-"`
}

// Load finds and parses the nearest loremaster.json walking up from dir.
// Pass an empty string to start from the current working directory.
func Load(startDir string) (*ProjectConfig, error) {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}

	for {
		candidate := filepath.Join(dir, "loremaster.json")
		data, err := os.ReadFile(candidate)
		if err == nil {
			var cfg ProjectConfig
			if err := json.Unmarshal(data, &cfg); err != nil {
				return nil, &ParseError{Path: candidate, Err: err}
			}
			cfg.Dir = dir
			return &cfg, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding the file.
			return nil, ErrNotFound
		}
		dir = parent
	}
}

// Write serializes cfg to loremaster.json in targetDir.
func Write(targetDir string, cfg *ProjectConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(targetDir, "loremaster.json")
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// ParseError wraps a JSON parse failure with the offending file path.
type ParseError struct {
	Path string
	Err  error
}

func (e *ParseError) Error() string {
	return "parsing " + e.Path + ": " + e.Err.Error()
}

func (e *ParseError) Unwrap() error { return e.Err }
