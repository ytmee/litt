package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ytmee/litt/internal/store"
)

func newIssueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "Manage issues",
	}
	cmd.AddCommand(newIssueCreateCmd())
	cmd.AddCommand(newIssueListCmd())
	cmd.AddCommand(newIssueShowCmd())
	cmd.AddCommand(newIssueUpdateCmd())
	cmd.AddCommand(newIssueCloseCmd())
	cmd.AddCommand(newIssueReopenCmd())
	cmd.AddCommand(newIssueParentCmd())
	cmd.AddCommand(newIssueChildrenCmd())
	cmd.AddCommand(newIssueBlockCmd())
	cmd.AddCommand(newIssueUnblockCmd())
	cmd.AddCommand(newIssueBlockedByCmd())
	cmd.AddCommand(newIssueBlockingCmd())
	cmd.AddCommand(newIssueReadyCmd())
	return cmd
}

func parseIssueNumber(s string) (int, error) {
	s = strings.TrimPrefix(s, "#")
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid issue number: %s", s)
	}
	return id, nil
}

func newIssueCreateCmd() *cobra.Command {
	var kind string
	var body string
	var labels []string

	cmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			issue, err := s.CreateIssue(args[0], kind, body, labels)
			if err != nil {
				return fmt.Errorf("create issue: %w", err)
			}
			cmd.Printf("Created issue #%d: %s\n", issue.ID, issue.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "task", "Issue kind (spec, task, or bug)")
	cmd.Flags().StringVar(&body, "body", "", "Issue body")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "Labels to attach (can be specified multiple times)")
	return cmd
}

func newIssueListCmd() *cobra.Command {
	var state string
	var kind string
	var label string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			filterState := state
			if filterState == "" {
				filterState = "open"
			}
			issues, err := s.ListIssues(filterState, kind, label)
			if err != nil {
				return fmt.Errorf("list issues: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(issues); err != nil {
					return fmt.Errorf("encode issues: %w", err)
				}
				return nil
			}

			if len(issues) == 0 {
				cmd.Println("No issues found.")
				return nil
			}

			cmd.Println("#    State   Kind     Title")
			for _, issue := range issues {
				labelNames := make([]string, len(issue.Labels))
				for j, l := range issue.Labels {
					labelNames[j] = l.Name
				}
				labelsStr := ""
				if len(labelNames) > 0 {
					labelsStr = " [" + strings.Join(labelNames, ", ") + "]"
				}
				cmd.Printf("#%-3d %-7s %-8s %s%s\n", issue.ID, issue.State, issue.Kind, issue.Title, labelsStr)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "Filter by state (open or closed)")
	cmd.Flags().StringVar(&kind, "kind", "", "Filter by kind (spec, task, or bug)")
	cmd.Flags().StringVar(&label, "label", "", "Filter by label name")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func newIssueShowCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "show <number>",
		Short: "Show issue details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseIssueNumber(args[0])
			if err != nil {
				return err
			}

			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			issue, err := s.GetIssue(id)
			if err != nil {
				return fmt.Errorf("show issue: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(issue); err != nil {
					return fmt.Errorf("encode issue: %w", err)
				}
				return nil
			}

			labelNames := make([]string, len(issue.Labels))
			for j, l := range issue.Labels {
				labelNames[j] = l.Name
			}

			parentStr := ""
			if issue.ParentIssueID != nil {
				parentStr = fmt.Sprintf("#%d", *issue.ParentIssueID)
			}

			closedAtStr := ""
			if issue.ClosedAt != nil {
				closedAtStr = *issue.ClosedAt
			}

			cmd.Printf("Title:   %s\n", issue.Title)
			cmd.Printf("Body:    %s\n", issue.Body)
			cmd.Printf("Kind:    %s\n", issue.Kind)
			cmd.Printf("State:   %s\n", issue.State)
			cmd.Printf("ID:      #%d\n", issue.ID)
			cmd.Printf("Parent:  %s\n", parentStr)
			cmd.Printf("Labels:  %s\n", strings.Join(labelNames, ", "))
			cmd.Printf("Created: %s\n", issue.CreatedAt)
			cmd.Printf("Updated: %s\n", issue.UpdatedAt)
			cmd.Printf("Closed:  %s\n", closedAtStr)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func newIssueUpdateCmd() *cobra.Command {
	var title string
	var body string
	var state string
	var kind string
	var addLabels []string
	var removeLabels []string

	cmd := &cobra.Command{
		Use:   "update <number>",
		Short: "Update an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseIssueNumber(args[0])
			if err != nil {
				return err
			}

			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			opts := store.UpdateIssueOptions{
				AddLabels:    addLabels,
				RemoveLabels: removeLabels,
			}
			if cmd.Flags().Changed("title") {
				opts.Title = &title
			}
			if cmd.Flags().Changed("body") {
				opts.Body = &body
			}
			if cmd.Flags().Changed("state") {
				opts.State = &state
			}
			if cmd.Flags().Changed("kind") {
				opts.Kind = &kind
			}

			if err := s.UpdateIssue(id, opts); err != nil {
				return fmt.Errorf("update issue: %w", err)
			}
			cmd.Printf("Updated issue #%d\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVar(&body, "body", "", "New body")
	cmd.Flags().StringVar(&state, "state", "", "New state (open or closed)")
	cmd.Flags().StringVar(&kind, "kind", "", "New kind (spec, task, or bug)")
	cmd.Flags().StringSliceVar(&addLabels, "add-label", nil, "Labels to add (can be specified multiple times)")
	cmd.Flags().StringSliceVar(&removeLabels, "remove-label", nil, "Labels to remove (can be specified multiple times)")
	return cmd
}

func newIssueCloseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "close <number>",
		Short: "Close an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseIssueNumber(args[0])
			if err != nil {
				return err
			}

			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			if err := s.CloseIssue(id); err != nil {
				return fmt.Errorf("close issue: %w", err)
			}
			cmd.Printf("Closed issue #%d\n", id)
			return nil
		},
	}
}

func newIssueReopenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reopen <number>",
		Short: "Reopen an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseIssueNumber(args[0])
			if err != nil {
				return err
			}

			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			if err := s.ReopenIssue(id); err != nil {
				return fmt.Errorf("reopen issue: %w", err)
			}
			cmd.Printf("Reopened issue #%d\n", id)
			return nil
		},
	}
}

func newIssueParentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parent",
		Short: "Manage issue parent relationships",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "set <issue> <parent>",
		Short: "Set the parent of an issue",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseIssueNumber(args[0])
			if err != nil {
				return err
			}
			parentID, err := parseIssueNumber(args[1])
			if err != nil {
				return err
			}

			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			if err := s.SetParent(id, parentID); err != nil {
				return fmt.Errorf("set parent: %w", err)
			}
			cmd.Printf("Set parent of #%d to #%d\n", id, parentID)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "clear <issue>",
		Short: "Clear the parent of an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseIssueNumber(args[0])
			if err != nil {
				return err
			}

			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			if err := s.ClearParent(id); err != nil {
				return fmt.Errorf("clear parent: %w", err)
			}
			cmd.Printf("Cleared parent of #%d\n", id)
			return nil
		},
	})
	return cmd
}

func newIssueChildrenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "children <parent>",
		Short: "List children of a parent issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parentID, err := parseIssueNumber(args[0])
			if err != nil {
				return err
			}

			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			children, err := s.ListChildren(parentID)
			if err != nil {
				return fmt.Errorf("list children: %w", err)
			}

			if len(children) == 0 {
				cmd.Printf("No children found for #%d.\n", parentID)
				return nil
			}

			cmd.Println("#    State   Kind     Title")
			for _, issue := range children {
				labelNames := make([]string, len(issue.Labels))
				for j, l := range issue.Labels {
					labelNames[j] = l.Name
				}
				labelsStr := ""
				if len(labelNames) > 0 {
					labelsStr = " [" + strings.Join(labelNames, ", ") + "]"
				}
				cmd.Printf("#%-3d %-7s %-8s %s%s\n", issue.ID, issue.State, issue.Kind, issue.Title, labelsStr)
			}
			return nil
		},
	}
}

func newIssueBlockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "block <blocker> <blocked>",
		Short: "Create a blocking relationship",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			blockerID, err := parseIssueNumber(args[0])
			if err != nil {
				return err
			}
			blockedID, err := parseIssueNumber(args[1])
			if err != nil {
				return err
			}

			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			if _, err := s.CreateBlock(blockerID, blockedID); err != nil {
				return fmt.Errorf("block: %w", err)
			}
			cmd.Printf("#%d now blocks #%d\n", blockerID, blockedID)
			return nil
		},
	}
}

func newIssueUnblockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unblock <blocker> <blocked>",
		Short: "Remove a blocking relationship",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			blockerID, err := parseIssueNumber(args[0])
			if err != nil {
				return err
			}
			blockedID, err := parseIssueNumber(args[1])
			if err != nil {
				return err
			}

			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			if err := s.RemoveBlock(blockerID, blockedID); err != nil {
				return fmt.Errorf("unblock: %w", err)
			}
			cmd.Printf("Removed block: #%d no longer blocks #%d\n", blockerID, blockedID)
			return nil
		},
	}
}

func newIssueBlockedByCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "blocked-by <issue>",
		Short: "List issues blocking the given issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseIssueNumber(args[0])
			if err != nil {
				return err
			}

			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			blockers, err := s.ListBlockedBy(id)
			if err != nil {
				return fmt.Errorf("list blocked-by: %w", err)
			}

			if len(blockers) == 0 {
				cmd.Printf("#%d is not blocked by any issue.\n", id)
				return nil
			}

			cmd.Printf("#%d is blocked by:\n", id)
			for _, b := range blockers {
				cmd.Printf("  #%d %s (%s)\n", b.ID, b.Title, b.State)
			}
			return nil
		},
	}
}

func newIssueBlockingCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "blocking <issue>",
		Short: "List issues blocked by the given issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseIssueNumber(args[0])
			if err != nil {
				return err
			}

			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			blocked, err := s.ListBlocking(id)
			if err != nil {
				return fmt.Errorf("list blocking: %w", err)
			}

			if len(blocked) == 0 {
				cmd.Printf("#%d is not blocking any issue.\n", id)
				return nil
			}

			cmd.Printf("#%d blocks:\n", id)
			for _, b := range blocked {
				cmd.Printf("  #%d %s (%s)\n", b.ID, b.Title, b.State)
			}
			return nil
		},
	}
}

func newIssueReadyCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "ready",
		Short: "List issues ready for an agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()

			issues, err := s.ListReadyIssues()
			if err != nil {
				return fmt.Errorf("list ready issues: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(issues); err != nil {
					return fmt.Errorf("encode ready issues: %w", err)
				}
				return nil
			}

			if len(issues) == 0 {
				cmd.Println("No ready issues found.")
				return nil
			}

			cmd.Println("#    State   Kind     Title")
			for _, issue := range issues {
				labelNames := make([]string, len(issue.Labels))
				for j, l := range issue.Labels {
					labelNames[j] = l.Name
				}
				labelsStr := ""
				if len(labelNames) > 0 {
					labelsStr = " [" + strings.Join(labelNames, ", ") + "]"
				}
				cmd.Printf("#%-3d %-7s %-8s %s%s\n", issue.ID, issue.State, issue.Kind, issue.Title, labelsStr)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}
