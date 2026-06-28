package tui

import (
	"context"
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glade-sh/glade/internal/cliui"
)

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "1":
			a.ActiveBoard = BoardProject
			return a, nil
		case "2":
			a.ActiveBoard = BoardTests
			return a, nil
		case "3":
			a.ActiveBoard = BoardData
			return a, nil
		case "4":
			a.ActiveBoard = BoardPlugins
			return a, nil
		case "up", "k":
			a.moveSelection(-1)
			return a, nil
		case "down", "j":
			a.moveSelection(1)
			return a, nil
		case "r":
			if a.RunningAction != nil {
				return a, nil
			}
			if a.LastAction == nil {
				return a, nil
			}
			action := *a.LastAction
			a.LastError = ""
			a.Progress = nil
			args := action.Args(a.actionContext())
			a.RunningAction = &action
			a.RunningArgs = args
			return a, a.runAction(action, args)
		case "enter":
			if a.RunningAction != nil {
				return a, nil
			}
			action, ok := a.selectedAction()
			if !ok {
				return a, nil
			}
			a.LastError = ""
			a.Progress = nil
			args := action.Args(a.actionContext())
			a.RunningAction = &action
			a.RunningArgs = args
			return a, a.runAction(action, args)
		}
	case commandFinishedMsg:
		a.LastAction = &msg.action
		a.LastResult = &msg.result
		a.RunningAction = nil
		a.RunningArgs = nil
		if len(msg.events) > 0 {
			a.Progress = msg.events
		}
		if msg.err != nil && msg.result.ExitCode != 0 && visibleStderr(msg.result.Stderr) == "" {
			a.LastError = msg.err.Error()
		} else {
			a.LastError = ""
		}
		return a, nil
	case commandStreamStartedMsg:
		return a, waitForRunUpdate(msg.action, msg.ch)
	case commandProgressMsg:
		if msg.event != nil {
			a.Progress = append(a.Progress, *msg.event)
		}
		return a, waitForRunUpdate(msg.action, msg.ch)
	}
	return a, nil
}

func (a *App) moveSelection(delta int) {
	actions := a.currentActions()
	if len(actions) == 0 {
		a.Selected[a.ActiveBoard] = 0
		return
	}
	next := a.Selected[a.ActiveBoard] + delta
	if next < 0 {
		next = len(actions) - 1
	}
	if next >= len(actions) {
		next = 0
	}
	a.Selected[a.ActiveBoard] = next
}

func (a App) runAction(action Action, args []string) tea.Cmd {
	if runner, ok := a.Runner.(StreamingRunner); ok {
		return func() tea.Msg {
			ch, err := runner.RunStreaming(context.Background(), args)
			if err != nil {
				return commandFinishedMsg{
					action: action,
					result: RunResult{Args: append([]string{}, args...), ExitCode: 1},
					err:    err,
				}
			}
			return commandStreamStartedMsg{action: action, ch: ch}
		}
	}
	return func() tea.Msg {
		result, err := a.Runner.Run(context.Background(), args)
		output, parseErr := ReadProgressOutput(strings.NewReader(result.Stderr))
		if parseErr == nil {
			return commandFinishedMsg{action: action, result: result, err: err, events: output.Events}
		}
		return commandFinishedMsg{action: action, result: result, err: err}
	}
}

func waitForRunUpdate(action Action, ch <-chan RunUpdate) tea.Cmd {
	return func() tea.Msg {
		update, ok := <-ch
		if !ok {
			return commandFinishedMsg{
				action: action,
				result: RunResult{ExitCode: 1},
				err:    errors.New("command stream closed before result"),
			}
		}
		if update.Result != nil {
			events := []cliui.Event(nil)
			if update.Result.Stderr != "" {
				if output, err := ReadProgressOutput(strings.NewReader(update.Result.Stderr)); err == nil {
					events = output.Events
				}
			}
			return commandFinishedMsg{action: action, result: *update.Result, err: update.Err, events: events}
		}
		return commandProgressMsg{action: action, ch: ch, event: update.Event, stderr: update.Stderr}
	}
}

func Run(opts AppOptions) error {
	_, err := tea.NewProgram(NewApp(opts)).Run()
	return err
}
