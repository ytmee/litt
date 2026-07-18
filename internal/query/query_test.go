package query

import (
	"fmt"
	"testing"

	"github.com/ytmee/litt/internal/store"
)

type mockReader struct {
	issues        []store.Issue
	blockedBy     map[int][]store.Issue
	blocking      map[int][]store.Issue
	blockErr      error
}

func (m *mockReader) ListBlockedBy(issueID int) ([]store.Issue, error) {
	if m.blockErr != nil {
		return nil, m.blockErr
	}
	return m.blockedBy[issueID], nil
}

func (m *mockReader) ListBlocking(issueID int) ([]store.Issue, error) {
	if m.blockErr != nil {
		return nil, m.blockErr
	}
	return m.blocking[issueID], nil
}

func (m *mockReader) ListIssues(state, kind, label string) ([]store.Issue, error) {
	if m.blockErr != nil {
		return nil, m.blockErr
	}
	var filtered []store.Issue
	for _, issue := range m.issues {
		if state != "" && issue.State != state {
			continue
		}
		if kind != "" && issue.Kind != kind {
			continue
		}
		if label != "" {
			hasLabel := false
			for _, l := range issue.Labels {
				if l.Name == label {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				continue
			}
		}
		filtered = append(filtered, issue)
	}
	return filtered, nil
}

func issue(id int, state, kind string, labels ...string) store.Issue {
	var ls []store.Label
	for _, name := range labels {
		ls = append(ls, store.Label{Name: name})
	}
	return store.Issue{
		ID:     id,
		Title:  fmt.Sprintf("Issue %d", id),
		State:  state,
		Kind:   kind,
		Labels: ls,
	}
}

func TestListIssues_Empty(t *testing.T) {
	r := &mockReader{}
	got, err := ListIssues(r, Params{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(got))
	}
}

func TestListIssues_FilterState(t *testing.T) {
	r := &mockReader{
		issues: []store.Issue{issue(1, "open", "task"), issue(2, "closed", "bug")},
	}
	got, err := ListIssues(r, Params{State: "open"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("expected issue 1, got %v", got)
	}
}

func TestListIssues_FilterStateKind(t *testing.T) {
	r := &mockReader{
		issues: []store.Issue{
			issue(1, "open", "task"),
			issue(2, "open", "bug"),
			issue(3, "closed", "task"),
		},
	}
	got, err := ListIssues(r, Params{State: "open", Kind: "bug"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("expected issue 2, got %v", got)
	}
}

func TestListIssues_FilterLabel(t *testing.T) {
	r := &mockReader{
		issues: []store.Issue{
			issue(1, "open", "task", "enhancement"),
			issue(2, "open", "task", "bug"),
		},
	}
	got, err := ListIssues(r, Params{Label: "enhancement"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("expected issue 1, got %v", got)
	}
}

func TestListIssues_BlocksIssue(t *testing.T) {
	r := &mockReader{
		blockedBy: map[int][]store.Issue{
			2: {issue(1, "open", "task")},
		},
	}
	got, err := ListIssues(r, Params{BlocksIssue: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("expected issue 1, got %v", got)
	}
}

func TestListIssues_BlockedByIssue(t *testing.T) {
	r := &mockReader{
		blocking: map[int][]store.Issue{
			1: {issue(2, "open", "task")},
		},
	}
	got, err := ListIssues(r, Params{BlockedByIssue: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("expected issue 2, got %v", got)
	}
}

func TestListIssues_MutuallyExclusiveBlockFilters(t *testing.T) {
	_, err := ListIssues(&mockReader{}, Params{BlocksIssue: 1, BlockedByIssue: 2})
	if err == nil {
		t.Fatal("expected error for mutually exclusive block filters")
	}
}

func TestListIssues_BlocksIssueWithPostFilter(t *testing.T) {
	r := &mockReader{
		blockedBy: map[int][]store.Issue{
			3: {
				issue(1, "open", "task", "enhancement"),
				issue(2, "closed", "bug"),
			},
		},
	}
	got, err := ListIssues(r, Params{BlocksIssue: 3, State: "open"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("expected issue 1, got %v", got)
	}
}

func TestListIssues_IsBlockedTrue(t *testing.T) {
	r := &mockReader{
		issues: []store.Issue{issue(1, "open", "task"), issue(2, "open", "task")},
		blockedBy: map[int][]store.Issue{
			1: {issue(3, "open", "task")},
			2: {},
		},
	}
	blocked := true
	got, err := ListIssues(r, Params{IsBlocked: &blocked})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("expected issue 1 (blocked), got %v", got)
	}
}

func TestListIssues_IsBlockedFalse(t *testing.T) {
	r := &mockReader{
		issues: []store.Issue{issue(1, "open", "task"), issue(2, "open", "task")},
		blockedBy: map[int][]store.Issue{
			1: {issue(3, "open", "task")},
			2: {},
		},
	}
	blocked := false
	got, err := ListIssues(r, Params{IsBlocked: &blocked})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("expected issue 2 (not blocked), got %v", got)
	}
}

func TestListIssues_ReaderError(t *testing.T) {
	_, err := ListIssues(&mockReader{blockErr: fmt.Errorf("db error")}, Params{})
	if err == nil {
		t.Fatal("expected error from reader")
	}
}

func TestListReady_ReturnsOnlyUnblockedReadyIssues(t *testing.T) {
	r := &mockReader{
		issues: []store.Issue{
			issue(1, "open", "task", "ready-for-agent"),
			issue(2, "open", "task", "ready-for-agent"),
			issue(3, "open", "task"), // no ready-for-agent label
		},
		blockedBy: map[int][]store.Issue{
			1: {issue(4, "open", "task")},
			2: {},
			3: {},
		},
	}
	got, err := ListReady(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("expected issue 2 (ready, not blocked), got %v", got)
	}
}

func TestListReady_ClosedBlockerDoesNotBlock(t *testing.T) {
	r := &mockReader{
		issues: []store.Issue{
			issue(1, "open", "task", "ready-for-agent"),
		},
		blockedBy: map[int][]store.Issue{
			1: {issue(2, "closed", "task")},
		},
	}
	got, err := ListReady(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("expected issue 1 (closed blocker does not block), got %v", got)
	}
}
