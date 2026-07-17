package cmd

import (
	"bytes"
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

	expected := []string{"needs-triage", "needs-info", "ready-for-agent", "ready-for-human", "wontfix", "bug", "enhancement"}
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
	if !strings.Contains(out, "litt init") {
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
	if !strings.Contains(string(data), "managed by litt") {
		t.Fatal("block was not updated")
	}
}
