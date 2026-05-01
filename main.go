package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/whoAngeel/n8n-workflow-exported/credentials"
	"github.com/whoAngeel/n8n-workflow-exported/exporter"
	"github.com/whoAngeel/n8n-workflow-exported/n8nclient"
	"github.com/whoAngeel/n8n-workflow-exported/tui"
)

const version = "0.7.0"

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	styleOK   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))  // green
	styleErr  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
	styleWarn = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // orange
	styleDim  = lipgloss.NewStyle().Faint(true)
	styleBold = lipgloss.NewStyle().Bold(true)
)

func main() {
	clearScreen()
	fmt.Println()
	printBanner()

	// Handle --reset flag: delete saved credentials and exit.
	if len(os.Args) > 1 && os.Args[1] == "--reset" {
		if err := credentials.DeleteCredentials(); err != nil {
			fmt.Fprintln(os.Stderr, styleErr.Render("✗ Could not delete credentials: "+err.Error()))
			os.Exit(1)
		}
		fmt.Println(styleOK.Render("✓ Saved credentials deleted. Next run will prompt for new credentials."))
		os.Exit(0)
	}

	// Step 1: Collect credentials interactively (stored in memory only).
	creds, err := credentials.CollectCredentials()
	if err != nil {
		fmt.Fprintln(os.Stderr, styleErr.Render("✗ Error collecting credentials: "+err.Error()))
		os.Exit(1)
	}

	// Step 2: Fetch all workflows from n8n (animated spinner while waiting).
	client := n8nclient.NewN8NClient(creds)
	workflows, err := tui.FetchWithSpinner(creds.BaseURL, client.GetAllWorkflows)
	if err != nil {
		fmt.Fprintln(os.Stderr, styleErr.Render("\n✗ Failed to connect to n8n: "+err.Error()))
		os.Exit(1)
	}

	if len(workflows) == 0 {
		fmt.Println(styleWarn.Render("⚠  No workflows found in this n8n instance."))
		os.Exit(0)
	}

	fmt.Printf("%s %s\n",
		styleOK.Render("✓"),
		styleBold.Render(fmt.Sprintf("Found %d workflow(s)", len(workflows))),
	)

	// Sort workflows alphabetically by name (case-insensitive).
	sort.Slice(workflows, func(i, j int) bool {
		return strings.ToLower(workflows[i].Name) < strings.ToLower(workflows[j].Name)
	})

	// Step 3: Interactive TUI selection.
	model := tui.NewSelectorModel(workflows)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, styleErr.Render("✗ UI error: "+err.Error()))
		os.Exit(1)
	}

	result, ok := finalModel.(tui.SelectorModel)
	if !ok || result.Cancelled {
		fmt.Println("\n" + styleWarn.Render("⚠  Export cancelled."))
		os.Exit(0)
	}

	selected := result.GetSelectedWorkflows()
	if len(selected) == 0 {
		fmt.Println("\n" + styleWarn.Render("⚠  No workflows selected for export."))
		os.Exit(0)
	}

	// Step 4: Export and clean selected workflows to disk.
	outputDir := exporter.ResolveOutputDir(creds.OutputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, styleErr.Render(fmt.Sprintf("✗ Cannot create output directory %q: %v", outputDir, err)))
		os.Exit(1)
	}

	fmt.Printf("\n%s\n",
		styleBold.Render(fmt.Sprintf("📦 Exporting %d workflow(s)...", len(selected))),
	)

	exportFn := func(wf n8nclient.Workflow) (string, error) {
		filename := exporter.SanitizeFilename(wf.Name) + ".json"
		outPath := filepath.Join(outputDir, filename)
		cleaned := exporter.CleanWorkflow(wf.Raw)
		data, err := json.MarshalIndent(cleaned, "", "  ")
		if err != nil {
			return "", err
		}
		return filename, os.WriteFile(outPath, data, 0644)
	}

	results, err := tui.RunExport(selected, exportFn)
	if err != nil {
		fmt.Fprintln(os.Stderr, styleErr.Render("✗ Export error: "+err.Error()))
		os.Exit(1)
	}

	okCount, fail := 0, 0
	for _, r := range results {
		if r.Err != nil {
			fail++
		} else {
			okCount++
		}
	}

	// Summary
	fmt.Println()
	if fail == 0 {
		fmt.Println(styleOK.Render(fmt.Sprintf("✓ Done: %d exported, 0 errors.", okCount)))
	} else {
		fmt.Println(styleWarn.Render(fmt.Sprintf("⚠  Done: %d exported, %d errors.", okCount, fail)))
	}
	fmt.Println(styleDim.Render("📁 Output: " + outputDir))
	fmt.Println()
}

// clearScreen clears the terminal on Windows (cls) and Unix (clear).
func clearScreen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}

// printBanner prints the ASCII art with version info alongside.
func printBanner() {
	ascii := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true).
		Render("▄▄  ▄▄ ▄████▄ ▄▄  ▄▄   ▄▄▄▄▄ ▄▄ ▄▄ ▄▄▄▄   ▄▄▄  ▄▄▄▄  ▄▄▄▄▄▄ ▄▄▄▄▄ ▄▄▄▄\n███▄██ ██▄▄██ ███▄██   ██▄▄  ▀█▄█▀ ██▄█▀ ██▀██ ██▄█▄   ██   ██▄▄  ██▄█▄\n██ ▀██ ▀█▄▄█▀ ██ ▀██   ██▄▄▄ ██ ██ ██    ▀███▀ ██ ██   ██   ██▄▄▄ ██ ██")

	info := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			styleDim.Render("n8n workflow exporter"),
			styleBold.Render("v"+version),
		))

	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Center, ascii, "  ", info))
	fmt.Println()
}
