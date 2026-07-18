package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	root.SetArgs(args)
	var out bytes.Buffer
	root.SetOut(&out)
	err := root.Execute()
	return out.String(), err
}

func chdir(t *testing.T, dir string) func() {
	t.Helper()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInit_CreatesDotLitt(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	out, err := runCmd(t, "init")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "Initialized") {
		t.Fatalf("unexpected output: %s", out)
	}

	if _, err := os.Stat(".litt/litt.db"); os.IsNotExist(err) {
		t.Fatal(".litt/litt.db not created")
	}
}

func TestInit_Idempotent(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, "init")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Initialized") {
		t.Fatalf("second init unexpected output: %s", out)
	}
}

func TestInit_AddsGitignore(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), ".litt/") {
		t.Fatal(".gitignore does not contain .litt/")
	}
}

func TestInit_GitignoreIdempotent(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}
	count := strings.Count(string(data), ".litt/")
	if count != 1 {
		t.Fatalf("expected 1 .litt/ entry, got %d", count)
	}
}

func TestInit_AppendsToExistingGitignore(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if err := os.WriteFile(".gitignore", []byte("*.log\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, ".litt/") {
		t.Fatal(".gitignore does not contain .litt/")
	}
	if !strings.Contains(content, "*.log") {
		t.Fatal(".gitignore lost existing content")
	}
}

func TestLabelList_PrintsLabels(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "label", "list")
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"needs-triage", "needs-info", "ready-for-agent", "ready-for-human", "wontfix", "enhancement"}
	for _, name := range expected {
		if !strings.Contains(out, name) {
			t.Errorf("output missing label %q", name)
		}
	}
}

func TestLabelList_JSON(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "label", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Fatal("JSON output does not start with [")
	}
	if !strings.Contains(out, `"name": "needs-triage"`) {
		t.Fatal("JSON output missing needs-triage")
	}
	if !strings.Contains(out, `"kind": "triage"`) {
		t.Fatal("JSON output missing kind field")
	}
}

func TestLabelList_NoInit(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	_, err := runCmd(t, "label", "list")
	if err == nil {
		t.Fatal("expected error for uninitialized repo")
	}
}

func TestAgentInstructions_PrintsBlock(t *testing.T) {
	out, err := runCmd(t, "agent", "instructions")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, managedBlockStart) {
		t.Fatal("output missing managed block start marker")
	}
	if !strings.Contains(out, "local issue tracker") {
		t.Fatal("output missing expected content")
	}
}

func TestAgentInstall_CreatesFile(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "AGENTS.md")
	out, err := runCmd(t, "agent", "install", "--target", target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Installed") {
		t.Fatalf("unexpected output: %s", out)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), managedBlockStart) {
		t.Fatal("AGENTS.md missing managed block start marker")
	}
}

func TestAgentInstall_Idempotent(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "AGENTS.md")

	if _, err := runCmd(t, "agent", "install", "--target", target); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "agent", "install", "--target", target); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	count := strings.Count(string(data), managedBlockStart)
	if count != 1 {
		t.Fatalf("expected 1 block after second install, got %d", count)
	}
}

func TestAgentInstall_ReplacesBlock(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "AGENTS.md")
	oldBlock := managedBlockStart + "old content" + managedBlockEnd
	if err := os.WriteFile(target, []byte(oldBlock), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := runCmd(t, "agent", "install", "--target", target); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "old content") {
		t.Fatal("old content was not replaced")
	}
	if !strings.Contains(string(data), "local issue tracker") {
		t.Fatal("block was not updated")
	}
}

func TestIssueCreate(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "create", "Test issue", "--kind", "task", "--body", "details")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Created issue #1") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestIssueCreateWithLabels(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "create", "Bug fix", "--kind", "task", "--label", "bug", "--label", "ready-for-agent")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Created issue #1") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestIssueCreateImplicitLabel(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "create", "Custom", "--label", "my-custom-label")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Created issue #1") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestIssueListShowsOpenByDefault(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Open issue"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Open issue") {
		t.Fatalf("expected open issue in output: %s", out)
	}
}

func TestIssueListDoesNotShowClosedByDefault(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Open issue"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "close", "1"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "list")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Open issue") {
		t.Fatal("closed issue should not appear in default list")
	}
}

func TestIssueListFilterState(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Issue 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Issue 2"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "close", "1"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "list", "--state", "closed")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Issue 1") {
		t.Fatalf("expected closed issue in output: %s", out)
	}
	if strings.Contains(out, "Issue 2") {
		t.Fatal("open issue should not appear in closed list")
	}
}

func TestIssueListFilterKind(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Task", "--kind", "task"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Spec", "--kind", "spec"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "list", "--kind", "spec")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Spec") {
		t.Fatalf("expected spec in output: %s", out)
	}
	if strings.Contains(out, "Task") {
		t.Fatal("task should not appear in spec filter")
	}
}

