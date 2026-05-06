package oaercli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/open-aer/oaer/internal/probe"
)

func runProbe(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(args) == 0 {
		return fmt.Errorf("usage: oaer probe org [--project <dir>] [--target-org <alias>] [--output <dir>] [probe-id ...]")
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "org":
		return runProbeOrg(ctx, subargs, w)
	default:
		return fmt.Errorf("unknown probe subcommand %q", subcommand)
	}
}

func runProbeOrg(ctx context.Context, args []string, w io.Writer) error {
	probeDir := "probes/sfdx"
	orgAlias := ""
	outputDir := "probes/output"
	var probeIDs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--project":
			if i+1 >= len(args) {
				return fmt.Errorf("--project requires a value")
			}
			probeDir = args[i+1]
			i++
		case "--target-org":
			if i+1 >= len(args) {
				return fmt.Errorf("--target-org requires a value")
			}
			orgAlias = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return fmt.Errorf("--output requires a value")
			}
			outputDir = args[i+1]
			i++
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown flag %q", arg)
			}
			probeIDs = append(probeIDs, arg)
		}
	}

	if orgAlias == "" {
		return fmt.Errorf("--target-org is required (provide a sfdx org alias or username)")
	}

	cfg := probe.Config{
		ProbeDir:  probeDir,
		OrgAlias:  orgAlias,
		OutputDir: outputDir,
		ProbeIDs:  probeIDs,
	}

	report, err := probe.Run(cfg)
	if err != nil {
		return err
	}

	if report.GapsFound > 0 {
		fmt.Fprintf(w, "\nProbe run complete with %d gaps.\n", report.GapsFound)
	} else {
		fmt.Fprintln(w, "\nProbe run complete — no gaps found.")
	}

	return nil
}
