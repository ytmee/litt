package query

import (
	"fmt"

	"github.com/ytmee/litt/internal/store"
)

type issueReader interface {
	ListIssues(state, kind, label string) ([]store.Issue, error)
	ListBlockedBy(issueID int) ([]store.Issue, error)
	ListBlocking(issueID int) ([]store.Issue, error)
}

type Params struct {
	State          string
	Kind           string
	Label          string
	IsBlocked      *bool
	BlocksIssue    int
	BlockedByIssue int
}

func ListIssues(reader issueReader, params Params) ([]store.Issue, error) {
	switch {
	case params.BlocksIssue != 0 && params.BlockedByIssue != 0:
		return nil, fmt.Errorf("blocks_issue and blocked_by_issue are mutually exclusive")
	case params.BlocksIssue != 0:
		issues, err := reader.ListBlockedBy(params.BlocksIssue)
		if err != nil {
			return nil, err
		}
		return filterPost(issues, params), nil
	case params.BlockedByIssue != 0:
		issues, err := reader.ListBlocking(params.BlockedByIssue)
		if err != nil {
			return nil, err
		}
		return filterPost(issues, params), nil
	}

	issues, err := reader.ListIssues(params.State, params.Kind, params.Label)
	if err != nil {
		return nil, err
	}
	if params.IsBlocked != nil {
		return filterBlocked(reader, issues, *params.IsBlocked)
	}
	return issues, nil
}

func ListReady(reader issueReader) ([]store.Issue, error) {
	issues, err := reader.ListIssues("open", "", "ready-for-agent")
	if err != nil {
		return nil, err
	}
	return filterBlocked(reader, issues, false)
}

func filterPost(issues []store.Issue, params Params) []store.Issue {
	if params.State == "" && params.Kind == "" && params.Label == "" {
		return issues
	}
	var filtered []store.Issue
	for _, issue := range issues {
		if params.State != "" && issue.State != params.State {
			continue
		}
		if params.Kind != "" && issue.Kind != params.Kind {
			continue
		}
		if params.Label != "" {
			if !hasLabel(issue, params.Label) {
				continue
			}
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

func filterBlocked(reader issueReader, issues []store.Issue, wantBlocked bool) ([]store.Issue, error) {
	var filtered []store.Issue
	for _, issue := range issues {
		blockers, err := reader.ListBlockedBy(issue.ID)
		if err != nil {
			return nil, err
		}
		hasOpenBlocker := false
		for _, b := range blockers {
			if b.State == "open" {
				hasOpenBlocker = true
				break
			}
		}
		if wantBlocked == hasOpenBlocker {
			filtered = append(filtered, issue)
		}
	}
	return filtered, nil
}

func hasLabel(issue store.Issue, name string) bool {
	for _, l := range issue.Labels {
		if l.Name == name {
			return true
		}
	}
	return false
}
