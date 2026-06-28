package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glade-sh/glade/internal/cliui"
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

func TestAppViewShowsSelectedActionContext(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	app := NewApp(AppOptions{ProjectRoot: "/tmp/acme", DBPath: ".glade/envs/dev.sqlite", TargetOrg: "devhub", InitialBoard: BoardData, Runner: &fakeRunner{}})
	app.Selected[BoardData] = 3
	view := app.View()
	for _, want := range []string{
		"Selected",
		"Command: glade db import sf",
		"DB: .glade/envs/dev.sqlite",
		"Object: Account",
		"Limit: 25",
		"Target org: devhub",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("selected context missing %q:\n%s", want, view)
		}
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

type failingRunner struct{}

func (failingRunner) Run(_ context.Context, args []string) (RunResult, error) {
	return RunResult{
		Args:     append([]string{}, args...),
		ExitCode: 1,
		Stderr: strings.Join([]string{
			`{"kind":"phase_start","phase":"db seed","label":"Opening fixture"}`,
			`{"kind":"done","label":"db seed failed","ok":false,"exitCode":1}`,
			"glade: open fixture.json: no such file or directory",
		}, "\n"),
	}, errors.New("exit status 1")
}

func TestAppViewShowsChildStderrOnFailureWithMixedProgressJSON(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	app := NewApp(AppOptions{ProjectRoot: "/tmp/acme", DBPath: ".glade/envs/dev.sqlite", InitialBoard: BoardData, Runner: failingRunner{}})
	app.Selected[BoardData] = 2
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command")
	}
	model, _ = model.Update(cmd())
	view := model.(App).View()
	for _, want := range []string{
		"Progress",
		"Opening fixture",
		"db seed failed",
		"Error output",
		"glade: open fixture.json: no such file or directory",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("failure view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Error: exit status 1") {
		t.Fatalf("failure view should prefer child stderr over wrapper status:\n%s", view)
	}
}

type streamingRunner struct {
	ch chan RunUpdate
}

func (r *streamingRunner) Run(_ context.Context, args []string) (RunResult, error) {
	return RunResult{Args: append([]string{}, args...), ExitCode: 0}, nil
}

func (r *streamingRunner) RunStreaming(_ context.Context, args []string) (<-chan RunUpdate, error) {
	r.ch = make(chan RunUpdate, 2)
	r.ch <- RunUpdate{Args: append([]string{}, args...), Event: &cliui.Event{Kind: cliui.EventPhaseStart, Phase: "check", Label: "Loading project"}}
	return r.ch, nil
}

func TestAppStreamsProgressBeforeCommandFinishes(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	runner := &streamingRunner{}
	app := NewApp(AppOptions{ProjectRoot: "/tmp/acme", DBPath: ".glade/envs/dev.sqlite", Runner: runner})
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected start command")
	}
	model, cmd = model.Update(cmd())
	if cmd == nil {
		t.Fatal("expected stream wait command")
	}
	model, cmd = model.Update(cmd())
	if cmd == nil {
		t.Fatal("expected stream to keep waiting after progress")
	}
	view := model.(App).View()
	for _, want := range []string{"Running", "Progress", "Loading project"} {
		if !strings.Contains(view, want) {
			t.Fatalf("streaming view missing %q:\n%s", want, view)
		}
	}
}
