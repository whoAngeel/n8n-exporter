// Package tui provides the interactive workflow selector built with Bubble Tea v1.
package tui

import (
	"fmt"
	"io"
	"strings"

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
)

// ── Types ─────────────────────────────────────────────────────────────────────

// SelectionMode controls which workflows are exported.
type SelectionMode int

const (
	ModeInclusion SelectionMode = iota // export only marked workflows
	ModeExclusion                      // export all except marked workflows
)

// workflowItem wraps a workflow for use in the bubbles/list component.
// idx is the stable position in the original workflows slice, unaffected by filtering.
type workflowItem struct {
	wf  n8nclient.Workflow
	idx int
}

func (i workflowItem) FilterValue() string { return i.wf.Name }

// itemDelegate renders each row with cursor indicator and checkbox.
// marked is a direct map reference shared with SelectorModel, so mutations are
// reflected in the next Render call without needing to call SetDelegate again.
type itemDelegate struct {
	marked map[int]bool
}

func (d itemDelegate) Height() int                              { return 1 }
func (d itemDelegate) Spacing() int                             { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	wfItem, ok := item.(workflowItem)
	if !ok {
		return
	}

	isCursor := index == m.Index()
	isMarked := d.marked[wfItem.idx]

	cursor := " "
	if isCursor {
		cursor = styleCursor.Render("▶")
	}

	check := styleUnchecked.Render("[ ]")
	name := wfItem.wf.Name
	if isMarked {
		check = styleChecked.Render("[✓]")
		name = styleChecked.Render(name)
	}

	fmt.Fprintf(w, "  %s %s  %s", cursor, check, name)
}

// ── Model ─────────────────────────────────────────────────────────────────────

// SelectorModel is the Bubble Tea model for the workflow selection TUI.
type SelectorModel struct {
	list      list.Model
	marked    map[int]bool
	mode      SelectionMode
	Cancelled bool
	workflows []n8nclient.Workflow
}

// NewSelectorModel returns a SelectorModel ready to run.
func NewSelectorModel(workflows []n8nclient.Workflow) SelectorModel {
	marked := make(map[int]bool)

	items := make([]list.Item, len(workflows))
	for i, wf := range workflows {
		items[i] = workflowItem{wf: wf, idx: i}
	}

	// delegate shares the same map so Render always sees up-to-date marks.
	delegate := itemDelegate{marked: marked}

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
		// While filtering, delegate all keys to the list component.
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
