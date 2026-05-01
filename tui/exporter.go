package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/whoAngeel/n8n-workflow-exported/n8nclient"
)

// ExportFunc exports one workflow and returns its filename.
type ExportFunc func(wf n8nclient.Workflow) (filename string, err error)

// ExportResult holds the outcome of a single workflow export.
type ExportResult struct {
	Name     string
	Filename string
	Err      error
}

type exportMsg ExportResult

type exportProgressModel struct {
	prog      progress.Model
	workflows []n8nclient.Workflow
	exportFn  ExportFunc
	results   []ExportResult
	current   int
}

var (
	styleExOK  = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleExErr = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleExDim = lipgloss.NewStyle().Faint(true)
)

func (m exportProgressModel) Init() tea.Cmd {
	return m.doNext()
}

func (m exportProgressModel) doNext() tea.Cmd {
	if m.current >= len(m.workflows) {
		return tea.Quit
	}
	wf := m.workflows[m.current]
	fn := m.exportFn
	return func() tea.Msg {
		filename, err := fn(wf)
		return exportMsg{Name: wf.Name, Filename: filename, Err: err}
	}
}

func (m exportProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case exportMsg:
		m.results = append(m.results, ExportResult(msg))
		m.current++
		return m, m.doNext()
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m exportProgressModel) View() string {
	total := len(m.workflows)
	if total == 0 {
		return ""
	}

	pct := float64(m.current) / float64(total)
	if pct > 1 {
		pct = 1
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n  %s  %d/%d\n\n",
		m.prog.ViewAs(pct),
		m.current, total,
	))

	for _, r := range m.results {
		if r.Err != nil {
			b.WriteString(fmt.Sprintf("  %s %-40s  %s\n",
				styleExErr.Render("✗"), r.Name,
				styleExDim.Render(r.Err.Error()),
			))
		} else {
			b.WriteString(fmt.Sprintf("  %s %-40s  %s\n",
				styleExOK.Render("✓"), r.Name,
				styleExDim.Render("→ "+r.Filename),
			))
		}
	}

	return b.String()
}

// RunExport runs all exports showing a progress bar with per-file results.
func RunExport(workflows []n8nclient.Workflow, exportFn ExportFunc) ([]ExportResult, error) {
	m := exportProgressModel{
		prog:      progress.New(progress.WithDefaultGradient(), progress.WithWidth(40)),
		workflows: workflows,
		exportFn:  exportFn,
	}

	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	em := final.(exportProgressModel)
	return em.results, nil
}
