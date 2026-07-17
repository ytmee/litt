package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/ytmee/litt/internal/store"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a litt repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			littDir := filepath.Join(".litt")
			if err := os.MkdirAll(littDir, 0755); err != nil {
				return fmt.Errorf("create .litt directory: %w", err)
			}
			dbPath := filepath.Join(littDir, "litt.db")
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer s.Close()
			if err := s.Migrate(); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
			if err := s.SeedLabels(); err != nil {
				return fmt.Errorf("seed labels: %w", err)
			}
			if err := appendGitignore(); err != nil {
				return fmt.Errorf("update .gitignore: %w", err)
			}
			cmd.Println("Initialized litt repository in .litt/")
			return nil
		},
	}
}

func appendGitignore() error {
	data, err := os.ReadFile(".gitignore")
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return os.WriteFile(".gitignore", []byte("# litt\n.litt/\n"), 0644)
	}
	content := string(data)
	if containsLine(content, ".litt/") {
		return nil
	}
	f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(data) > 0 && data[len(data)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	if _, err := f.WriteString(".litt/\n"); err != nil {
		return err
	}
	return nil
}

func containsLine(content, line string) bool {
	for i := 0; i < len(content); {
		j := i
		for j < len(content) && content[j] != '\n' {
			j++
		}
		l := content[i:j]
		if j < len(content) {
			j++
		}
		if l == line || l == line+"\r" {
			return true
		}
		i = j
	}
	return false
}
