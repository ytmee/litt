package tui

import (
	"testing"

	"charm.land/bubbletea/v2"
	"github.com/ytmee/litt/internal/store"
)

func TestTUI_QuitOnQ(t *testing.T) {
	s, err := store.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}

	m := newModel(s)
	_, cmd := m.Update(tea.KeyPressMsg{Text: "q"})
	if cmd == nil {
		t.Fatal("expected quit command, got nil")
	}
}

func TestTUI_LoadsIssues(t *testing.T) {
	s, err := store.OpenInMemory()
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

	_, err = s.CreateIssue("Issue 1", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Issue 2", "spec", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newModel(s)
	if len(m.issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(m.issues))
	}
	if m.issues[0].Title != "Issue 1" {
		t.Fatalf("expected first issue title 'Issue 1', got %q", m.issues[0].Title)
	}
	if m.issues[1].Title != "Issue 2" {
		t.Fatalf("expected second issue title 'Issue 2', got %q", m.issues[1].Title)
	}
}

func TestTUI_NavigateJK(t *testing.T) {
	s, err := store.OpenInMemory()
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

	for i := 0; i < 3; i++ {
		_, err := s.CreateIssue("Issue", "task", "", nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	m := newModel(s)
	if m.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.cursor)
	}

	m.Update(tea.KeyPressMsg{Text: "j"})
	if m.cursor != 1 {
		t.Fatalf("expected cursor 1 after j, got %d", m.cursor)
	}
	m.Update(tea.KeyPressMsg{Text: "j"})
	if m.cursor != 2 {
		t.Fatalf("expected cursor 2 after second j, got %d", m.cursor)
	}

	m.Update(tea.KeyPressMsg{Text: "k"})
	if m.cursor != 1 {
		t.Fatalf("expected cursor 1 after k, got %d", m.cursor)
	}

	// j at end clamps
	m.Update(tea.KeyPressMsg{Text: "j"})
	m.Update(tea.KeyPressMsg{Text: "j"})
	if m.cursor != 2 {
		t.Fatalf("expected cursor 2 (clamped), got %d", m.cursor)
	}

	// k at start clamps
	m.cursor = 0
	m.Update(tea.KeyPressMsg{Text: "k"})
	if m.cursor != 0 {
		t.Fatalf("expected cursor 0 (clamped), got %d", m.cursor)
	}
}

func TestTUI_EnterShowsDetail(t *testing.T) {
	s, err := store.OpenInMemory()
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

	_, err = s.CreateIssue("Detail issue", "bug", "some body", nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newModel(s)
	if m.detail != nil {
		t.Fatal("expected no detail initially")
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.detail == nil {
		t.Fatal("expected detail to be set after Enter")
	}
	if m.detail.Title != "Detail issue" {
		t.Fatalf("expected title 'Detail issue', got %q", m.detail.Title)
	}
	if m.detail.Body != "some body" {
		t.Fatalf("expected body 'some body', got %q", m.detail.Body)
	}
	if m.detail.Kind != "bug" {
		t.Fatalf("expected kind 'bug', got %q", m.detail.Kind)
	}
}
