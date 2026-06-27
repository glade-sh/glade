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
		String("view", "").
		String("query", "").
		String("fixture", "").
		Bool("no-ui", "").
		Parse(args)
	if err != nil {
		return err
	}
	root := parsed.String("project")
	if root == "" {
		root = "."
	}
	board := tui.BoardProject
	if parsed.String("view") != "" {
		parsedBoard, ok := tui.BoardFromString(parsed.String("view"))
		if !ok {
			return fmt.Errorf("--view must be one of project, tests, data, plugins")
		}
		board = parsedBoard
	}
	opts := tui.AppOptions{
		ProjectRoot:  root,
		DBPath:       parsed.String("db"),
		Query:        parsed.String("query"),
		Fixture:      parsed.String("fixture"),
		InitialBoard: board,
		Runner:       tui.ExecRunner{Dir: root},
	}
	if parsed.Bool("no-ui") {
		return writeTUIDryRun(stdout, opts)
	}
	return tui.Run(opts)
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
	if _, err := fmt.Fprintf(w, "view: %s\n", opts.InitialBoard); err != nil {
		return err
	}
	return nil
}
