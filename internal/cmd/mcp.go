package cmd

import (
	"context"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/ytmee/litt/internal/query"
	"github.com/ytmee/litt/internal/store"
)

type mcpServer struct {
	s      *store.Store
	dbPath string
	mu     sync.Mutex
}

func (ms *mcpServer) storeForRead() *store.Store {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.s != nil {
		return ms.s
	}
	s, err := store.OpenIfExists(ms.dbPath)
	if err != nil {
		return nil
	}
	if s != nil {
		ms.s = s
	}
	return s
}

func (ms *mcpServer) storeForWrite() (*store.Store, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.s != nil {
		return ms.s, nil
	}
	if ms.dbPath == "" {
		return nil, fmt.Errorf("no database path configured")
	}
	s, err := store.Ensure(ms.dbPath)
	if err != nil {
		return nil, err
	}
	ms.s = s
	return ms.s, nil
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

func newMCPServer(st *store.Store) *mcp.Server {
	return buildMCPServer(&mcpServer{s: st})
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
		s, err := ms.storeForWrite()
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
		s, err := ms.storeForWrite()
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
		var blocksIssue, blockedByIssue int
		if input.BlocksIssue != nil {
			blocksIssue = *input.BlocksIssue
		}
		if input.BlockedByIssue != nil {
			blockedByIssue = *input.BlockedByIssue
		}

		s := ms.storeForRead()
		if s == nil {
			return nil, map[string]any{"issues": []store.Issue{}}, nil
		}

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

		issues, err := query.ListIssues(s, query.Params{
			State:          state,
			Kind:           kind,
			Label:          label,
			IsBlocked:      input.IsBlocked,
			BlocksIssue:    blocksIssue,
			BlockedByIssue: blockedByIssue,
		})
		if err != nil {
			return nil, nil, err
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
		s := ms.storeForRead()
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
		s := ms.storeForRead()
		if s == nil {
			return nil, map[string]any{"issues": []store.Issue{}}, nil
		}
		issues, err := query.ListReady(s)
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
		s, err := ms.storeForWrite()
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
		s, err := ms.storeForWrite()
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
		s, err := ms.storeForWrite()
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
		s, err := ms.storeForWrite()
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
