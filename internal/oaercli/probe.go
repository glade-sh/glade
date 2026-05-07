package oaercli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/open-aer/oaer/internal/config"
	"github.com/open-aer/oaer/internal/probe"
)

func runProbe(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(args) == 0 {
		return fmt.Errorf("usage: oaer probe <subcommand> [args...]\n  subcommands: org, local, deploy")
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "org":
		return runProbeOrg(ctx, subargs, w)
	case "local":
		return runProbeLocal(ctx, subargs, w)
	case "deploy":
		return runProbeDeploy(ctx, subargs, w)
	case "tooling-snippet":
		return runProbeToolingSnippet(ctx, subargs, w)
	default:
		return fmt.Errorf("unknown probe subcommand %q", subcommand)
	}
}

func runProbeOrg(ctx context.Context, args []string, w io.Writer) error {
	probeDir := "probes/sfdx"
	orgAlias := ""
	outputDir := "probes/output"
	tier := "full"
	goldenCache := ""
	useGoldenCache := false
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
		case "--tier":
			if i+1 >= len(args) {
				return fmt.Errorf("--tier requires a value")
			}
			tier = args[i+1]
			i++
		case "--golden-cache":
			if i+1 >= len(args) {
				return fmt.Errorf("--golden-cache requires a value")
			}
			goldenCache = args[i+1]
			i++
		case "--use-golden-cache":
			useGoldenCache = true
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

	// Load oaer.yml features if present
	var features []string
	cfgFile, _, err := config.LoadNearest(probeDir)
	if err == nil {
		features = cfgFile.Org.Features
	}

	cfg := probe.Config{
		ProbeDir:       probeDir,
		OrgAlias:       orgAlias,
		OutputDir:      outputDir,
		ProbeIDs:       probeIDs,
		Features:       features,
		Tier:           tier,
		GoldenCache:    goldenCache,
		UseGoldenCache: useGoldenCache,
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
	var features []string

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
		case "--feature":
			if i+1 >= len(args) {
				return fmt.Errorf("--feature requires a value")
			}
			features = append(features, args[i+1])
			i++
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown flag %q", arg)
			}
			probeIDs = append(probeIDs, arg)
		}
	}

	// Load oaer.yml features if present
	cfg, _, err := config.LoadNearest(probeDir)
	if err == nil {
		features = append(features, cfg.Org.Features...)
	}

	if len(probeIDs) == 0 {
		return fmt.Errorf("probe local requires at least one probe id")
	}

	localExec := &probe.LocalExecutor{ProbeDir: probeDir, Features: features}
	results, _, err := localExec.CaptureLocal(probeIDs)
	if err != nil {
		return err
	}

	report := &probe.LocalRunReport{
		ProbesRun: len(results),
		Results:   make([]probe.ProbeResult, 0, len(results)),
	}
	for _, id := range probeIDs {
		if result, ok := results[id]; ok {
			report.Results = append(report.Results, result)
		}
	}
	if err := probe.WriteLocalRunReport(report, outputDir+"/local-results.json"); err != nil {
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

func runProbeDeploy(ctx context.Context, args []string, w io.Writer) error {
	probeDir := "probes/sfdx"
	orgAlias := ""

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
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown flag %q", arg)
			}
		}
	}

	if orgAlias == "" {
		return fmt.Errorf("--target-org is required")
	}

	deployer := probe.NewDeployer(probeDir, orgAlias)
	if err := deployer.Deploy(ctx, w); err != nil {
		return err
	}
	return nil
}

func runProbeToolingSnippet(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	probeDir := "probes/sfdx"
	orgAlias := ""
	outputPath := ""
	manifestPath := ""
	id := "snippet"
	category := ""
	source := ""
	file := ""
	jsonOut := false

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
			outputPath = args[i+1]
			i++
		case "--manifest":
			if i+1 >= len(args) {
				return fmt.Errorf("--manifest requires a value")
			}
			manifestPath = args[i+1]
			i++
		case "--id":
			if i+1 >= len(args) {
				return fmt.Errorf("--id requires a value")
			}
			id = args[i+1]
			i++
		case "--category":
			if i+1 >= len(args) {
				return fmt.Errorf("--category requires a value")
			}
			category = args[i+1]
			i++
		case "--source":
			if i+1 >= len(args) {
				return fmt.Errorf("--source requires a value")
			}
			source = args[i+1]
			i++
		case "--file":
			if i+1 >= len(args) {
				return fmt.Errorf("--file requires a value")
			}
			file = args[i+1]
			i++
		case "--json":
			jsonOut = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown flag %q", arg)
			}
			if source != "" {
				return fmt.Errorf("use only one inline source argument")
			}
			source = arg
		}
	}

	if orgAlias == "" {
		return fmt.Errorf("--target-org is required")
	}
	var snippets []probe.ToolingSnippet
	if manifestPath != "" {
		if source != "" || file != "" {
			return fmt.Errorf("use --manifest or --source/--file, not both")
		}
		loaded, err := probe.ReadToolingSnippetManifest(manifestPath)
		if err != nil {
			return err
		}
		snippets = loaded
	} else {
		snippets = []probe.ToolingSnippet{{ID: id, Category: category, Source: source, File: file}}
	}

	exec := &probe.SFDXExecutor{OrgAlias: orgAlias}
	report, err := exec.CaptureToolingSnippets(probeDir, snippets)
	if err != nil {
		return err
	}
	if outputPath != "" {
		if err := probe.WriteToolingSnippetReport(outputPath, report); err != nil {
			return err
		}
	}
	if jsonOut || outputPath == "" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	fmt.Fprintf(w, "%s: wrote %d Tooling snippet result(s)\n", outputPath, len(report.Snippets))
	return nil
}
