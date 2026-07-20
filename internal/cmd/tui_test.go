package cmd

import (
	"strings"
	"testing"
)

func TestTUI_Help(t *testing.T) {
	out, err := runCmd(t, "tui", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Interactive TUI issue browser") {
		t.Fatalf("help output missing description: %s", out)
	}
}

func TestTUI_RequiresInit(t *testing.T) {
	dir := t.TempDir()
	defer chdir(t, dir)()

	_, err := runCmd(t, "tui")
	if err == nil {
		t.Fatal("expected error for uninitialized repo")
	}
	if !strings.Contains(err.Error(), "not a litt repository") {
		t.Fatalf("unexpected error: %v", err)
	}
}
