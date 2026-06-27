package tui

import "github.com/glade-sh/glade/internal/cliui"

type AppOptions struct {
	ProjectRoot  string
	DBPath       string
	Query        string
	Fixture      string
	InitialBoard Board
	Runner       Runner
}

type CommandState struct {
	Action   Action
	Running  bool
	Error    string
	ExitCode int
	Stdout   string
	Stderr   string
}

type App struct {
	ProjectRoot string
	DBPath      string
	Query       string
	Fixture     string

	Catalog     Catalog
	ActiveBoard Board
	Selected    map[Board]int
	LastAction  *Action
	LastResult  *RunResult
	LastError   string
	Progress    []cliui.Event
	Runner      Runner
}

type commandFinishedMsg struct {
	action Action
	result RunResult
	err    error
	events []cliui.Event
}

func NewApp(opts AppOptions) App {
	board := opts.InitialBoard
	if board == "" {
		board = BoardProject
	}
	runner := opts.Runner
	if runner == nil {
		runner = ExecRunner{Dir: opts.ProjectRoot}
	}
	return App{
		ProjectRoot: defaultString(opts.ProjectRoot, "."),
		DBPath:      defaultString(opts.DBPath, ".glade/envs/dev.sqlite"),
		Query:       opts.Query,
		Fixture:     opts.Fixture,
		Catalog:     DefaultCatalog(),
		ActiveBoard: board,
		Selected:    make(map[Board]int),
		Runner:      runner,
	}
}

func (a App) actionContext() ActionContext {
	return ActionContext{
		ProjectRoot: a.ProjectRoot,
		DBPath:      a.DBPath,
		Query:       a.Query,
		Fixture:     a.Fixture,
	}
}

func (a App) currentActions() []Action {
	return a.Catalog.ActionsForBoard(a.ActiveBoard)
}

func (a App) selectedAction() (Action, bool) {
	actions := a.currentActions()
	if len(actions) == 0 {
		return Action{}, false
	}
	index := a.Selected[a.ActiveBoard]
	if index < 0 {
		index = 0
	}
	if index >= len(actions) {
		index = len(actions) - 1
	}
	return actions[index], true
}
