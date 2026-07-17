package store

import (
	"fmt"
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
	if count != 2 {
		t.Fatalf("expected 2 migrations, got %d", count)
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
	if count != 2 {
		t.Fatalf("expected 2 migrations after second run, got %d", count)
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

func setup(t *testing.T) *Store {
	t.Helper()
	s, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedLabels(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCreateIssue(t *testing.T) {
	s := setup(t)
	defer s.Close()

	issue, err := s.CreateIssue("Test issue", "task", "details", nil)
	if err != nil {
		t.Fatal(err)
	}
	if issue.ID != 1 {
		t.Fatalf("expected id 1, got %d", issue.ID)
	}
	if issue.Title != "Test issue" {
		t.Fatalf("expected title %q, got %q", "Test issue", issue.Title)
	}
	if issue.Body != "details" {
		t.Fatalf("expected body %q, got %q", "details", issue.Body)
	}
	if issue.State != "open" {
		t.Fatalf("expected state %q, got %q", "open", issue.State)
	}
	if issue.Kind != "task" {
		t.Fatalf("expected kind %q, got %q", "task", issue.Kind)
	}
	if issue.ClosedAt != nil {
		t.Fatal("expected closed_at to be nil")
	}
}

func TestCreateIssueWithLabels(t *testing.T) {
	s := setup(t)
	defer s.Close()

	issue, err := s.CreateIssue("Feature", "feature", "", []string{"bug", "enhancement"})
	if err != nil {
		t.Fatal(err)
	}
	if issue.ID != 1 {
		t.Fatalf("expected id 1, got %d", issue.ID)
	}
	if len(issue.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(issue.Labels))
	}
}

func TestCreateIssueImplicitLabelCreation(t *testing.T) {
	s := setup(t)
	defer s.Close()

	issue, err := s.CreateIssue("Custom label test", "task", "", []string{"unknown-label"})
	if err != nil {
		t.Fatal(err)
	}
	if len(issue.Labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(issue.Labels))
	}
	if issue.Labels[0].Name != "unknown-label" {
		t.Fatalf("expected label name %q, got %q", "unknown-label", issue.Labels[0].Name)
	}
	if issue.Labels[0].Kind != "custom" {
		t.Fatalf("expected label kind %q, got %q", "custom", issue.Labels[0].Kind)
	}
}

func TestGetIssue(t *testing.T) {
	s := setup(t)
	defer s.Close()

	created, err := s.CreateIssue("Test", "task", "body", []string{"bug"})
	if err != nil {
		t.Fatal(err)
	}

	issue, err := s.GetIssue(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if issue.Title != "Test" {
		t.Fatalf("expected title %q, got %q", "Test", issue.Title)
	}
	if len(issue.Labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(issue.Labels))
	}
	if issue.Labels[0].Name != "bug" {
		t.Fatalf("expected label %q, got %q", "bug", issue.Labels[0].Name)
	}
}

func TestGetIssueNotFound(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.GetIssue(999)
	if err == nil {
		t.Fatal("expected error for non-existent issue")
	}
}

func TestListIssuesDefault(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Issue 1", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Issue 2", "feature", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	issues, err := s.ListIssues("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
}

func TestListIssuesFilterState(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Issue 1", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CloseIssue(1); err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Issue 2", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	openIssues, err := s.ListIssues("open", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(openIssues) != 1 {
		t.Fatalf("expected 1 open issue, got %d", len(openIssues))
	}

	closedIssues, err := s.ListIssues("closed", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(closedIssues) != 1 {
		t.Fatalf("expected 1 closed issue, got %d", len(closedIssues))
	}
}

func TestListIssuesFilterKind(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Task", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Feature", "feature", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	features, err := s.ListIssues("", "feature", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(features) != 1 {
		t.Fatalf("expected 1 feature issue, got %d", len(features))
	}
}

func TestListIssuesFilterLabel(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Bug", "task", "", []string{"bug"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Feature", "feature", "", []string{"enhancement"})
	if err != nil {
		t.Fatal(err)
	}

	bugIssues, err := s.ListIssues("", "", "bug")
	if err != nil {
		t.Fatal(err)
	}
	if len(bugIssues) != 1 {
		t.Fatalf("expected 1 issue with bug label, got %d", len(bugIssues))
	}
}

func TestUpdateIssueTitle(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Original", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	title := "Updated"
	err = s.UpdateIssue(1, UpdateIssueOptions{Title: &title})
	if err != nil {
		t.Fatal(err)
	}

	issue, err := s.GetIssue(1)
	if err != nil {
		t.Fatal(err)
	}
	if issue.Title != "Updated" {
		t.Fatalf("expected title %q, got %q", "Updated", issue.Title)
	}
}

func TestUpdateIssueStateClosed(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Test", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	state := "closed"
	err = s.UpdateIssue(1, UpdateIssueOptions{State: &state})
	if err != nil {
		t.Fatal(err)
	}

	issue, err := s.GetIssue(1)
	if err != nil {
		t.Fatal(err)
	}
	if issue.State != "closed" {
		t.Fatalf("expected state %q, got %q", "closed", issue.State)
	}
	if issue.ClosedAt == nil {
		t.Fatal("expected closed_at to be set")
	}
}

func TestUpdateIssueStateOpen(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Test", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CloseIssue(1); err != nil {
		t.Fatal(err)
	}

	state := "open"
	err = s.UpdateIssue(1, UpdateIssueOptions{State: &state})
	if err != nil {
		t.Fatal(err)
	}

	issue, err := s.GetIssue(1)
	if err != nil {
		t.Fatal(err)
	}
	if issue.State != "open" {
		t.Fatalf("expected state %q, got %q", "open", issue.State)
	}
	if issue.ClosedAt != nil {
		t.Fatal("expected closed_at to be cleared")
	}
}

func TestCloseIssue(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Test", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.CloseIssue(1); err != nil {
		t.Fatal(err)
	}

	issue, err := s.GetIssue(1)
	if err != nil {
		t.Fatal(err)
	}
	if issue.State != "closed" {
		t.Fatalf("expected state %q, got %q", "closed", issue.State)
	}
	if issue.ClosedAt == nil {
		t.Fatal("expected closed_at to be set")
	}
}

func TestReopenIssue(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Test", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CloseIssue(1); err != nil {
		t.Fatal(err)
	}
	if err := s.ReopenIssue(1); err != nil {
		t.Fatal(err)
	}

	issue, err := s.GetIssue(1)
	if err != nil {
		t.Fatal(err)
	}
	if issue.State != "open" {
		t.Fatalf("expected state %q, got %q", "open", issue.State)
	}
	if issue.ClosedAt != nil {
		t.Fatal("expected closed_at to be cleared after reopen")
	}
}

func TestUpdateIssueLabelsAdd(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Test", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateIssueLabels(1, []string{"bug", "enhancement"}, nil); err != nil {
		t.Fatal(err)
	}

	issue, err := s.GetIssue(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(issue.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(issue.Labels))
	}
}

func TestUpdateIssueLabelsRemove(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Test", "task", "", []string{"bug", "enhancement"})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateIssueLabels(1, nil, []string{"bug"}); err != nil {
		t.Fatal(err)
	}

	issue, err := s.GetIssue(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(issue.Labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(issue.Labels))
	}
	if issue.Labels[0].Name != "enhancement" {
		t.Fatalf("expected remaining label %q, got %q", "enhancement", issue.Labels[0].Name)
	}
}

func TestUpdateIssueLabelsTriageMutualExclusion(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Test", "task", "", []string{"needs-triage"})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateIssueLabels(1, []string{"ready-for-agent"}, nil); err != nil {
		t.Fatal(err)
	}

	issue, err := s.GetIssue(1)
	if err != nil {
		t.Fatal(err)
	}

	hasNeedsTriage := false
	hasReadyForAgent := false
	for _, l := range issue.Labels {
		if l.Name == "needs-triage" {
			hasNeedsTriage = true
		}
		if l.Name == "ready-for-agent" {
			hasReadyForAgent = true
		}
	}
	if hasNeedsTriage {
		t.Fatal("needs-triage should have been removed by triage mutual exclusion")
	}
	if !hasReadyForAgent {
		t.Fatal("ready-for-agent should have been added")
	}
}

