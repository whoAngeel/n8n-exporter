package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/whoAngeel/n8n-workflow-exported/n8nclient"
)

type fetchResultMsg struct {
	workflows []n8nclient.Workflow
	err       error
}

type loaderModel struct {
	spinner spinner.Model
	msg     string
	result  *fetchResultMsg
	fetchFn func() ([]n8nclient.Workflow, error)
}

func (m loaderModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			wfs, err := m.fetchFn()
			return fetchResultMsg{workflows: wfs, err: err}
		},
	)
}

func (m loaderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fetchResultMsg:
		m.result = &msg
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.result = &fetchResultMsg{err: fmt.Errorf("cancelled")}
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m loaderModel) View() string {
	if m.result != nil {
		return ""
	}
	return "\n  " + m.spinner.View() + "  " + m.msg + "\n"
}

// FetchWithSpinner shows an animated spinner while fn fetches workflows from n8n.
func FetchWithSpinner(baseURL string, fn func() ([]n8nclient.Workflow, error)) ([]n8nclient.Workflow, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	connMsg := lipgloss.NewStyle().Faint(true).Render("Connecting to ") +
		lipgloss.NewStyle().Bold(true).Render(baseURL)

	m := loaderModel{spinner: s, msg: connMsg, fetchFn: fn}
	p := tea.NewProgram(m)

	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	lm := final.(loaderModel)
	if lm.result == nil {
		return nil, fmt.Errorf("fetch cancelled")
	}
	return lm.result.workflows, lm.result.err
}
