package tui

type Board string

const (
	BoardProject Board = "project"
	BoardTests   Board = "tests"
	BoardData    Board = "data"
	BoardPlugins Board = "plugins"
)

type ActionContext struct {
	ProjectRoot string
	DBPath      string
	Query       string
	Fixture    string
}

type Action struct {
	ID          string
	Board       Board
	Label       string
	Description string
	Args        func(ActionContext) []string
}

type Catalog struct {
	Actions []Action
}

func DefaultCatalog() Catalog {
	return Catalog{Actions: []Action{
		{ID: "project.doctor", Board: BoardProject, Label: "Doctor", Description: "Check tool and project state.", Args: projectArgs("doctor", "--json")},
		{ID: "project.schema", Board: BoardProject, Label: "Load Schema", Description: "Load project metadata.", Args: projectArgs("schema", "load", "--json", "--progress-json")},
		{ID: "project.check", Board: BoardProject, Label: "Check", Description: "Run semantic checks.", Args: projectArgs("check", "--json", "--progress-json")},
		{ID: "tests.changed", Board: BoardTests, Label: "Changed Tests", Description: "Run tests changed since HEAD.", Args: func(ctx ActionContext) []string {
			return []string{"test", "changed", "--project", ctx.ProjectRoot, "--since", "HEAD", "--json", "--progress-json"}
		}},
		{ID: "tests.lastFailed", Board: BoardTests, Label: "Last Failed", Description: "Run last failed tests.", Args: projectArgs("test", "--last-failed", "--json", "--progress-json")},
		{ID: "tests.daemon", Board: BoardTests, Label: "Daemon Status", Description: "Inspect the test daemon.", Args: projectArgs("test", "daemon", "status")},
		{ID: "data.inspect", Board: BoardData, Label: "Inspect DB", Description: "Show local DB counts.", Args: dbArgs("inspect", "--json")},
		{ID: "data.query", Board: BoardData, Label: "Query DB", Description: "Run local SOQL.", Args: func(ctx ActionContext) []string {
			return []string{"db", "query", "--db", ctx.DBPath, "--project", ctx.ProjectRoot, "--json", defaultString(ctx.Query, "SELECT Id FROM Account LIMIT 10")}
		}},
		{ID: "data.seed", Board: BoardData, Label: "Seed DB", Description: "Apply a fixture to the local DB.", Args: func(ctx ActionContext) []string {
			return []string{"db", "seed", "--db", ctx.DBPath, "--project", ctx.ProjectRoot, "--json", "--progress-json", defaultString(ctx.Fixture, "fixture.json")}
		}},
		{ID: "data.reset", Board: BoardData, Label: "Reset DB", Description: "Clear local DB data.", Args: dbArgs("reset", "--json")},
		{ID: "data.export", Board: BoardData, Label: "Export DB", Description: "Write fixture JSON to stdout.", Args: dbArgs("export")},
		{ID: "plugins.list", Board: BoardPlugins, Label: "Installed Plugins", Description: "List installed plugins.", Args: func(ActionContext) []string {
			return []string{"plugins", "list", "--json"}
		}},
		{ID: "plugins.available", Board: BoardPlugins, Label: "Available Plugins", Description: "List installable plugins.", Args: func(ActionContext) []string {
			return []string{"plugins", "available", "--progress-json"}
		}},
		{ID: "plugins.doctor", Board: BoardPlugins, Label: "Plugin Doctor", Description: "Check plugin state.", Args: func(ActionContext) []string {
			return []string{"plugins", "doctor", "--json", "--progress-json"}
		}},
	}}
}

func (c Catalog) Action(id string) (Action, bool) {
	for _, action := range c.Actions {
		if action.ID == id {
			return action, true
		}
	}
	return Action{}, false
}

func (c Catalog) ActionsForBoard(board Board) []Action {
	var out []Action
	for _, action := range c.Actions {
		if action.Board == board {
			out = append(out, action)
		}
	}
	return out
}

func Boards() []Board {
	return []Board{BoardProject, BoardTests, BoardData, BoardPlugins}
}

func BoardFromString(value string) (Board, bool) {
	for _, board := range Boards() {
		if string(board) == value {
			return board, true
		}
	}
	return "", false
}

func projectArgs(parts ...string) func(ActionContext) []string {
	return func(ctx ActionContext) []string {
		args := append([]string{}, parts...)
		return append(args, "--project", ctx.ProjectRoot)
	}
}

func dbArgs(command string, tail ...string) func(ActionContext) []string {
	return func(ctx ActionContext) []string {
		args := []string{"db", command, "--db", ctx.DBPath, "--project", ctx.ProjectRoot}
		return append(args, tail...)
	}
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
