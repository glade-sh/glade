package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle   = lipgloss.NewStyle().Bold(true)
	activeStyle   = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true)
	dimStyle      = lipgloss.NewStyle().Faint(true)
)

func (a App) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  project=%s  db=%s\n", headerStyle.Render("Glade TUI"), a.ProjectRoot, a.DBPath)
	b.WriteString(a.renderTabs())
	b.WriteString("\n\n")
	b.WriteString(a.renderActions())
	b.WriteString("\n")
	b.WriteString(a.renderProgress())
	b.WriteString(a.renderLastResult())
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("1 project  2 tests  3 data  4 plugins  enter run  r rerun  q quit"))
	return b.String()
}

func (a App) renderTabs() string {
	labels := make([]string, 0, len(Boards()))
	for _, board := range Boards() {
		label := string(board)
		if board == a.ActiveBoard {
			label = activeStyle.Render("[" + label + "]")
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, " ")
}

func (a App) renderActions() string {
	actions := a.currentActions()
	if len(actions) == 0 {
		return "No actions."
	}
	selected := a.Selected[a.ActiveBoard]
	var lines []string
	for i, action := range actions {
		prefix := "  "
		if i == selected {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%s - %s", prefix, action.Label, action.Description)
		if i == selected {
			line = selectedStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (a App) renderProgress() string {
	if len(a.Progress) == 0 {
		return ""
	}
	lines := []string{"\nProgress:"}
	for _, event := range a.Progress {
		label := strings.TrimSpace(event.Label)
		if label == "" {
			label = string(event.Kind)
		}
		if event.Total > 0 {
			label = fmt.Sprintf("%s %d/%d", label, event.Current, event.Total)
		}
		lines = append(lines, "  "+label)
	}
	return strings.Join(lines, "\n") + "\n"
}

func (a App) renderLastResult() string {
	if a.LastResult == nil {
		return ""
	}
	command := strings.Join(a.LastResult.Args, " ")
	lines := []string{fmt.Sprintf("\nLast: glade %s", command), fmt.Sprintf("Exit: %d", a.LastResult.ExitCode)}
	if a.LastError != "" {
		lines = append(lines, "Error: "+a.LastError)
	}
	if trimmed := strings.TrimSpace(a.LastResult.Stdout); trimmed != "" {
		lines = append(lines, "Output:", indent(trimmed))
	}
	return strings.Join(lines, "\n") + "\n"
}

func indent(text string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return strings.Join(lines, "\n")
}