func TestResolveLabelCreatesCustom(t *testing.T) {
	s := setup(t)
	defer s.Close()

	label, err := s.ResolveLabel("my-custom-label")
	if err != nil {
		t.Fatal(err)
	}
	if label.Name != "my-custom-label" {
		t.Fatalf("expected name %q, got %q", "my-custom-label", label.Name)
	}
	if label.Kind != "custom" {
		t.Fatalf("expected kind %q, got %q", "custom", label.Kind)
	}
}

func TestResolveLabelExisting(t *testing.T) {
	s := setup(t)
	defer s.Close()

	label, err := s.ResolveLabel("bug")
	if err != nil {
		t.Fatal(err)
	}
	if label.Name != "bug" {
		t.Fatalf("expected name %q, got %q", "bug", label.Name)
	}
	if label.Kind != "category" {
		t.Fatalf("expected kind %q, got %q", "category", label.Kind)
	}
}

func TestCloseIssueNotFound(t *testing.T) {
	s := setup(t)
	defer s.Close()

	err := s.CloseIssue(999)
	if err == nil {
		t.Fatal("expected error for non-existent issue")
	}
}

func TestSetParent(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Child", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Parent", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.SetParent(1, 2); err != nil {
		t.Fatal(err)
	}

	issue, err := s.GetIssue(1)
	if err != nil {
		t.Fatal(err)
	}
	if issue.ParentIssueID == nil || *issue.ParentIssueID != 2 {
		t.Fatalf("expected parent_issue_id 2, got %v", issue.ParentIssueID)
	}
}

func TestSetParentSelf(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Issue", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	err = s.SetParent(1, 1)
	if err == nil {
		t.Fatal("expected error for setting self as parent")
	}
}

