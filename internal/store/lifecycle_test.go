package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsure_CreatesAndMigrates(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := Ensure(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 migrations, got %d", count)
	}

	labels, err := s.ListLabels()
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) == 0 {
		t.Fatal("expected seeded labels")
	}
}

func TestEnsure_Idempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s1, err := Ensure(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	s1.Close()

	s2, err := Ensure(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	var count int
	if err := s2.db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 migrations, got %d", count)
	}
}

func TestEnsure_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subdir", "litt.db")

	s, err := Ensure(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("Ensure should create the database file")
	}
}

func TestOpenIfExists_Exists(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s1, err := Ensure(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	s1.Close()

	s2, err := OpenIfExists(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if s2 == nil {
		t.Fatal("OpenIfExists should return a store when db exists")
	}
	s2.Close()
}

func TestOpenIfExists_NotExists(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nonexistent.db")

	s, err := OpenIfExists(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if s != nil {
		s.Close()
		t.Fatal("OpenIfExists should return nil when db does not exist")
	}
}
