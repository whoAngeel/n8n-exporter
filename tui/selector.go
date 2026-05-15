// Package tui provides the interactive workflow selector built with Bubble Tea v1.
package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/whoAngeel/n8n-workflow-exported/n8nclient"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	styleTitle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styleModeIncl  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	styleModeExcl  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	styleHelp      = lipgloss.NewStyle().Faint(true)
	styleCursor    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styleChecked   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleUnchecked = lipgloss.NewStyle().Faint(true)
	styleCount     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styleTodayItem = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // yellow
	styleSepLine   = lipgloss.NewStyle().Faint(true)
)

// ── Types ─────────────────────────────────────────────────────────────────────

// SelectionMode controls which workflows are exported.
type SelectionMode int

const (
	ModeInclusion SelectionMode = iota // export only marked workflows
	ModeExclusion                      // export all except marked workflows
)

// workflowItem wraps a workflow for use in the bubbles/list component.
// idx is the stable position in the original workflows slice.
// isToday indicates the workflow was updated today.
type workflowItem struct {
	wf      n8nclient.Workflow
	idx     int
	isToday bool
}

func (i workflowItem) FilterValue() string { return i.wf.Name }

// separatorItem is a non-selectable divider rendered as a horizontal line.
type separatorItem struct{}

func (s separatorItem) FilterValue() string { return "" }

// ── Delegates ─────────────────────────────────────────────────────────────────

// itemDelegate renders workflow rows with cursor, checkbox, and today badge.
type itemDelegate struct {
	marked map[int]bool
}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	switch v := item.(type) {
	case separatorItem:
		_ = v
		fmt.Fprintf(w, "  %s", styleSepLine.Render(strings.Repeat("─", 52)))
		return

	case workflowItem:
		isCursor := index == m.Index()
		isMarked := d.marked[v.idx]

		cursor := " "
		if isCursor {
			cursor = styleCursor.Render("▶")
		}

		badge := "  " // two spaces to align with "★ "
		if v.isToday {
			badge = styleTodayItem.Render("★ ")
		}

		check := styleUnchecked.Render("[ ]")
		name := v.wf.Name
		if isMarked {
			check = styleChecked.Render("[✓]")
			name = styleChecked.Render(name)
		} else if v.isToday {
			name = styleTodayItem.Render(name)
		}

		fmt.Fprintf(w, "  %s %s %s%s", cursor, check, badge, name)
	}
}

// ── Model ─────────────────────────────────────────────────────────────────────

// SelectorModel is the Bubble Tea model for the workflow selection TUI.
type SelectorModel struct {
	list      list.Model
	marked    map[int]bool
	mode      SelectionMode
	Cancelled bool
	workflows []n8nclient.Workflow // original slice, stable indices
}

// isUpdatedToday returns true if the ISO 8601 timestamp corresponds to today
// in the local timezone.
func isUpdatedToday(updatedAt string) bool {
	if updatedAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.000Z", updatedAt)
		if err != nil {
			return false
		}
	}
	now := time.Now()
	y1, m1, d1 := t.Local().Date()
	y2, m2, d2 := now.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// buildItems returns the list items ordered as:
//  1. Workflows modified today (with isToday=true), sorted as received
//  2. A separator line (only if there is at least one today item AND at least one other)
//  3. Remaining workflows
func buildItems(workflows []n8nclient.Workflow) []list.Item {
	var todayItems []list.Item
	var restItems []list.Item

	for i, wf := range workflows {
		today := isUpdatedToday(wf.UpdatedAt)
		item := workflowItem{wf: wf, idx: i, isToday: today}
		if today {
			todayItems = append(todayItems, item)
		} else {
			restItems = append(restItems, item)
		}
	}

	if len(todayItems) == 0 {
		return restItems
	}
	if len(restItems) == 0 {
		return todayItems
	}

	items := make([]list.Item, 0, len(todayItems)+1+len(restItems))
	items = append(items, todayItems...)
	items = append(items, separatorItem{})
	items = append(items, restItems...)
	return items
}

// NewSelectorModel returns a SelectorModel ready to run.
func NewSelectorModel(workflows []n8nclient.Workflow) SelectorModel {
	marked := make(map[int]bool)
	delegate := itemDelegate{marked: marked}

	items := buildItems(workflows)

	l := list.New(items, delegate, 80, 20)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	return SelectorModel{
		list:      l,
		marked:    marked,
		mode:      ModeInclusion,
		workflows: workflows,
	}
}

func (m SelectorModel) Init() tea.Cmd { return nil }

func (m SelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width-4, msg.Height-8)
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.Cancelled = true
			return m, tea.Quit

		case " ":
			// Only toggle if the selected item is a workflow (not the separator).
			if item, ok := m.list.SelectedItem().(workflowItem); ok {
				m.marked[item.idx] = !m.marked[item.idx]
			}
			return m, nil

		case "tab":
			if m.mode == ModeInclusion {
				m.mode = ModeExclusion
			} else {
				m.mode = ModeInclusion
			}
			return m, nil

		case "enter":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m SelectorModel) View() string {
	var b strings.Builder

	modeLabel := styleModeIncl.Render("[INCLUSIÓN]")
	if m.mode == ModeExclusion {
		modeLabel = styleModeExcl.Render("[EXCLUSIÓN]")
	}

	b.WriteString(fmt.Sprintf("\n  %s  %s\n",
		styleTitle.Render("n8n Workflow Exporter"),
		modeLabel,
	))
	b.WriteString("  " + strings.Repeat("─", 55) + "\n")
	b.WriteString(styleHelp.Render("  ↑/↓ navegar   SPACE marcar   TAB modo   / filtrar   ENTER exportar   q salir") + "\n\n")

	b.WriteString(m.list.View())

	selected := len(m.GetSelectedWorkflows())
	b.WriteString(fmt.Sprintf("\n\n  %s %s / %d\n\n",
		styleHelp.Render("Workflows a exportar:"),
		styleCount.Render(fmt.Sprintf("%d", selected)),
		len(m.workflows),
	))

	return b.String()
}

// GetSelectedWorkflows returns the workflows to export based on mode and marked set.
// Never returns nil.
func (m SelectorModel) GetSelectedWorkflows() []n8nclient.Workflow {
	result := make([]n8nclient.Workflow, 0)
	for i, wf := range m.workflows {
		isMarked := m.marked[i]
		if m.mode == ModeInclusion && isMarked {
			result = append(result, wf)
		} else if m.mode == ModeExclusion && !isMarked {
			result = append(result, wf)
		}
	}
	return result
}
