package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type fakeRunner struct {
	args []string
}

func (r *fakeRunner) Run(_ context.Context, args []string) (RunResult, error) {
	r.args = append([]string{}, args...)
	return RunResult{
		Args:     args,
		ExitCode: 0,
		Stdout:   `{"status":"passed"}`,
		Stderr:   "{\"kind\":\"done\",\"label\":\"check complete\",\"ok\":true,\"exitCode\":0}\n",
	}, nil
}

func TestAppSwitchesBoards(t *testing.T) {
	app := NewApp(AppOptions{ProjectRoot: "/tmp/acme", DBPath: ".glade/envs/dev.sqlite", Runner: &fakeRunner{}})
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	next := model.(App)
	if next.ActiveBoard != BoardTests {
		t.Fatalf("board = %s, want tests", next.ActiveBoard)
	}
}

func TestAppRunsSelectedAction(t *testing.T) {
	runner := &fakeRunner{}
	app := NewApp(AppOptions{ProjectRoot: "/tmp/acme", DBPath: ".glade/envs/dev.sqlite", Runner: runner})
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	model, _ = model.Update(msg)
	next := model.(App)
	if len(runner.args) == 0 || runner.args[0] != "doctor" {
		t.Fatalf("runner args = %#v", runner.args)
	}
	if next.LastResult == nil || next.LastResult.ExitCode != 0 {
		t.Fatalf("last result = %#v", next.LastResult)
	}
	if len(next.Progress) != 1 || next.Progress[0].Label != "check complete" {
		t.Fatalf("progress = %#v", next.Progress)
	}
}

func TestAppViewShowsRunningCommandImmediately(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	app := NewApp(AppOptions{ProjectRoot: "/tmp/acme", DBPath: ".glade/envs/dev.sqlite", Runner: &fakeRunner{}})
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	view := model.(App).View()
	for _, want := range []string{"Running", "glade doctor --json --project /tmp/acme"} {
		if !strings.Contains(view, want) {
			t.Fatalf("running view missing %q:\n%s", want, view)
		}
	}
}

func TestAppViewShowsBoardAndActions(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	app := NewApp(AppOptions{ProjectRoot: "/tmp/acme", DBPath: ".glade/envs/dev.sqlite", Runner: &fakeRunner{}})
	view := app.View()
	for _, want := range []string{"Glade TUI", "project=/tmp/acme", "Doctor"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestAppViewShowsTargetOrgWhenSet(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	app := NewApp(AppOptions{ProjectRoot: "/tmp/acme", DBPath: ".glade/envs/dev.sqlite", TargetOrg: "devhub", Runner: &fakeRunner{}})
	view := app.View()
	if !strings.Contains(view, "org=devhub") {
		t.Fatalf("view missing target org:\n%s", view)
	}
}

func TestAppViewSummarizesDBInspectJSON(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	app := NewApp(AppOptions{ProjectRoot: "/tmp/acme", DBPath: ".glade/envs/dev.sqlite", Runner: &fakeRunner{}})
	app.LastResult = &RunResult{
		Args:     []string{"db", "import", "sf", "--db", ".glade/envs/dev.sqlite", "--project", "/tmp/acme", "--object", "Account", "--json"},
		ExitCode: 0,
		Stdout:   `{"path":".glade/envs/dev.sqlite","schemaVersion":1,"objects":3,"records":4,"byObject":{"Account":1,"User":2,"Profile":1},"users":2,"profiles":1,"permissions":0}`,
	}
	view := app.View()
	for _, want := range []string{"Database state", "records: 4", "Account: 1", "User: 2"} {
		if !strings.Contains(view, want) {
			t.Fatalf("summary view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, `"byObject"`) || strings.Contains(view, `"schemaVersion"`) {
		t.Fatalf("summary view leaked raw JSON:\n%s", view)
	}
}

func TestAppViewUsesAccentColorWhenAllowed(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	app := NewApp(AppOptions{ProjectRoot: "/tmp/acme", DBPath: ".glade/envs/dev.sqlite", Runner: &fakeRunner{}})
	view := app.View()
	if !strings.Contains(view, "\x1b[38;") {
		t.Fatalf("view missing accent color:\n%q", view)
	}
}

func TestAppViewHonorsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "xterm-256color")
	app := NewApp(AppOptions{ProjectRoot: "/tmp/acme", DBPath: ".glade/envs/dev.sqlite", Runner: &fakeRunner{}})
	view := app.View()
	if strings.Contains(view, "\x1b[") {
		t.Fatalf("view should not contain ANSI when NO_COLOR is set:\n%q", view)
	}
}

func TestAppViewLabelsResultState(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	app := NewApp(AppOptions{ProjectRoot: "/tmp/acme", DBPath: ".glade/envs/dev.sqlite", Runner: &fakeRunner{}})
	app.LastResult = &RunResult{Args: []string{"check", "--project", "/tmp/acme"}, ExitCode: 1}
	app.LastError = "check failed"
	view := app.View()
	for _, want := range []string{"Last run", "Result: failed (exit 1)", "Error: check failed"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "\x1b[38;") {
		t.Fatalf("view missing result color:\n%q", view)
	}
}