func TestIssueListFilterLabel(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Buggy", "--label", "bug"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Nice", "--label", "enhancement"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "list", "--label", "bug")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Buggy") {
		t.Fatalf("expected buggy in output: %s", out)
	}
	if strings.Contains(out, "Nice") {
		t.Fatal("enhancement issue should not appear in bug filter")
	}
}

func TestIssueListJSON(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Test issue", "--label", "bug"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Fatal("JSON output does not start with [")
	}
	if !strings.Contains(out, `"title": "Test issue"`) {
		t.Fatal("JSON output missing title")
	}
	if !strings.Contains(out, `"name": "bug"`) {
		t.Fatal("JSON output missing label")
	}
}

func TestIssueShow(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Test issue", "--body", "details", "--label", "bug"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "show", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Test issue") {
		t.Fatal("show output missing title")
	}
	if !strings.Contains(out, "details") {
		t.Fatal("show output missing body")
	}
	if !strings.Contains(out, "#1") {
		t.Fatal("show output missing issue ID")
	}
	if !strings.Contains(out, "bug") {
		t.Fatal("show output missing label")
	}
}

func TestIssueShowWithHash(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Test issue"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "show", "#1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Test issue") {
		t.Fatal("show with hash prefix failed")
	}
}

func TestIssueShowJSON(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Test issue"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "show", "1", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatal("JSON output does not start with {")
	}
	if !strings.Contains(out, `"title": "Test issue"`) {
		t.Fatal("JSON output missing title")
	}
}

func TestIssueUpdateTitle(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Original"); err != nil {
		t.Fatal(err)
	}

	_, err := runCmd(t, "issue", "update", "1", "--title", "Updated")
	if err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "show", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Updated") {
		t.Fatal("title was not updated")
	}
}

func TestIssueUpdateAddLabel(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Test"); err != nil {
		t.Fatal(err)
	}

	_, err := runCmd(t, "issue", "update", "1", "--add-label", "bug", "--add-label", "enhancement")
	if err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "show", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "bug") {
		t.Fatal("bug label not added")
	}
	if !strings.Contains(out, "enhancement") {
		t.Fatal("enhancement label not added")
	}
}

func TestIssueUpdateRemoveLabel(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Test", "--label", "bug", "--label", "enhancement"); err != nil {
		t.Fatal(err)
	}

	_, err := runCmd(t, "issue", "update", "1", "--remove-label", "bug")
	if err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "show", "1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "bug") {
		t.Fatal("bug label should have been removed")
	}
	if !strings.Contains(out, "enhancement") {
		t.Fatal("enhancement label should still be present")
	}
}

func TestIssueUpdateTriageMutualExclusion(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Test", "--label", "needs-triage"); err != nil {
		t.Fatal(err)
	}

	_, err := runCmd(t, "issue", "update", "1", "--add-label", "ready-for-agent")
	if err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "show", "1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "needs-triage") {
		t.Fatal("needs-triage should have been replaced by ready-for-agent")
	}
	if !strings.Contains(out, "ready-for-agent") {
		t.Fatal("ready-for-agent should have been added")
	}
}

func TestIssueUpdateImplicitLabelCreation(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Test"); err != nil {
		t.Fatal(err)
	}

	_, err := runCmd(t, "issue", "update", "1", "--add-label", "new-custom-label")
	if err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "show", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "new-custom-label") {
		t.Fatal("custom label should have been created and attached")
	}
}

func TestIssueClose(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Test"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "close", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Closed issue #1") {
		t.Fatalf("unexpected output: %s", out)
	}

	showOut, err := runCmd(t, "issue", "show", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(showOut, "closed") {
		t.Fatal("issue should be closed")
	}
}

func TestIssueReopen(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Test"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "close", "1"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "reopen", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Reopened issue #1") {
		t.Fatalf("unexpected output: %s", out)
	}

	showOut, err := runCmd(t, "issue", "show", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(showOut, "open") {
		t.Fatal("issue should be open after reopen")
	}
}

func TestIssueUpdateStateOpenClearsClosedAt(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Test"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "close", "1"); err != nil {
		t.Fatal(err)
	}

	_, err := runCmd(t, "issue", "update", "1", "--state", "open")
	if err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "show", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "open") {
		t.Fatal("issue should be open")
	}
}

func TestIssueParentSet(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Child"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Parent"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "parent", "set", "1", "2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Set parent of #1 to #2") {
		t.Fatalf("unexpected output: %s", out)
	}

	showOut, err := runCmd(t, "issue", "show", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(showOut, "Parent:  #2") {
		t.Fatalf("expected parent #2 in show output: %s", showOut)
	}
}

func TestIssueParentSetSelf(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Issue"); err != nil {
		t.Fatal(err)
	}

	_, err := runCmd(t, "issue", "parent", "set", "1", "1")
	if err == nil {
		t.Fatal("expected error for setting self as parent")
	}
}

