// Package tui provides the interactive workflow selector built with Bubble Tea v1.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/whoAngeel/n8n-workflow-exported/n8nclient"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	styleTitle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))  // bright cyan
	styleModeIncl  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))  // bright green
	styleModeExcl  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")) // orange
	styleHelp      = lipgloss.NewStyle().Faint(true)
	styleCursor    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")) // bright cyan
	styleChecked   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))            // green checkmark
	styleUnchecked = lipgloss.NewStyle().Faint(true)
	styleFooter    = lipgloss.NewStyle().Faint(true)
	styleCount     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
)

// ── Types ─────────────────────────────────────────────────────────────────────

// SelectionMode controls which workflows are exported.
type SelectionMode int

const (
	ModeInclusion SelectionMode = iota // export only marked workflows
	ModeExclusion                      // export all except marked workflows
)

// SelectorModel is the Bubble Tea model for the workflow selection TUI.
type SelectorModel struct {
	workflows []n8nclient.Workflow
	cursor    int
	marked    map[int]bool
	mode      SelectionMode
	Cancelled bool
}

// NewSelectorModel returns a SelectorModel with safe initial state.
func NewSelectorModel(workflows []n8nclient.Workflow) SelectorModel {
	return SelectorModel{
		workflows: workflows,
		cursor:    0,
		marked:    make(map[int]bool),
		mode:      ModeInclusion,
		Cancelled: false,
	}
}

// Init satisfies tea.Model. No initial command needed.
func (m SelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input and returns the updated model.
func (m SelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Cancelled = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.workflows)-1 {
				m.cursor++
			}

		case " ": // toggle mark
			m.marked[m.cursor] = !m.marked[m.cursor]

		case "tab": // toggle inclusion/exclusion mode
			if m.mode == ModeInclusion {
				m.mode = ModeExclusion
			} else {
				m.mode = ModeInclusion
			}

		case "enter": // confirm selection
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the full TUI screen with lipgloss styles.
func (m SelectorModel) View() string {
	var b strings.Builder

	// ── Header ───────────────────────────────────────────────────────────────
	modeLabel := styleModeIncl.Render("[INCLUSIÓN]")
	if m.mode == ModeExclusion {
		modeLabel = styleModeExcl.Render("[EXCLUSIÓN]")
	}
	b.WriteString(fmt.Sprintf("\n  %s  %s\n",
		styleTitle.Render("n8n Workflow Exporter"),
		modeLabel,
	))
	b.WriteString("  " + strings.Repeat("─", 55) + "\n")
	b.WriteString(styleHelp.Render("  ↑/↓ navegar   SPACE marcar   TAB cambiar modo   ENTER exportar   q salir") + "\n\n")

	// ── Workflow list ─────────────────────────────────────────────────────────
	for i, wf := range m.workflows {
		var cursor, check, name string

		if i == m.cursor {
			cursor = styleCursor.Render("▶")
		} else {
			cursor = " "
		}

		if m.marked[i] {
			check = styleChecked.Render("[✓]")
			name = styleChecked.Render(wf.Name)
		} else {
			check = styleUnchecked.Render("[ ]")
			name = wf.Name
		}

		b.WriteString(fmt.Sprintf("  %s %s  %s\n", cursor, check, name))
	}

	// ── Footer ────────────────────────────────────────────────────────────────
	selected := len(m.GetSelectedWorkflows())
	b.WriteString(fmt.Sprintf("\n  %s %s / %d\n\n",
		styleFooter.Render("Workflows a exportar:"),
		styleCount.Render(fmt.Sprintf("%d", selected)),
		len(m.workflows),
	))

	return b.String()
}

// GetSelectedWorkflows returns the workflows to export based on the current mode
// and marked set. Never returns nil.
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
