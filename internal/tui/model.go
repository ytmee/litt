package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/ytmee/litt/internal/store"
)

const (
	splitThreshold = 100
	leftPercent    = 40
	rightPercent   = 60
)

type issueLister interface {
	ListIssues(state, kind, label string, parentID *int) ([]store.Issue, error)
	GetIssue(id int) (*store.Issue, error)
}

type model struct {
	store  issueLister
	issues []store.Issue
	cursor int
	detail *store.Issue
	width  int
	height int
}

var (
	styleSelected = lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("15"))
	styleNormal   = lipgloss.NewStyle()
	styleDetail   = lipgloss.NewStyle().Padding(0, 1)
	styleStatus   = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("15")).Padding(0, 1)
)

func newModel(s issueLister) *model {
	issues, err := s.ListIssues("", "", "", nil)
	m := &model{
		store:  s,
		cursor: 0,
		width:  80,
		height: 24,
	}
	if err == nil {
		m.issues = issues
	}
	return m
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) View() tea.View {
	left := m.renderList()
	right := m.renderDetail()
	status := m.renderStatus()

	var panels string
	if m.width < splitThreshold {
		panels = left
	} else {
		panels = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	for lipgloss.Height(panels) < m.height-1 {
		panels += "\n"
	}

	v := tea.NewView(panels + status)
	v.AltScreen = true
	return v
}

func (m *model) renderList() string {
	listHeight := m.height - 1
	if listHeight < 1 {
		listHeight = 1
	}

	var b strings.Builder
	start := 0
	if m.cursor >= listHeight {
		start = m.cursor - listHeight + 1
	}
	end := start + listHeight
	if end > len(m.issues) {
		end = len(m.issues)
	}

	for i := start; i < end; i++ {
		issue := m.issues[i]
		line := fmt.Sprintf(" #%d %s [%s]", issue.ID, issue.Title, issue.Kind)
		if i == m.cursor {
			b.WriteString(styleSelected.Render(line) + "\n")
		} else {
			b.WriteString(styleNormal.Render(line) + "\n")
		}
	}

	leftWidth := m.width
	if m.width >= splitThreshold {
		leftWidth = m.width * leftPercent / 100
	}
	return lipgloss.NewStyle().Width(leftWidth).MaxHeight(listHeight).Render(b.String())
}

func (m *model) renderDetail() string {
	if m.detail == nil {
		rightWidth := m.width*rightPercent/100 - 1
		if rightWidth < 1 {
			rightWidth = 1
		}
		return lipgloss.NewStyle().Width(rightWidth).Render("")
	}
	i := m.detail
	labels := make([]string, len(i.Labels))
	for idx, l := range i.Labels {
		labels[idx] = l.Name
	}
	labelStr := ""
	if len(labels) > 0 {
		labelStr = "\nLabels: " + strings.Join(labels, ", ")
	}

	body := fmt.Sprintf(
		" #%d: %s\n State: %s\n Kind: %s%s\n\n %s",
		i.ID, i.Title, i.State, i.Kind, labelStr, i.Body,
	)
	rightWidth := m.width*rightPercent/100 - 1
	if rightWidth < 1 {
		rightWidth = 1
	}
	return styleDetail.Width(rightWidth).Render(body)
}

func (m *model) renderStatus() string {
	left := fmt.Sprintf(" %d issues — view:list ", len(m.issues))
	right := " j/k navigate  Enter detail  q quit "
	padding := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 1 {
		padding = 1
	}
	return styleStatus.Render(left + strings.Repeat(" ", padding) + right)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.issues)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if len(m.issues) > 0 {
				detail, err := m.store.GetIssue(m.issues[m.cursor].ID)
				if err == nil {
					m.detail = detail
				}
			}
		}
	}
	return m, nil
}
