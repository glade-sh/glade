package tui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/glade-sh/glade/internal/cliui"
	"github.com/muesli/termenv"
)

type viewStyles struct {
	header          lipgloss.Style
	meta            lipgloss.Style
	section         lipgloss.Style
	activeTab       lipgloss.Style
	inactiveTab     lipgloss.Style
	selectedPointer lipgloss.Style
	actionLabel     lipgloss.Style
	description     lipgloss.Style
	dim             lipgloss.Style
	progress        lipgloss.Style
	success         lipgloss.Style
	failure         lipgloss.Style
}

func (a App) View() string {
	styles := currentViewStyles()
	var b strings.Builder
	meta := []string{
		styles.meta.Render("project=" + a.ProjectRoot),
		styles.meta.Render("db=" + displayDB(a.DBPath)),
	}
	if a.TargetOrg != "" {
		meta = append(meta, styles.meta.Render("org="+a.TargetOrg))
	}
	fmt.Fprintf(&b, "%s  %s\n", styles.header.Render("Glade TUI"), strings.Join(meta, "  "))
	b.WriteString(a.renderTabs(styles))
	b.WriteString("\n\n")
	b.WriteString(styles.section.Render("Actions"))
	b.WriteString("\n")
	b.WriteString(a.renderActions(styles))
	b.WriteString("\n")
	b.WriteString(a.renderProgress(styles))
	b.WriteString(a.renderLastResult(styles))
	b.WriteString("\n")
	b.WriteString(styles.dim.Render("1 project  2 tests  3 data  4 plugins  enter run  r rerun  q quit"))
	return b.String()
}

func (a App) renderTabs(styles viewStyles) string {
	labels := make([]string, 0, len(Boards()))
	for _, board := range Boards() {
		label := string(board)
		if board == a.ActiveBoard {
			label = styles.activeTab.Render(" " + label + " ")
		} else {
			label = styles.inactiveTab.Render(label)
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, " ")
}

func (a App) renderActions(styles viewStyles) string {
	actions := a.currentActions()
	if len(actions) == 0 {
		return "No actions."
	}
	selected := a.Selected[a.ActiveBoard]
	var lines []string
	for i, action := range actions {
		prefix := "  "
		if i == selected {
			prefix = styles.selectedPointer.Render("> ")
		}
		label := action.Label
		description := action.Description
		if i == selected {
			label = styles.actionLabel.Render(label)
			description = styles.description.Render(description)
		}
		line := fmt.Sprintf("%s%s - %s", prefix, label, description)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (a App) renderProgress(styles viewStyles) string {
	if len(a.Progress) == 0 {
		return ""
	}
	lines := []string{"\n" + styles.section.Render("Progress")}
	for _, event := range a.Progress {
		label := strings.TrimSpace(event.Label)
		if label == "" {
			label = string(event.Kind)
		}
		if event.Total > 0 {
			label = fmt.Sprintf("%s %d/%d", label, event.Current, event.Total)
		}
		lines = append(lines, "  "+styles.progress.Render(label))
	}
	return strings.Join(lines, "\n") + "\n"
}

func (a App) renderLastResult(styles viewStyles) string {
	if a.LastResult == nil {
		return ""
	}
	command := strings.Join(a.LastResult.Args, " ")
	result := "ok"
	resultStyle := styles.success
	if a.LastResult.ExitCode != 0 {
		result = "failed"
		resultStyle = styles.failure
	}
	lines := []string{
		"\n" + styles.section.Render("Last run"),
		"Command: " + styles.dim.Render("glade "+command),
		resultStyle.Render(fmt.Sprintf("Result: %s (exit %d)", result, a.LastResult.ExitCode)),
	}
	if a.LastError != "" {
		lines = append(lines, styles.failure.Render("Error: "+a.LastError))
	}
	if trimmed := strings.TrimSpace(a.LastResult.Stdout); trimmed != "" {
		lines = append(lines, styles.section.Render("Output"), indent(trimmed))
	}
	return strings.Join(lines, "\n") + "\n"
}

func currentViewStyles() viewStyles {
	if !cliui.ColorEnabled(true, os.Getenv("NO_COLOR")) {
		return viewStyles{}
	}
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.ANSI256)
	return viewStyles{
		header:          renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),
		meta:            renderer.NewStyle().Foreground(lipgloss.Color("245")),
		section:         renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),
		activeTab:       renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("36")),
		inactiveTab:     renderer.NewStyle().Foreground(lipgloss.Color("245")),
		selectedPointer: renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),
		actionLabel:     renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),
		description:     renderer.NewStyle().Foreground(lipgloss.Color("250")),
		dim:             renderer.NewStyle().Foreground(lipgloss.Color("245")),
		progress:        renderer.NewStyle().Foreground(lipgloss.Color("147")),
		success:         renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),
		failure:         renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("203")),
	}
}

func displayDB(dbPath string) string {
	if dbPath == "" {
		return "(not set)"
	}
	return dbPath
}

func indent(text string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return strings.Join(lines, "\n")
}