func TestIssueParentSetCycle(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := runCmd(t, "issue", "create", fmt.Sprintf("Issue %d", i+1)); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := runCmd(t, "issue", "parent", "set", "2", "3"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "parent", "set", "1", "2"); err != nil {
		t.Fatal(err)
	}

	_, err := runCmd(t, "issue", "parent", "set", "3", "1")
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestIssueParentClear(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Child"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Parent"); err != nil {
		t.Fatal(err)
	}

	if _, err := runCmd(t, "issue", "parent", "set", "1", "2"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "parent", "clear", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Cleared parent of #1") {
		t.Fatalf("unexpected output: %s", out)
	}

	showOut, err := runCmd(t, "issue", "show", "1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(showOut, "#2") {
		t.Fatal("parent should have been cleared")
	}
}

func TestIssueChildren(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Parent"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Child 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Child 2"); err != nil {
		t.Fatal(err)
	}

	if _, err := runCmd(t, "issue", "parent", "set", "2", "1"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "parent", "set", "3", "1"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "children", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Child 1") {
		t.Fatalf("output missing Child 1: %s", out)
	}
	if !strings.Contains(out, "Child 2") {
		t.Fatalf("output missing Child 2: %s", out)
	}
}

func TestIssueBlock(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Blocker"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Blocked"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "block", "1", "2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "#1 now blocks #2") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestIssueUnblock(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Blocker"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Blocked"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "block", "1", "2"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "unblock", "1", "2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Removed block") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestIssueBlockCycle(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := runCmd(t, "issue", "create", fmt.Sprintf("Issue %d", i+1)); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := runCmd(t, "issue", "block", "1", "2"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "block", "2", "3"); err != nil {
		t.Fatal(err)
	}

	_, err := runCmd(t, "issue", "block", "3", "1")
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestIssueBlockSelf(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Issue"); err != nil {
		t.Fatal(err)
	}

	_, err := runCmd(t, "issue", "block", "1", "1")
	if err == nil {
		t.Fatal("expected error for self-block")
	}
}

func TestIssueBlockedBy(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Blocker"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Blocked"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "block", "1", "2"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "blocked-by", "2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "#2 is blocked by") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "#1") {
		t.Fatalf("expected blocker #1 in output: %s", out)
	}
}

func TestIssueBlocking(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Blocker"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Blocked"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "block", "1", "2"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "blocking", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "#1 blocks") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "#2") {
		t.Fatalf("expected blocked #2 in output: %s", out)
	}
}

func TestIssueReady(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Ready issue", "--label", "ready-for-agent"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "ready")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Ready issue") {
		t.Fatalf("expected ready issue in output: %s", out)
	}
}

func TestIssueReadyJSON(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Ready issue", "--label", "ready-for-agent"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "ready", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Fatal("JSON output does not start with [")
	}
	if !strings.Contains(out, `"title": "Ready issue"`) {
		t.Fatal("JSON output missing title")
	}
}

func TestIssueReadyBlockedByOpen(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Blocker", "--label", "ready-for-agent"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Blocked", "--label", "ready-for-agent"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "block", "1", "2"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "ready")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Blocked") {
		t.Fatal("blocked issue should not appear in ready output")
	}
	if !strings.Contains(out, "Blocker") {
		t.Fatal("blocker should appear in ready output")
	}
}

func TestIssueReadyBlockedByClosed(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Blocker", "--label", "ready-for-agent"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "Blocked", "--label", "ready-for-agent"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "block", "1", "2"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "close", "1"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "ready")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Blocked") {
		t.Fatal("issue blocked by closed issue should appear in ready output")
	}
}

func TestIssueReadyNoLabel(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, "issue", "create", "No label issue"); err != nil {
		t.Fatal(err)
	}

	out, err := runCmd(t, "issue", "ready")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "No ready issues found") {
		t.Fatalf("expected no ready issues: %s", out)
	}
}
func TestInitWithDBFlag(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	customDB := filepath.Join(dir, "custom", "data.db")

	out, err := runCmd(t, "init", "--db", customDB)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Initialized") {
		t.Fatalf("unexpected output: %s", out)
	}
	if _, err := os.Stat(customDB); os.IsNotExist(err) {
		t.Fatal("custom db was not created")
	}
}

func TestUpwardDiscovery(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	if _, err := runCmd(t, "init"); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer chdir(t, subDir)()

	out, err := runCmd(t, "issue", "list")
	if err != nil {
		t.Fatalf("expected success from subdirectory, got: %v", err)
	}
	if !strings.Contains(out, "No issues found") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestDBFlagOverridesDiscovery(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	customDir := filepath.Join(dir, "custom")
	customDB := filepath.Join(customDir, "data.db")

	if _, err := runCmd(t, "init", "--db", customDB); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer chdir(t, subDir)()

	out, err := runCmd(t, "issue", "list", "--db", customDB)
	if err != nil {
		t.Fatalf("expected success with --db, got: %v", err)
	}
	if !strings.Contains(out, "No issues found") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestIssueNoInit(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	_, err := runCmd(t, "issue", "list")
	if err == nil {
		t.Fatal("expected error for uninitialized repo")
	}
}
