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
	case "local":
		return runProbeLocal(ctx, subargs, w)
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

func runProbeLocal(ctx context.Context, args []string, w io.Writer) error {
	probeDir := "probes/sfdx"
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

	cfg := probe.Config{
		ProbeDir:  probeDir,
		OutputDir: outputDir,
		ProbeIDs:  probeIDs,
	}

	if len(cfg.ProbeIDs) == 0 {
		return fmt.Errorf("probe local requires at least one probe id")
	}

	localExec := &probe.LocalExecutor{ProbeDir: cfg.ProbeDir}
	results, err := localExec.CaptureLocal(cfg.ProbeIDs)
	if err != nil {
		return err
	}

	report := &probe.GapReport{ProbesRun: len(results)}
	if err := probe.WriteReport(report, outputDir+"/gap-report.json"); err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Fprintln(w, "No probes executed.")
		return nil
	}

	for _, r := range results {
		status := "OK"
		if r.ExceptionType != nil {
			status = "EXCEPTION"
		}
		fmt.Fprintf(w, "%s => %v (%s)\n", r.ProbeID, r.Result, status)
	}

	fmt.Fprintf(w, "\nLocal probe run complete: %d probes executed.\n", len(results))
	return nil
}
