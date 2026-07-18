package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/ytmee/litt/internal/store"
)

type mcpServer struct {
	store  *store.Store
	dbPath string
	mu     sync.Mutex
}

func (ms *mcpServer) getStore() *store.Store {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.store != nil {
		return ms.store
	}
	if ms.dbPath == "" {
		return nil
	}
	if _, err := os.Stat(ms.dbPath); os.IsNotExist(err) {
		return nil
	}
	s, err := store.Open(ms.dbPath)
	if err != nil {
		return nil
	}
	ms.store = s
	return ms.store
}

func (ms *mcpServer) getOrInitStore() (*store.Store, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.store != nil {
		return ms.store, nil
	}
	if ms.dbPath == "" {
		return nil, fmt.Errorf("no database path configured")
	}
	needInit := false
	if _, err := os.Stat(ms.dbPath); os.IsNotExist(err) {
		needInit = true
		if err := os.MkdirAll(filepath.Dir(ms.dbPath), 0755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}
	s, err := store.Open(ms.dbPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if needInit {
		if err := s.Migrate(); err != nil {
			s.Close()
			return nil, fmt.Errorf("migrate: %w", err)
		}
		if err := s.SeedLabels(); err != nil {
			s.Close()
			return nil, fmt.Errorf("seed labels: %w", err)
		}
	}
	ms.store = s
	return ms.store, nil
}

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start an MCP stdio server",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath, err := resolveDBPath(cmd)
			if err != nil {
				return err
			}
			server := newMCPServerLazy(dbPath)
			return server.Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
	return cmd
}

func newMCPServer(s *store.Store) *mcp.Server {
	return buildMCPServer(&mcpServer{store: s})
}

func newMCPServerLazy(dbPath string) *mcp.Server {
	return buildMCPServer(&mcpServer{dbPath: dbPath})
}

func buildMCPServer(ms *mcpServer) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "litt", Version: "0.1"}, nil)

	type createIssueInput struct {
		Kind   *string  `json:"kind,omitempty"`
		Title  string   `json:"title"`
		Body   string   `json:"body,omitempty"`
		Labels []string `json:"labels,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_issue",
		Description: "Create a new litt issue. Kind can be 'spec', 'task', or 'bug'.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input createIssueInput) (*mcp.CallToolResult, any, error) {
		if input.Title == "" {
			return nil, nil, fmt.Errorf("title is required")
		}
		kind := "task"
		if input.Kind != nil && *input.Kind != "" {
			kind = *input.Kind
		}
		if input.Labels == nil {
			input.Labels = []string{}
		}
		s, err := ms.getOrInitStore()
		if err != nil {
			return nil, nil, err
		}
		issue, err := s.CreateIssue(input.Title, kind, input.Body, input.Labels)
		if err != nil {
			return nil, nil, err
		}
		return nil, issue, nil
	})

	type updateIssueInput struct {
		Number       int      `json:"number"`
		Title        *string  `json:"title,omitempty"`
		Body         *string  `json:"body,omitempty"`
		State        *string  `json:"state,omitempty"`
		Kind         *string  `json:"kind,omitempty"`
		AddLabels    []string `json:"add_labels,omitempty"`
		RemoveLabels []string `json:"remove_labels,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_issue",
		Description: "Update an existing litt issue. Kind can be 'spec', 'task', or 'bug' if provided.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input updateIssueInput) (*mcp.CallToolResult, any, error) {
		s, err := ms.getOrInitStore()
		if err != nil {
			return nil, nil, err
		}
		opts := store.UpdateIssueOptions{
			Title:        input.Title,
			Body:         input.Body,
			State:        input.State,
			Kind:         input.Kind,
			AddLabels:    input.AddLabels,
			RemoveLabels: input.RemoveLabels,
		}
		hasFields := input.Title != nil || input.Body != nil || input.State != nil || input.Kind != nil ||
			len(input.AddLabels) > 0 || len(input.RemoveLabels) > 0
		if hasFields {
			if err := s.UpdateIssue(input.Number, opts); err != nil {
				return nil, nil, err
			}
		}
		issue, err := s.GetIssue(input.Number)
		if err != nil {
			return nil, nil, err
		}
		return nil, issue, nil
	})

	type queryIssuesInput struct {
		State          *string `json:"state,omitempty"`
		Kind           *string `json:"kind,omitempty"`
		Label          *string `json:"label,omitempty"`
		IsBlocked      *bool   `json:"is_blocked,omitempty"`
		BlocksIssue    *int    `json:"blocks_issue,omitempty"`
		BlockedByIssue *int    `json:"blocked_by_issue,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_issues",
		Description: "Query litt issues with optional filters",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input queryIssuesInput) (*mcp.CallToolResult, any, error) {
		state := ""
		kind := ""
		label := ""
		if input.State != nil {
			state = *input.State
		}
		if input.Kind != nil {
			kind = *input.Kind
		}
		if input.Label != nil {
			label = *input.Label
		}

		if input.BlocksIssue != nil && input.BlockedByIssue != nil {
			return nil, nil, fmt.Errorf("blocks_issue and blocked_by_issue are mutually exclusive")
		}

		s := ms.getStore()
		if s == nil {
			return nil, map[string]any{"issues": []store.Issue{}}, nil
		}

		var issues []store.Issue
		var err error

		if input.BlocksIssue != nil {
			blockers, listErr := s.ListBlockedBy(*input.BlocksIssue)
			if listErr != nil {
				return nil, nil, listErr
			}
			issues = blockers
		} else if input.BlockedByIssue != nil {
			blocked, listErr := s.ListBlocking(*input.BlockedByIssue)
			if listErr != nil {
				return nil, nil, listErr
			}
			issues = blocked
		} else {
			issues, err = s.ListIssues(state, kind, label)
			if err != nil {
				return nil, nil, err
			}
		}

		if input.BlocksIssue != nil || input.BlockedByIssue != nil {
			var filtered []store.Issue
			for _, issue := range issues {
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
			issues = filtered
		}

		if input.IsBlocked != nil {
			var filtered []store.Issue
			for _, issue := range issues {
				blockers, listErr := s.ListBlockedBy(issue.ID)
				if listErr != nil {
					return nil, nil, listErr
				}
				hasOpenBlocker := false
				for _, b := range blockers {
					if b.State == "open" {
						hasOpenBlocker = true
						break
					}
				}
				if *input.IsBlocked == hasOpenBlocker {
					filtered = append(filtered, issue)
				}
			}
			issues = filtered
		}

		return nil, map[string]any{"issues": issues}, nil
	})

	type getIssueInput struct {
		Number int `json:"number"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_issue",
		Description: "Get a single litt issue by number",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getIssueInput) (*mcp.CallToolResult, any, error) {
		s := ms.getStore()
		if s == nil {
			return nil, nil, fmt.Errorf("issue %d not found", input.Number)
		}
		issue, err := s.GetIssue(input.Number)
		if err != nil {
			return nil, nil, err
		}
		return nil, issue, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ready_issues",
		Description: "Get litt issues ready for an agent (open, labeled ready-for-agent, not blocked by open issues)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		s := ms.getStore()
		if s == nil {
			return nil, map[string]any{"issues": []store.Issue{}}, nil
		}
		issues, err := s.ListReadyIssues()
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"issues": issues}, nil
	})

	type setParentInput struct {
		IssueNumber  int `json:"issue_number"`
		ParentNumber int `json:"parent_number"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_parent",
		Description: "Set the parent of a litt issue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input setParentInput) (*mcp.CallToolResult, any, error) {
		s, err := ms.getOrInitStore()
		if err != nil {
			return nil, nil, err
		}
		if err := s.SetParent(input.IssueNumber, input.ParentNumber); err != nil {
			return nil, nil, err
		}
		issue, err := s.GetIssue(input.IssueNumber)
		if err != nil {
			return nil, nil, err
		}
		return nil, issue, nil
	})

	type clearParentInput struct {
		IssueNumber int `json:"issue_number"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "clear_parent",
		Description: "Clear the parent of a litt issue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input clearParentInput) (*mcp.CallToolResult, any, error) {
		s, err := ms.getOrInitStore()
		if err != nil {
			return nil, nil, err
		}
		if err := s.ClearParent(input.IssueNumber); err != nil {
			return nil, nil, err
		}
		issue, err := s.GetIssue(input.IssueNumber)
		if err != nil {
			return nil, nil, err
		}
		return nil, issue, nil
	})

	type addBlockingInput struct {
		BlockerNumber int `json:"blocker_number"`
		BlockedNumber int `json:"blocked_number"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_blocking",
		Description: "Create a blocking relationship between litt issues",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input addBlockingInput) (*mcp.CallToolResult, any, error) {
		s, err := ms.getOrInitStore()
		if err != nil {
			return nil, nil, err
		}
		created, err := s.CreateBlock(input.BlockerNumber, input.BlockedNumber)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]interface{}{
			"success":        true,
			"created":        created,
			"blocker_number": input.BlockerNumber,
			"blocked_number": input.BlockedNumber,
		}, nil
	})

	type removeBlockingInput struct {
		BlockerNumber int `json:"blocker_number"`
		BlockedNumber int `json:"blocked_number"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "remove_blocking",
		Description: "Remove a blocking relationship between litt issues",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input removeBlockingInput) (*mcp.CallToolResult, any, error) {
		s, err := ms.getOrInitStore()
		if err != nil {
			return nil, nil, err
		}
		if err := s.RemoveBlock(input.BlockerNumber, input.BlockedNumber); err != nil {
			return nil, nil, err
		}
		return nil, map[string]interface{}{"success": true}, nil
	})

	return server
}
