package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
			if a.LastAction == nil {
				return a, nil
			}
			action := *a.LastAction
			a.LastError = ""
			a.Progress = nil
			return a, a.runAction(action)
		case "enter":
			action, ok := a.selectedAction()
			if !ok {
				return a, nil
			}
			a.LastError = ""
			a.Progress = nil
			return a, a.runAction(action)
		}
	case commandFinishedMsg:
		a.LastAction = &msg.action
		a.LastResult = &msg.result
		a.Progress = msg.events
		if msg.err != nil && msg.result.ExitCode != 0 {
			a.LastError = msg.err.Error()
		} else {
			a.LastError = ""
		}
		return a, nil
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

func (a App) runAction(action Action) tea.Cmd {
	return func() tea.Msg {
		args := action.Args(a.actionContext())
		result, err := a.Runner.Run(context.Background(), args)
		events, parseErr := ReadProgressEvents(strings.NewReader(result.Stderr))
		if parseErr != nil {
			events = nil
		}
		return commandFinishedMsg{action: action, result: result, err: err, events: events}
	}
}

func Run(opts AppOptions) error {
	_, err := tea.NewProgram(NewApp(opts)).Run()
	return err
}
