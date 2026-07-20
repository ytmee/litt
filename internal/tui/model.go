package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

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
	ListLabels() ([]store.Label, error)
}

type model struct {
	store     issueLister
	allIssues []store.Issue
	issues    []store.Issue
	cursor    int
	detail    *store.Issue
	width     int
	height    int

	searchMode  bool
	searchQuery string
	filterState string
	filterKind  string
	filterLabel string
	labels      []store.Label
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
		m.allIssues = issues
		m.issues = issues
	}
	if labels, err := s.ListLabels(); err == nil {
		m.labels = labels
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
	if m.searchMode {
		return m.renderSearchStatus()
	}
	left := fmt.Sprintf(" %d issues — view:list ", len(m.issues))
	right := " j/k navigate  Enter detail  q quit "
	if m.filterState != "" || m.filterKind != "" || m.filterLabel != "" {
		s := m.filterState
		if s == "" {
			s = "all"
		}
		k := m.filterKind
		if k == "" {
			k = "all"
		}
		l := m.filterLabel
		if l == "" {
			l = "all"
		}
		right = fmt.Sprintf(" s:%s  k:%s  l:%s ", s, k, l)
	}
	padding := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 1 {
		padding = 1
	}
	return styleStatus.Render(left + strings.Repeat(" ", padding) + right)
}

func (m *model) renderSearchStatus() string {
	query := m.searchQuery
	stateStr := m.filterState
	if stateStr == "" {
		stateStr = "all"
	}
	kindStr := m.filterKind
	if kindStr == "" {
		kindStr = "all"
	}
	labelStr := m.filterLabel
	if labelStr == "" {
		labelStr = "all"
	}

	left := fmt.Sprintf(" /%s  (%d/%d)  s:%s  k:%s  l:%s ", query, len(m.issues), len(m.allIssues), stateStr, kindStr, labelStr)
	right := " Esc clear "
	padding := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 1 {
		padding = 1
	}
	return styleStatus.Render(left + strings.Repeat(" ", padding) + right)
}

func (m *model) applyFilters() {
	if !m.searchMode && m.filterState == "" && m.filterKind == "" && m.filterLabel == "" {
		m.issues = m.allIssues
		return
	}

	var filtered []store.Issue
	for _, issue := range m.allIssues {
		if m.searchQuery != "" && !strings.Contains(strings.ToLower(issue.Title), strings.ToLower(m.searchQuery)) {
			continue
		}
		if m.filterState != "" && issue.State != m.filterState {
			continue
		}
		if m.filterKind != "" && issue.Kind != m.filterKind {
			continue
		}
		if m.filterLabel != "" {
			hasLabel := false
			for _, l := range issue.Labels {
				if l.Name == m.filterLabel {
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
	m.issues = filtered

	if m.cursor >= len(m.issues) {
		if len(m.issues) == 0 {
			m.cursor = 0
		} else {
			m.cursor = len(m.issues) - 1
		}
	}
}

func (m *model) enterSearchMode() {
	m.searchMode = true
	m.searchQuery = ""
	m.applyFilters()
}

func (m *model) exitSearchMode() {
	m.searchMode = false
	m.searchQuery = ""
	m.filterState = ""
	m.filterKind = ""
	m.filterLabel = ""
	m.applyFilters()
}

func (m *model) cycleFilterState() {
	switch m.filterState {
	case "":
		m.filterState = "open"
	case "open":
		m.filterState = "closed"
	case "closed":
		m.filterState = ""
	}
	m.applyFilters()
}

func (m *model) cycleFilterKind() {
	switch m.filterKind {
	case "":
		m.filterKind = "spec"
	case "spec":
		m.filterKind = "task"
	case "task":
		m.filterKind = "bug"
	case "bug":
		m.filterKind = ""
	}
	m.applyFilters()
}

func (m *model) cycleFilterLabel() {
	if len(m.labels) == 0 {
		m.filterLabel = ""
		m.applyFilters()
		return
	}
	idx := -1
	for i, l := range m.labels {
		if l.Name == m.filterLabel {
			idx = i
			break
		}
	}
	if idx == -1 {
		m.filterLabel = m.labels[0].Name
	} else if idx == len(m.labels)-1 {
		m.filterLabel = ""
	} else {
		m.filterLabel = m.labels[idx+1].Name
	}
	m.applyFilters()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyPressMsg:
		k := msg.Key()
		if k.Mod.Contains(tea.ModCtrl) {
			switch k.Code {
			case 's':
				m.cycleFilterState()
			case 'k':
				m.cycleFilterKind()
			case 'l':
				m.cycleFilterLabel()
			}
			return m, nil
		}

		switch msg.String() {
		case "q":
			if m.searchMode {
				m.searchQuery += "q"
				m.applyFilters()
			} else {
				return m, tea.Quit
			}
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
		case "/":
			if !m.searchMode {
				m.enterSearchMode()
			}
		case "esc":
			if m.searchMode {
				m.exitSearchMode()
			}
		case "backspace":
			if m.searchMode && len(m.searchQuery) > 0 {
				_, size := utf8.DecodeLastRuneInString(m.searchQuery)
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-size]
				m.applyFilters()
			}
		case "space":
			if m.searchMode {
				m.searchQuery += " "
				m.applyFilters()
			}
		default:
			if m.searchMode {
				t := msg.String()
				if len(t) > 0 && t[0] > 32 {
					m.searchQuery += t
					m.applyFilters()
				}
			}
		}
	}
	return m, nil
}
