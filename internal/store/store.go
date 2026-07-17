package store

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Label struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
}

var seedLabels = []Label{
	{Name: "needs-triage", Color: "e4e669", Description: "Maintainer needs to evaluate this issue", Kind: "triage"},
	{Name: "needs-info", Color: "fef2c0", Description: "Waiting on reporter for more information", Kind: "triage"},
	{Name: "ready-for-agent", Color: "006B75", Description: "Fully specified, ready for an AFK agent", Kind: "triage"},
	{Name: "ready-for-human", Color: "bfdadc", Description: "Requires human implementation", Kind: "triage"},
	{Name: "wontfix", Color: "ffffff", Description: "Will not be actioned", Kind: "triage"},
	{Name: "bug", Color: "d73a4a", Description: "Something isn't working", Kind: "category"},
	{Name: "enhancement", Color: "a2eeef", Description: "New feature or request", Kind: "category"},
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set journal mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	return &Store{db: db}, nil
}

func OpenInMemory() (*Store, error) {
	return Open(":memory:")
}

func (s *Store) Close() error {
	return s.db.Close()
}

type migration struct {
	version int
	sql     string
}

func loadMigrations() ([]migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	var migrations []migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		var version int
		if _, err := fmt.Sscanf(entry.Name(), "%d_", &version); err != nil {
			continue
		}
		migrations = append(migrations, migration{version: version, sql: string(data)})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})
	return migrations, nil
}

func (s *Store) Migrate() error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	for _, m := range migrations {
		err := s.db.QueryRow("SELECT 1 FROM schema_migrations WHERE version = ?", m.version).Scan(new(int))
		if err == sql.ErrNoRows {
			if _, err := s.db.Exec(m.sql); err != nil {
				return fmt.Errorf("apply migration %d: %w", m.version, err)
			}
			if _, err := s.db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.version); err != nil {
				return fmt.Errorf("record migration %d: %w", m.version, err)
			}
		} else if err != nil {
			return fmt.Errorf("check migration %d: %w", m.version, err)
		}
	}
	return nil
}

func (s *Store) SeedLabels() error {
	for _, l := range seedLabels {
		_, err := s.db.Exec(
			"INSERT OR IGNORE INTO labels (name, color, description, kind) VALUES (?, ?, ?, ?)",
			l.Name, l.Color, l.Description, l.Kind,
		)
		if err != nil {
			return fmt.Errorf("seed label %s: %w", l.Name, err)
		}
	}
	return nil
}

func (s *Store) ListLabels() ([]Label, error) {
	rows, err := s.db.Query("SELECT id, name, color, description, kind FROM labels ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	defer rows.Close()
	var labels []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.Name, &l.Color, &l.Description, &l.Kind); err != nil {
			return nil, fmt.Errorf("scan label: %w", err)
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

func (s *Store) DB() *sql.DB {
	return s.db
}
