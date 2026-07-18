package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/ytmee/litt/internal/store"
)

func initDBPath(cmd *cobra.Command) (string, error) {
	if cmd.Flags().Changed("db") {
		path, _ := cmd.Flags().GetString("db")
		if path == "" {
			return "", fmt.Errorf("--db flag requires a non-empty path")
		}
		return path, nil
	}
	return filepath.Join(".litt", "litt.db"), nil
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a litt repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath, err := initDBPath(cmd)
			if err != nil {
				return err
			}
			s, err := store.Ensure(dbPath)
			if err != nil {
				return fmt.Errorf("initialize store: %w", err)
			}
			defer s.Close()
			if !cmd.Flags().Changed("db") {
				if err := appendGitignore(); err != nil {
					return fmt.Errorf("update .gitignore: %w", err)
				}
			}
			cmd.Printf("Initialized litt repository at %s\n", dbPath)
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
