package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// JSONStore is a file-backed replay store: one JSONL file per run holding
// every recorded entry. Lookup scans the file (fine for Stage-3 scale).
type JSONStore struct {
	mu   sync.Mutex
	path string
	f    *os.File
	enc  *json.Encoder
}

// NewJSONStore opens (creating if needed) a JSONL store at path.
func NewJSONStore(path string) (*JSONStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("replay: mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("replay: open %s: %w", path, err)
	}
	return &JSONStore{path: path, f: f, enc: json.NewEncoder(f)}, nil
}

// Lookup scans the store for a matching entry.
func (s *JSONStore) Lookup(ctx context.Context, kind Kind, hash string) (json.RawMessage, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.f.Sync(); err != nil {
		return nil, false, err
	}
	if _, err := s.f.Seek(0, 0); err != nil {
		return nil, false, err
	}
	dec := json.NewDecoder(s.f)
	for {
		var e Entry
		if err := dec.Decode(&e); err != nil {
			break // io.EOF or corrupt tail
		}
		if e.Kind == kind && e.InputHash == hash {
			return e.Output, true, nil
		}
	}
	return nil, false, nil
}

// Put appends one entry.
func (s *JSONStore) Put(ctx context.Context, e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enc.Encode(e)
}

// Close closes the underlying file.
func (s *JSONStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f.Close()
}

var _ Store = (*JSONStore)(nil)
