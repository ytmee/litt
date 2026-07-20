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

	m.Update(tea.KeyPressMsg{Text: "j"})
	m.Update(tea.KeyPressMsg{Text: "j"})
	if m.cursor != 2 {
		t.Fatalf("expected cursor 2 (clamped), got %d", m.cursor)
	}

	// j at end clamps
	m.cursor = 0
	m.Update(tea.KeyPressMsg{Text: "k"})
	if m.cursor != 0 {
		t.Fatalf("expected cursor 0 (clamped), got %d", m.cursor)
	}
	// k at start clamps
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

func TestTUI_SearchMode(t *testing.T) {
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

	_, err = s.CreateIssue("Test", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newModel(s)

	m.Update(tea.KeyPressMsg{Text: "/"})
	if !m.searchMode {
		t.Fatal("expected searchMode true after /")
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.searchMode {
		t.Fatal("expected searchMode false after Esc")
	}
}

func TestTUI_SearchTitleFilter(t *testing.T) {
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

	_, err = s.CreateIssue("Alpha Bug", "bug", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Beta Feature", "spec", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Gamma Task", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newModel(s)
	if len(m.issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(m.issues))
	}

	m.Update(tea.KeyPressMsg{Text: "/"})
	if !m.searchMode {
		t.Fatal("expected search mode after /")
	}

	m.Update(tea.KeyPressMsg{Text: "A"})
	m.Update(tea.KeyPressMsg{Text: "l"})
	if m.searchQuery != "Al" {
		t.Fatalf("expected searchQuery 'Al', got %q", m.searchQuery)
	}
	if len(m.issues) != 1 {
		t.Fatalf("expected 1 issue matching 'Al', got %d", len(m.issues))
	}
	if m.issues[0].Title != "Alpha Bug" {
		t.Fatalf("expected filtered issue 'Alpha Bug', got %q", m.issues[0].Title)
	}

	m.Update(tea.KeyPressMsg{Text: "p"})
	if m.searchQuery != "Alp" {
		t.Fatalf("expected searchQuery 'Alp', got %q", m.searchQuery)
	}
	if len(m.issues) != 1 {
		t.Fatalf("expected 1 issue matching 'Alp', got %d", len(m.issues))
	}

	// Searching for "Gamma" should match case-insensitively
	m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m.Update(tea.KeyPressMsg{Text: "/"})
	m.Update(tea.KeyPressMsg{Text: "G"})
	m.Update(tea.KeyPressMsg{Text: "a"})
	m.Update(tea.KeyPressMsg{Text: "m"})
	if m.searchQuery != "Gam" {
		t.Fatalf("expected searchQuery 'Gam', got %q", m.searchQuery)
	}
	if len(m.issues) != 1 {
		t.Fatalf("expected 1 issue matching 'Gam', got %d", len(m.issues))
	}
	if m.issues[0].Title != "Gamma Task" {
		t.Fatalf("expected 'Gamma Task', got %q", m.issues[0].Title)
	}

	// Lowercase "gam" should also match
	m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m.Update(tea.KeyPressMsg{Text: "/"})
	m.Update(tea.KeyPressMsg{Text: "g"})
	m.Update(tea.KeyPressMsg{Text: "a"})
	m.Update(tea.KeyPressMsg{Text: "m"})
	if len(m.issues) != 1 {
		t.Fatalf("expected 1 issue matching 'gam', got %d", len(m.issues))
	}
	if m.issues[0].Title != "Gamma Task" {
		t.Fatalf("expected 'Gamma Task', got %q", m.issues[0].Title)
	}

	// Nothing matching
	m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m.Update(tea.KeyPressMsg{Text: "/"})
	m.Update(tea.KeyPressMsg{Text: "x"})
	m.Update(tea.KeyPressMsg{Text: "y"})
	m.Update(tea.KeyPressMsg{Text: "z"})
	if len(m.issues) != 0 {
		t.Fatalf("expected 0 issues matching 'xyz', got %d", len(m.issues))
	}
}

func TestTUI_SearchBackspace(t *testing.T) {
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

	_, err = s.CreateIssue("Alpha", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Beta", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newModel(s)
	m.Update(tea.KeyPressMsg{Text: "/"})
	m.Update(tea.KeyPressMsg{Text: "A"})
	m.Update(tea.KeyPressMsg{Text: "l"})
	m.Update(tea.KeyPressMsg{Text: "p"})
	if m.searchQuery != "Alp" {
		t.Fatalf("expected searchQuery 'Alp', got %q", m.searchQuery)
	}
	if len(m.issues) != 1 {
		t.Fatalf("expected 1 issue matching 'Alp', got %d", len(m.issues))
	}

	m.Update(tea.KeyPressMsg{Text: "backspace"})
	if m.searchQuery != "Al" {
		t.Fatalf("expected searchQuery 'Al' after backspace, got %q", m.searchQuery)
	}
	if len(m.issues) != 1 {
		t.Fatalf("expected 1 issue matching 'Al', got %d", len(m.issues))
	}

	m.Update(tea.KeyPressMsg{Text: "backspace"})
	m.Update(tea.KeyPressMsg{Text: "backspace"})
	if m.searchQuery != "" {
		t.Fatalf("expected empty searchQuery, got %q", m.searchQuery)
	}
	if len(m.issues) != 2 {
		t.Fatalf("expected 2 issues after clearing search, got %d", len(m.issues))
	}
}

func TestTUI_FilterState(t *testing.T) {
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

	_, err = s.CreateIssue("Open issue", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Closed issue", "bug", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CloseIssue(2); err != nil {
		t.Fatal(err)
	}

	m := newModel(s)
	if len(m.issues) != 2 {
		t.Fatalf("expected 2 issues initially, got %d", len(m.issues))
	}

	m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if m.filterState != "open" {
		t.Fatalf("expected filterState 'open', got %q", m.filterState)
	}
	if len(m.issues) != 1 {
		t.Fatalf("expected 1 open issue, got %d", len(m.issues))
	}
	if m.issues[0].Title != "Open issue" {
		t.Fatalf("expected 'Open issue', got %q", m.issues[0].Title)
	}

	m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if m.filterState != "closed" {
		t.Fatalf("expected filterState 'closed', got %q", m.filterState)
	}
	if len(m.issues) != 1 {
		t.Fatalf("expected 1 closed issue, got %d", len(m.issues))
	}

	m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if m.filterState != "" {
		t.Fatalf("expected filterState '', got %q", m.filterState)
	}
	if len(m.issues) != 2 {
		t.Fatalf("expected 2 issues after clearing filter, got %d", len(m.issues))
	}
}

func TestTUI_FilterKind(t *testing.T) {
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

	_, err = s.CreateIssue("Spec item", "spec", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Task item", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Bug item", "bug", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newModel(s)

	m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if m.filterKind != "spec" {
		t.Fatalf("expected filterKind 'spec', got %q", m.filterKind)
	}
	if len(m.issues) != 1 {
		t.Fatalf("expected 1 spec issue, got %d", len(m.issues))
	}
	if m.issues[0].Title != "Spec item" {
		t.Fatalf("expected 'Spec item', got %q", m.issues[0].Title)
	}

	m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if m.filterKind != "task" {
		t.Fatalf("expected filterKind 'task', got %q", m.filterKind)
	}
	if len(m.issues) != 1 {
		t.Fatalf("expected 1 task issue, got %d", len(m.issues))
	}

	m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if m.filterKind != "bug" {
		t.Fatalf("expected filterKind 'bug', got %q", m.filterKind)
	}

	m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if m.filterKind != "" {
		t.Fatalf("expected filterKind '', got %q", m.filterKind)
	}
}

func TestTUI_FilterLabel(t *testing.T) {
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

	_, err = s.CreateIssue("Enhanced task", "task", "", []string{"enhancement"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Plain task", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newModel(s)

	m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	if m.filterLabel != "needs-triage" {
		t.Fatalf("expected filterLabel 'needs-triage' (first label), got %q", m.filterLabel)
	}
	if len(m.issues) != 0 {
		t.Fatalf("expected 0 issues with label 'needs-triage', got %d", len(m.issues))
	}

	// Cycle through to "enhancement"
	m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	if m.filterLabel != "enhancement" {
		t.Fatalf("expected filterLabel 'enhancement', got %q", m.filterLabel)
	}
	if len(m.issues) != 1 {
		t.Fatalf("expected 1 issue with label 'enhancement', got %d", len(m.issues))
	}
	if m.issues[0].Title != "Enhanced task" {
		t.Fatalf("expected 'Enhanced task', got %q", m.issues[0].Title)
	}
}

func TestTUI_FilterCombined(t *testing.T) {
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

	_, err = s.CreateIssue("Alpha open bug", "bug", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Beta open task", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Gamma open task", "task", "", []string{"enhancement"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Delta closed task", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CloseIssue(4); err != nil {
		t.Fatal(err)
	}

	m := newModel(s)
	if len(m.issues) != 4 {
		t.Fatalf("expected 4 issues, got %d", len(m.issues))
	}

	m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if m.filterState != "open" {
		t.Fatalf("expected filterState 'open', got %q", m.filterState)
	}
	if len(m.issues) != 3 {
		t.Fatalf("expected 3 open issues, got %d", len(m.issues))
	}

	m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if m.filterKind != "task" {
		t.Fatalf("expected filterKind 'task', got %q", m.filterKind)
	}
	if len(m.issues) != 2 {
		t.Fatalf("expected 2 open task issues, got %d", len(m.issues))
	}

	// Open search and type "Gamma"
	m.Update(tea.KeyPressMsg{Text: "/"})
	m.Update(tea.KeyPressMsg{Text: "G"})
	if len(m.issues) != 1 {
		t.Fatalf("expected 1 issue matching all filters, got %d", len(m.issues))
	}
	if m.issues[0].Title != "Gamma open task" {
		t.Fatalf("expected 'Gamma open task', got %q", m.issues[0].Title)
	}

	// Esc clears all filters
	m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.searchMode {
		t.Fatal("expected searchMode false after Esc")
	}
	if m.filterState != "" {
		t.Fatalf("expected filterState cleared, got %q", m.filterState)
	}
	if m.filterKind != "" {
		t.Fatalf("expected filterKind cleared, got %q", m.filterKind)
	}
	if len(m.issues) != 4 {
		t.Fatalf("expected all 4 issues restored, got %d", len(m.issues))
	}
}

func TestTUI_EscRestoresFullList(t *testing.T) {
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

	_, err = s.CreateIssue("One", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Two", "spec", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Three", "bug", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newModel(s)

	m.Update(tea.KeyPressMsg{Text: "/"})
	m.Update(tea.KeyPressMsg{Text: "O"})
	m.Update(tea.KeyPressMsg{Text: "n"})
	m.Update(tea.KeyPressMsg{Text: "e"})
	if len(m.issues) != 1 {
		t.Fatalf("expected 1 issue after search, got %d", len(m.issues))
	}
	if m.issues[0].Title != "One" {
		t.Fatalf("expected 'One', got %q", m.issues[0].Title)
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.searchMode {
		t.Fatal("expected searchMode false after Esc")
	}
	if len(m.issues) != 3 {
		t.Fatalf("expected all 3 issues restored, got %d", len(m.issues))
	}
}

func TestTUI_SearchModeDoesNotQuit(t *testing.T) {
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

	_, err = s.CreateIssue("Test", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	m := newModel(s)
	m.Update(tea.KeyPressMsg{Text: "/"})
	_, cmd := m.Update(tea.KeyPressMsg{Text: "q"})
	if cmd != nil {
		t.Fatal("expected no quit command when in search mode")
	}
	if m.searchQuery != "q" {
		t.Fatalf("expected 'q' appended to search query, got %q", m.searchQuery)
	}
}
