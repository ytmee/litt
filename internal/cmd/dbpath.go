package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/ytmee/litt/internal/store"
)

func resolveDBPath(cmd *cobra.Command) (string, error) {
	if cmd.Flags().Changed("db") {
		path, _ := cmd.Flags().GetString("db")
		if path == "" {
			return "", fmt.Errorf("--db flag requires a non-empty path")
		}
		return path, nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		candidate := filepath.Join(dir, ".litt", "litt.db")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return filepath.Join(".litt", "litt.db"), nil
}

func openStore(cmd *cobra.Command) (*store.Store, error) {
	dbPath, err := resolveDBPath(cmd)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("not a litt repository (no .litt/litt.db found); run 'litt init' first")
	}
	s, err := store.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	return s, nil
}

func openOrInitStore(cmd *cobra.Command) (*store.Store, error) {
	dbPath, err := resolveDBPath(cmd)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		littDir := filepath.Dir(dbPath)
		if err := os.MkdirAll(littDir, 0755); err != nil {
			return nil, fmt.Errorf("create %s: %w", littDir, err)
		}
		s, err := store.Open(dbPath)
		if err != nil {
			return nil, fmt.Errorf("open store: %w", err)
		}
		if err := s.Migrate(); err != nil {
			s.Close()
			return nil, fmt.Errorf("migrate: %w", err)
		}
		if err := s.SeedLabels(); err != nil {
			s.Close()
			return nil, fmt.Errorf("seed labels: %w", err)
		}
		return s, nil
	}

	s, err := store.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	return s, nil
}
