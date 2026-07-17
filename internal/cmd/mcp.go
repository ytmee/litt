package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/ytmee/litt/internal/store"
)

type mcpIssueResponse struct {
	Number        int           `json:"number"`
	Ref           string        `json:"ref"`
	Title         string        `json:"title"`
	Body          string        `json:"body"`
	State         string        `json:"state"`
	Kind          string        `json:"kind"`
	ParentIssueID *int          `json:"parent_issue_id"`
	Labels        []store.Label `json:"labels"`
	CreatedAt     string        `json:"created_at"`
	UpdatedAt     string        `json:"updated_at"`
	ClosedAt      *string       `json:"closed_at"`
}

func newMCPIssue(issue *store.Issue) mcpIssueResponse {
	return mcpIssueResponse{
		Number:        issue.ID,
		Ref:           fmt.Sprintf("#%d", issue.ID),
		Title:         issue.Title,
		Body:          issue.Body,
		State:         issue.State,
		Kind:          issue.Kind,
		ParentIssueID: issue.ParentIssueID,
		Labels:        issue.Labels,
		CreatedAt:     issue.CreatedAt,
		UpdatedAt:     issue.UpdatedAt,
		ClosedAt:      issue.ClosedAt,
	}
}

func 	newMCPCmd() *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start an MCP stdio server",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := dbPath
			if path == "" {
				path = filepath.Join(".litt", "litt.db")
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return fmt.Errorf("database not found at %s; run 'litt init' first or use --db", path)
			}
			s, err := store.Open(path)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer s.Close()

			server := newMCPServer(s)
			return server.Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to litt database (auto-detect by default)")
	return cmd
}

func newMCPServer(s *store.Store) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "litt", Version: "0.1"}, nil)

	type createIssueInput struct {
		Kind   string   `json:"kind"`
		Title  string   `json:"title"`
		Body   string   `json:"body,omitempty"`
		Labels []string `json:"labels,omitempty"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_issue",
		Description: "Create a new issue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input createIssueInput) (*mcp.CallToolResult, any, error) {
		if input.Title == "" {
			return nil, nil, fmt.Errorf("title is required")
		}
		if input.Kind == "" {
			return nil, nil, fmt.Errorf("kind is required")
		}
		if input.Labels == nil {
			input.Labels = []string{}
		}
		issue, err := s.CreateIssue(input.Title, input.Kind, input.Body, input.Labels)
		if err != nil {
			return nil, nil, err
		}
		return nil, newMCPIssue(issue), nil
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
		Description: "Update an existing issue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input updateIssueInput) (*mcp.CallToolResult, any, error) {
		opts := store.UpdateIssueOptions{
			Title:        input.Title,
			Body:         input.Body,
			State:        input.State,
			Kind:         input.Kind,
			AddLabels:    input.AddLabels,
			RemoveLabels: input.RemoveLabels,
		}
		if err := s.UpdateIssue(input.Number, opts); err != nil {
			return nil, nil, err
		}
		issue, err := s.GetIssue(input.Number)
		if err != nil {
			return nil, nil, err
		}
		return nil, newMCPIssue(issue), nil
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
		Description: "Query issues with optional filters",
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

		if input.IsBlocked != nil && *input.IsBlocked {
			var filtered []store.Issue
			for _, issue := range issues {
				blockers, listErr := s.ListBlockedBy(issue.ID)
				if listErr != nil {
					return nil, nil, listErr
				}
				if len(blockers) > 0 {
					filtered = append(filtered, issue)
				}
			}
			issues = filtered
		}

		result := make([]mcpIssueResponse, len(issues))
		for i := range issues {
			result[i] = newMCPIssue(&issues[i])
		}
		return nil, result, nil
	})

	type getIssueInput struct {
		Number int `json:"number"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_issue",
		Description: "Get a single issue by number",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getIssueInput) (*mcp.CallToolResult, any, error) {
		issue, err := s.GetIssue(input.Number)
		if err != nil {
			return nil, nil, err
		}
		return nil, newMCPIssue(issue), nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ready_issues",
		Description: "Get issues ready for an agent (open, labeled ready-for-agent, not blocked by open issues)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		issues, err := s.ListReadyIssues()
		if err != nil {
			return nil, nil, err
		}
		result := make([]mcpIssueResponse, len(issues))
		for i := range issues {
			result[i] = newMCPIssue(&issues[i])
		}
		return nil, result, nil
	})

	type setParentInput struct {
		IssueNumber  int `json:"issue_number"`
		ParentNumber int `json:"parent_number"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_parent",
		Description: "Set the parent of an issue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input setParentInput) (*mcp.CallToolResult, any, error) {
		if err := s.SetParent(input.IssueNumber, input.ParentNumber); err != nil {
			return nil, nil, err
		}
		issue, err := s.GetIssue(input.IssueNumber)
		if err != nil {
			return nil, nil, err
		}
		return nil, newMCPIssue(issue), nil
	})

	type clearParentInput struct {
		IssueNumber int `json:"issue_number"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "clear_parent",
		Description: "Clear the parent of an issue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input clearParentInput) (*mcp.CallToolResult, any, error) {
		if err := s.ClearParent(input.IssueNumber); err != nil {
			return nil, nil, err
		}
		issue, err := s.GetIssue(input.IssueNumber)
		if err != nil {
			return nil, nil, err
		}
		return nil, newMCPIssue(issue), nil
	})

	type addBlockingInput struct {
		BlockerNumber int `json:"blocker_number"`
		BlockedNumber int `json:"blocked_number"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_blocking",
		Description: "Create a blocking relationship between issues",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input addBlockingInput) (*mcp.CallToolResult, any, error) {
		if err := s.CreateBlock(input.BlockerNumber, input.BlockedNumber); err != nil {
			return nil, nil, err
		}
		return nil, map[string]interface{}{
			"success":        true,
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
		Description: "Remove a blocking relationship between issues",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input removeBlockingInput) (*mcp.CallToolResult, any, error) {
		if err := s.RemoveBlock(input.BlockerNumber, input.BlockedNumber); err != nil {
			return nil, nil, err
		}
		return nil, map[string]interface{}{"success": true}, nil
	})

	return server
}
