package gladecli

import (
	"context"
	"fmt"
	"io"

	"github.com/glade-sh/glade/internal/flagparse"
	"github.com/glade-sh/glade/internal/tui"
)

func runTUI(ctx context.Context, args []string, stdout io.Writer, _ io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	parsed, err := flagparse.New("glade tui").
		String("project", "p").
		String("db", "").
		String("env", "").
		String("view", "").
		String("query", "").
		String("fixture", "").
		String("target-org", "o").
		String("object", "").
		Bool("no-ui", "").
		Parse(args)
	if err != nil {
		return err
	}
	root := parsed.String("project")
	if root == "" {
		root = "."
	}
	envName := parsed.String("env")
	if envName == "" {
		envName = "dev"
	}
	if parsed.String("env") != "" {
		if err := validateDBEnvName(envName); err != nil {
			return err
		}
	}
	board := tui.BoardProject
	if parsed.String("view") != "" {
		parsedBoard, ok := tui.BoardFromString(parsed.String("view"))
		if !ok {
			return fmt.Errorf("--view must be one of project, tests, data, plugins")
		}
		board = parsedBoard
	}
	dbPath := parsed.String("db")
	if dbPath == "" && board == tui.BoardData {
		dbPath = projectEnvDBPath(root, envName)
	}
	opts := tui.AppOptions{
		ProjectRoot:  root,
		DBPath:       dbPath,
		Query:        parsed.String("query"),
		Fixture:      parsed.String("fixture"),
		TargetOrg:    parsed.String("target-org"),
		ImportObject: parsed.String("object"),
		InitialBoard: board,
		Runner:       tui.ExecRunner{Dir: root},
	}
	if parsed.Bool("no-ui") {
		return writeTUIDryRun(stdout, opts)
	}
	return tui.Run(opts)
}

func runTUIView(ctx context.Context, args []string, board tui.Board, stdout io.Writer, stderr io.Writer) error {
	parsed, err := flagparse.New("glade tui alias").
		Bool("ui", "").
		String("project", "p").
		String("db", "").
		String("env", "").
		String("query", "").
		String("fixture", "").
		String("target-org", "o").
		String("object", "").
		Bool("no-ui", "").
		Parse(args)
	if err != nil {
		return err
	}
	if !parsed.Bool("ui") {
		return fmt.Errorf("missing --ui")
	}
	tuiArgs := []string{"--view", string(board)}
	if parsed.String("project") != "" {
		tuiArgs = append(tuiArgs, "--project", parsed.String("project"))
	}
	if parsed.String("db") != "" {
		tuiArgs = append(tuiArgs, "--db", parsed.String("db"))
	}
	if parsed.String("env") != "" {
		tuiArgs = append(tuiArgs, "--env", parsed.String("env"))
	}
	if parsed.String("query") != "" {
		tuiArgs = append(tuiArgs, "--query", parsed.String("query"))
	}
	if parsed.String("fixture") != "" {
		tuiArgs = append(tuiArgs, "--fixture", parsed.String("fixture"))
	}
	if parsed.String("target-org") != "" {
		tuiArgs = append(tuiArgs, "--target-org", parsed.String("target-org"))
	}
	if parsed.String("object") != "" {
		tuiArgs = append(tuiArgs, "--object", parsed.String("object"))
	}
	if parsed.Bool("no-ui") {
		tuiArgs = append(tuiArgs, "--no-ui")
	}
	return runTUI(ctx, tuiArgs, stdout, stderr)
}

func hasUIFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--ui" {
			return true
		}
	}
	return false
}

func writeTUIDryRun(w io.Writer, opts tui.AppOptions) error {
	if _, err := fmt.Fprintln(w, "Glade TUI"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "project: %s\n", opts.ProjectRoot); err != nil {
		return err
	}
	if opts.DBPath != "" {
		if _, err := fmt.Fprintf(w, "db: %s\n", opts.DBPath); err != nil {
			return err
		}
	}
	if opts.TargetOrg != "" {
		if _, err := fmt.Fprintf(w, "target-org: %s\n", opts.TargetOrg); err != nil {
			return err
		}
	}
	if opts.ImportObject != "" {
		if _, err := fmt.Fprintf(w, "object: %s\n", opts.ImportObject); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "view: %s\n", opts.InitialBoard); err != nil {
		return err
	}
	return nil
}