func TestSetParentCycle(t *testing.T) {
	s := setup(t)
	defer s.Close()

	for i := 0; i < 3; i++ {
		_, err := s.CreateIssue(fmt.Sprintf("Issue %d", i+1), "task", "", nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := s.SetParent(2, 3); err != nil {
		t.Fatal(err)
	}
	if err := s.SetParent(1, 2); err != nil {
		t.Fatal(err)
	}

	err := s.SetParent(3, 1)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestClearParent(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Child", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Parent", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.SetParent(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.ClearParent(1); err != nil {
		t.Fatal(err)
	}

	issue, err := s.GetIssue(1)
	if err != nil {
		t.Fatal(err)
	}
	if issue.ParentIssueID != nil {
		t.Fatalf("expected nil parent_issue_id, got %v", *issue.ParentIssueID)
	}
}

func TestListChildren(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Parent", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		_, err := s.CreateIssue(fmt.Sprintf("Child %d", i+1), "task", "", nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, id := range []int{2, 3, 4} {
		if err := s.SetParent(id, 1); err != nil {
			t.Fatal(err)
		}
	}

	children, err := s.ListChildren(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(children))
	}
}

func TestCreateBlock(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Blocker", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Blocked", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.CreateBlock(1, 2); err != nil {
		t.Fatal(err)
	}
}

func TestCreateBlockSelf(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Issue", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	err = s.CreateBlock(1, 1)
	if err == nil {
		t.Fatal("expected error for issue blocking itself")
	}
}

func TestCreateBlockCycle(t *testing.T) {
	s := setup(t)
	defer s.Close()

	for i := 0; i < 3; i++ {
		_, err := s.CreateIssue(fmt.Sprintf("Issue %d", i+1), "task", "", nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := s.CreateBlock(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateBlock(2, 3); err != nil {
		t.Fatal(err)
	}

	err := s.CreateBlock(3, 1)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestRemoveBlock(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Blocker", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Blocked", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.CreateBlock(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveBlock(1, 2); err != nil {
		t.Fatal(err)
	}
}

func TestListBlockedBy(t *testing.T) {
	s := setup(t)
	defer s.Close()

	for i := 0; i < 3; i++ {
		_, err := s.CreateIssue(fmt.Sprintf("Issue %d", i+1), "task", "", nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := s.CreateBlock(1, 3); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateBlock(2, 3); err != nil {
		t.Fatal(err)
	}

	blockers, err := s.ListBlockedBy(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(blockers) != 2 {
		t.Fatalf("expected 2 blockers, got %d", len(blockers))
	}
}

func TestListBlocking(t *testing.T) {
	s := setup(t)
	defer s.Close()

	for i := 0; i < 3; i++ {
		_, err := s.CreateIssue(fmt.Sprintf("Issue %d", i+1), "task", "", nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := s.CreateBlock(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateBlock(1, 3); err != nil {
		t.Fatal(err)
	}

	blocked, err := s.ListBlocking(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocked) != 2 {
		t.Fatalf("expected 2 blocked issues, got %d", len(blocked))
	}
}

func TestListReadyIssues(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Ready issue", "task", "", []string{"ready-for-agent"})
	if err != nil {
		t.Fatal(err)
	}

	issues, err := s.ListReadyIssues()
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 ready issue, got %d", len(issues))
	}
}

func TestListReadyIssuesNotLabeled(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("No label", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	issues, err := s.ListReadyIssues()
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected 0 ready issues, got %d", len(issues))
	}
}

func TestListReadyIssuesClosed(t *testing.T) {
	s := setup(t)
	defer s.Close()

	_, err := s.CreateIssue("Closed issue", "task", "", []string{"ready-for-agent"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CloseIssue(1); err != nil {
		t.Fatal(err)
	}

	issues, err := s.ListReadyIssues()
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected 0 ready issues, got %d", len(issues))
	}
}

func TestListReadyIssuesBlockedByOpen(t *testing.T) {
	s := setup(t)
	defer s.Close()

	for i := 0; i < 2; i++ {
		_, err := s.CreateIssue(fmt.Sprintf("Issue %d", i+1), "task", "", []string{"ready-for-agent"})
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := s.CreateBlock(1, 2); err != nil {
		t.Fatal(err)
	}

	issues, err := s.ListReadyIssues()
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 ready issue (unblocked one), got %d", len(issues))
	}
	if issues[0].ID != 1 {
		t.Fatalf("expected issue 1 to be ready (the blocker), got issue %d", issues[0].ID)
	}
}

func TestListReadyIssuesBlockedByClosed(t *testing.T) {
	s := setup(t)
	defer s.Close()

	for i := 0; i < 2; i++ {
		_, err := s.CreateIssue(fmt.Sprintf("Issue %d", i+1), "task", "", []string{"ready-for-agent"})
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := s.CreateBlock(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseIssue(1); err != nil {
		t.Fatal(err)
	}

	issues, err := s.ListReadyIssues()
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 ready issue (blocked by closed), got %d", len(issues))
	}
	if issues[0].ID != 2 {
		t.Fatalf("expected issue 2 to be ready, got issue %d", issues[0].ID)
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
