package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
)

type Config struct {
	CatalogLabel string `json:"catalog_label"`
}

type Store struct {
	path    string
	current atomic.Pointer[Config]
}

func NewStore(path string) (*Store, error) {
	store := &Store{path: path}
	if err := store.Reload(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Current() Config {
	return *s.current.Load()
}

func (s *Store) Reload() error {
	file, err := os.Open(s.path)
	if err != nil {
		return fmt.Errorf("open config %q: %w", s.path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var next Config
	if err := decoder.Decode(&next); err != nil {
		return fmt.Errorf("decode config %q: %w", s.path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return fmt.Errorf("decode config %q: %w", s.path, err)
	}
	if strings.TrimSpace(next.CatalogLabel) == "" {
		return fmt.Errorf("config %q: catalog_label must not be empty", s.path)
	}
	s.current.Store(&next)
	return nil
}
