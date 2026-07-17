package cmd

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ytmee/litt/internal/store"
)

func mcpTestSetup(t *testing.T) (*store.Store, *mcp.ClientSession, func()) {
	t.Helper()
	s, err := store.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(); err != nil {
		s.Close()
		t.Fatal(err)
	}
	if err := s.SeedLabels(); err != nil {
		s.Close()
		t.Fatal(err)
	}

	server := newMCPServer(s)
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, st, nil)
	if err != nil {
		s.Close()
		t.Fatal(err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1"}, nil)
	clientSession, err := client.Connect(ctx, ct, nil)
	if err != nil {
		serverSession.Close()
		s.Close()
		t.Fatal(err)
	}

	cleanup := func() {
		clientSession.Close()
		serverSession.Close()
		s.Close()
	}

	return s, clientSession, cleanup
}

func mcpToolSuccess(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("call tool %s: %v", name, err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				t.Fatalf("unexpected error from %s: %s", name, tc.Text)
			}
		}
		t.Fatalf("unexpected error from %s", name)
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func mcpToolErr(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return err.Error()
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				return tc.Text
			}
		}
		return "tool error"
	}
	t.Fatalf("expected error from %s, got success", name)
	return ""
}

func TestMCPToolsList(t *testing.T) {
	_, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{
		"create_issue", "update_issue", "query_issues", "get_issue",
		"get_ready_issues", "set_parent", "clear_parent", "add_blocking", "remove_blocking",
	}
	if len(res.Tools) != len(expected) {
		t.Fatalf("expected %d tools, got %d", len(expected), len(res.Tools))
	}
	names := make(map[string]bool)
	for _, tool := range res.Tools {
		names[tool.Name] = true
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestMCPCreateIssue(t *testing.T) {
	_, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	text := mcpToolSuccess(t, session, "create_issue", map[string]any{
		"kind":   "task",
		"title":  "Test issue",
		"body":   "details",
		"labels": []string{"bug"},
	})
	if !strings.Contains(text, `"number":1`) && !strings.Contains(text, `"number": 1`) {
		t.Fatalf("expected number 1 in response, got: %s", text)
	}
	if !strings.Contains(text, `"ref":"#1"`) && !strings.Contains(text, `"ref": "#1"`) {
		t.Fatalf("expected ref #1 in response, got: %s", text)
	}
	if !strings.Contains(text, "Test issue") {
		t.Fatalf("expected title in response, got: %s", text)
	}
}

func TestMCPCreateIssueMissingTitle(t *testing.T) {
	_, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	errMsg := mcpToolErr(t, session, "create_issue", map[string]any{
		"kind": "task",
	})
	if !strings.Contains(errMsg, "title") {
		t.Fatalf("expected error about missing title, got: %s", errMsg)
	}
}

func TestMCPCreateIssueInvalidKind(t *testing.T) {
	_, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	errMsg := mcpToolErr(t, session, "create_issue", map[string]any{
		"kind":  "invalid",
		"title": "Test",
	})
	if !strings.Contains(errMsg, "constraint") && !strings.Contains(errMsg, "kind") {
		t.Fatalf("expected error about invalid kind, got: %s", errMsg)
	}
}

func TestMCPGetIssue(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	_, err := s.CreateIssue("Test", "task", "body", []string{"bug"})
	if err != nil {
		t.Fatal(err)
	}

	text := mcpToolSuccess(t, session, "get_issue", map[string]any{
		"number": 1,
	})
	if !strings.Contains(text, "Test") {
		t.Fatalf("expected title in response, got: %s", text)
	}
	if !strings.Contains(text, "body") {
		t.Fatalf("expected body in response, got: %s", text)
	}
	if !strings.Contains(text, "bug") {
		t.Fatalf("expected label 'bug' in response, got: %s", text)
	}
}

func TestMCPGetIssueNotFound(t *testing.T) {
	_, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	errMsg := mcpToolErr(t, session, "get_issue", map[string]any{
		"number": 999,
	})
	if !strings.Contains(errMsg, "not found") {
		t.Fatalf("expected 'not found' error, got: %s", errMsg)
	}
}

func TestMCPUpdateIssue(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	_, err := s.CreateIssue("Original", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	text := mcpToolSuccess(t, session, "update_issue", map[string]any{
		"number": 1,
		"title":  "Updated",
	})
	if !strings.Contains(text, "Updated") {
		t.Fatalf("expected 'Updated' in response, got: %s", text)
	}
}

func TestMCPUpdateIssueAddLabels(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	_, err := s.CreateIssue("Test", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	text := mcpToolSuccess(t, session, "update_issue", map[string]any{
		"number":     1,
		"add_labels": []string{"bug", "enhancement"},
	})
	if !strings.Contains(text, "bug") || !strings.Contains(text, "enhancement") {
		t.Fatalf("expected labels in response, got: %s", text)
	}
}

func TestMCPUpdateIssueState(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	_, err := s.CreateIssue("Test", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	text := mcpToolSuccess(t, session, "update_issue", map[string]any{
		"number": 1,
		"state":  "closed",
	})
	if !strings.Contains(text, `"state":"closed"`) && !strings.Contains(text, `"state": "closed"`) {
		t.Fatalf("expected state closed in response, got: %s", text)
	}
}

func TestMCPUpdateIssueNotFound(t *testing.T) {
	_, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	errMsg := mcpToolErr(t, session, "update_issue", map[string]any{
		"number": 999,
		"title":  "Test",
	})
	if !strings.Contains(errMsg, "not found") {
		t.Fatalf("expected 'not found' error, got: %s", errMsg)
	}
}

func TestMCPQueryIssues(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	for i := 0; i < 3; i++ {
		_, err := s.CreateIssue(fmt.Sprintf("Issue %d", i+1), "task", "", nil)
		if err != nil {
			t.Fatal(err)
		}
	}

	text := mcpToolSuccess(t, session, "query_issues", map[string]any{})
	if !strings.Contains(text, "Issue 1") || !strings.Contains(text, "Issue 3") {
		t.Fatalf("expected all issues in response, got: %s", text)
	}
}

func TestMCPQueryIssuesFilterState(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	_, err := s.CreateIssue("Open", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Closed", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CloseIssue(2); err != nil {
		t.Fatal(err)
	}

	text := mcpToolSuccess(t, session, "query_issues", map[string]any{
		"state": "closed",
	})
	if !strings.Contains(text, "Closed") {
		t.Fatalf("expected 'Closed' in response, got: %s", text)
	}
	if strings.Contains(text, "Open") {
		t.Fatal("should not contain 'Open'")
	}
}

func TestMCPQueryIssuesFilterLabel(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	_, err := s.CreateIssue("Bug", "task", "", []string{"bug"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Feature", "task", "", []string{"enhancement"})
	if err != nil {
		t.Fatal(err)
	}

	text := mcpToolSuccess(t, session, "query_issues", map[string]any{
		"label": "bug",
	})
	if !strings.Contains(text, "Bug") {
		t.Fatalf("expected 'Bug' in response, got: %s", text)
	}
	if strings.Contains(text, "Feature") {
		t.Fatal("should not contain 'Feature'")
	}
}

func TestMCPQueryIssuesFilterKind(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	_, err := s.CreateIssue("Task", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Feature", "feature", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	text := mcpToolSuccess(t, session, "query_issues", map[string]any{
		"kind": "feature",
	})
	if !strings.Contains(text, "Feature") {
		t.Fatalf("expected 'Feature' in response, got: %s", text)
	}
	if strings.Contains(text, "Task") {
		t.Fatal("should not contain 'Task'")
	}
}

func TestMCPQueryIssuesBlocksIssue(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

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

	text := mcpToolSuccess(t, session, "query_issues", map[string]any{
		"blocks_issue": 2,
	})
	if !strings.Contains(text, "Blocker") {
		t.Fatalf("expected 'Blocker' in response, got: %s", text)
	}
}

func TestMCPQueryIssuesBlockedByIssue(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

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

	text := mcpToolSuccess(t, session, "query_issues", map[string]any{
		"blocked_by_issue": 1,
	})
	if !strings.Contains(text, "Blocked") {
		t.Fatalf("expected 'Blocked' in response, got: %s", text)
	}
}

func TestMCPQueryIssuesBothBlockFilters(t *testing.T) {
	_, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	errMsg := mcpToolErr(t, session, "query_issues", map[string]any{
		"blocks_issue":     1,
		"blocked_by_issue": 2,
	})
	if !strings.Contains(errMsg, "mutually exclusive") {
		t.Fatalf("expected mutual exclusivity error, got: %s", errMsg)
	}
}

func TestMCPQueryIssuesIsBlocked(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	_, err := s.CreateIssue("Blocker", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Blocked", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Neither", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateBlock(1, 2); err != nil {
		t.Fatal(err)
	}

	text := mcpToolSuccess(t, session, "query_issues", map[string]any{
		"is_blocked": true,
	})
	if !strings.Contains(text, "Blocked") {
		t.Fatalf("expected 'Blocked' in response, got: %s", text)
	}
	if strings.Contains(text, "Neither") {
		t.Fatal("should not contain 'Neither'")
	}
}

func TestMCPGetReadyIssues(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	_, err := s.CreateIssue("Ready", "task", "", []string{"ready-for-agent"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Not ready", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	text := mcpToolSuccess(t, session, "get_ready_issues", map[string]any{})
	if !strings.Contains(text, "Ready") {
		t.Fatalf("expected 'Ready' in response, got: %s", text)
	}
	if strings.Contains(text, "Not ready") {
		t.Fatal("should not contain 'Not ready'")
	}
}

func TestMCPSetParent(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	_, err := s.CreateIssue("Child", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Parent", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	text := mcpToolSuccess(t, session, "set_parent", map[string]any{
		"issue_number":  1,
		"parent_number": 2,
	})
	if !strings.Contains(text, `"parent_issue_id":2`) && !strings.Contains(text, `"parent_issue_id": 2`) {
		t.Fatalf("expected parent_issue_id 2 in response, got: %s", text)
	}
}

func TestMCPSetParentCycle(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

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

	errMsg := mcpToolErr(t, session, "set_parent", map[string]any{
		"issue_number":  3,
		"parent_number": 1,
	})
	if !strings.Contains(errMsg, "cycle") {
		t.Fatalf("expected cycle error, got: %s", errMsg)
	}
}

func TestMCPSetParentNotFound(t *testing.T) {
	_, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	errMsg := mcpToolErr(t, session, "set_parent", map[string]any{
		"issue_number":  1,
		"parent_number": 999,
	})
	if !strings.Contains(errMsg, "not found") {
		t.Fatalf("expected 'not found' error, got: %s", errMsg)
	}
}

func TestMCPClearParent(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

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

	text := mcpToolSuccess(t, session, "clear_parent", map[string]any{
		"issue_number": 1,
	})
	if strings.Contains(text, `"parent_issue_id":2`) || strings.Contains(text, `"parent_issue_id": 2`) {
		t.Fatalf("expected parent to be cleared, got: %s", text)
	}
}

func TestMCPClearParentNotFound(t *testing.T) {
	_, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	errMsg := mcpToolErr(t, session, "clear_parent", map[string]any{
		"issue_number": 999,
	})
	if !strings.Contains(errMsg, "not found") {
		t.Fatalf("expected 'not found' error, got: %s", errMsg)
	}
}

func TestMCPAddBlocking(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	_, err := s.CreateIssue("Blocker", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Blocked", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	text := mcpToolSuccess(t, session, "add_blocking", map[string]any{
		"blocker_number": 1,
		"blocked_number": 2,
	})
	if !strings.Contains(text, "true") {
		t.Fatalf("expected success in response, got: %s", text)
	}
}

func TestMCPAddBlockingCycle(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

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

	errMsg := mcpToolErr(t, session, "add_blocking", map[string]any{
		"blocker_number": 3,
		"blocked_number": 1,
	})
	if !strings.Contains(errMsg, "cycle") {
		t.Fatalf("expected cycle error, got: %s", errMsg)
	}
}

func TestMCPRemoveBlocking(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

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

	text := mcpToolSuccess(t, session, "remove_blocking", map[string]any{
		"blocker_number": 1,
		"blocked_number": 2,
	})
	if !strings.Contains(text, "true") {
		t.Fatalf("expected success in response, got: %s", text)
	}
}

func TestMCPRemoveBlockingNotFound(t *testing.T) {
	s, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	_, err := s.CreateIssue("Blocker", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateIssue("Blocked", "task", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	errMsg := mcpToolErr(t, session, "remove_blocking", map[string]any{
		"blocker_number": 1,
		"blocked_number": 2,
	})
	if !strings.Contains(errMsg, "not found") {
		t.Fatalf("expected 'block edge not found' error, got: %s", errMsg)
	}
}

func TestMCPUnknownTool(t *testing.T) {
	_, session, cleanup := mcpTestSetup(t)
	defer cleanup()

	_, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "nonexistent",
		Arguments: map[string]any{},
	})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}


