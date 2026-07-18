package store

import (
	"fmt"
	"os"
	"path/filepath"
)

func Ensure(path string) (*Store, error) {
	needInit := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		needInit = true
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}
	s, err := Open(path)
	if err != nil {
		return nil, err
	}
	if needInit {
		if err := s.Migrate(); err != nil {
			s.Close()
			return nil, fmt.Errorf("migrate: %w", err)
		}
		if err := s.SeedLabels(); err != nil {
			s.Close()
			return nil, fmt.Errorf("seed labels: %w", err)
		}
	}
	return s, nil
}

func OpenIfExists(path string) (*Store, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	return Open(path)
}
