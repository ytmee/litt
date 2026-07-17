package store

import (
	"testing"
)

func TestMigrate(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 migration, got %d", count)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 migration after second run, got %d", count)
	}
}

func TestSeedLabels(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedLabels(); err != nil {
		t.Fatal(err)
	}

	labels, err := s.ListLabels()
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 7 {
		t.Fatalf("expected 7 seeded labels, got %d", len(labels))
	}
}

func TestSeedLabelsIdempotent(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedLabels(); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedLabels(); err != nil {
		t.Fatal(err)
	}

	labels, err := s.ListLabels()
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 7 {
		t.Fatalf("expected 7 labels after second seed, got %d", len(labels))
	}
}

func TestListLabels(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedLabels(); err != nil {
		t.Fatal(err)
	}

	labels, err := s.ListLabels()
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]string{
		"needs-triage":    "triage",
		"needs-info":      "triage",
		"ready-for-agent": "triage",
		"ready-for-human": "triage",
		"wontfix":         "triage",
		"bug":             "category",
		"enhancement":     "category",
	}

	if len(labels) != len(expected) {
		t.Fatalf("expected %d labels, got %d", len(expected), len(labels))
	}

	for _, l := range labels {
		kind, ok := expected[l.Name]
		if !ok {
			t.Errorf("unexpected label: %s", l.Name)
			continue
		}
		if l.Kind != kind {
			t.Errorf("label %s: expected kind %q, got %q", l.Name, kind, l.Kind)
		}
		if l.Color == "" {
			t.Errorf("label %s: empty color", l.Name)
		}
		if l.ID == 0 {
			t.Errorf("label %s: zero ID", l.Name)
		}
	}
}

func TestListLabelsEmpty(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}

	labels, err := s.ListLabels()
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 0 {
		t.Fatalf("expected 0 labels, got %d", len(labels))
	}
}

func TestTablesCreated(t *testing.T) {
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}

	expectedTables := []string{"issues", "labels", "issue_labels", "issue_blocks", "schema_migrations"}
	for _, name := range expectedTables {
		var count int
		err := s.db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			name,
		).Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("table %s not found", name)
		}
	}
}
