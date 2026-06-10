package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/glade-sh/glade/internal/lwc"
)

func runRender(ctx context.Context, args []string, stdout io.Writer) error {
	_ = ctx
	if len(args) == 0 {
		return errors.New("usage: glade render lwc <component> [--project <root>] [--props <json>]")
	}
	switch args[0] {
	case "lwc":
		return runRenderLWC(args[1:], stdout)
	case "help", "-h", "--help":
		printRenderHelp(stdout)
		return nil
	default:
		return fmt.Errorf("unknown render target %q", args[0])
	}
}

func runRenderLWC(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: glade render lwc <component> [--project <root>] [--props <json>]")
	}
	component := args[0]
	root := "."
	propsJSON := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--props":
			if i+1 >= len(args) {
				return errors.New("--props requires a value")
			}
			propsJSON = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	overrides := map[string]string{}
	if propsJSON != "" {
		if err := json.Unmarshal([]byte(propsJSON), &overrides); err != nil {
			return fmt.Errorf("parse --props: %w", err)
		}
	}
	html, err := lwc.RenderComponentForTest(root, component, overrides)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, html)
	return nil
}

func printRenderHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  glade render lwc <component> [--project <root>] [--props <json>]")
}
