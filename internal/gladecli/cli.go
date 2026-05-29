package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexlog"
	"github.com/glade-sh/glade/internal/config"
	"github.com/glade-sh/glade/internal/dap"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/lsp"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/profile"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/projectscan"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

var Version = "0.0.0-dev"

// Run executes the glade CLI and returns a process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printHelp(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		if len(args) > 1 {
			printHelpTopic(stdout, args[1:])
			return 0
		}
		printHelp(stdout)
		return 0
	case "version", "--version":
		fmt.Fprintf(stdout, "glade %s\n", Version)
		return 0
	case "doctor":
		if err := runDoctor(ctx, stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "parse":
		result, err := runParse(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		if result.HasErrors() {
			return 1
		}
		return 0
	case "inspect":
		index, err := runInspect(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		if index.HasErrors() {
			return 1
		}
		return 0
	case "schema":
		if err := runSchema(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "check":
		result, err := runCheck(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		if result.HasErrors() {
			return 1
		}
		return 0
	case "exec":
		if err := runExec(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "test":
		result, err := runTest(ctx, args[1:], stdout, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		summary := result.Summary()
		if summary.Failed > 0 || summary.Errors > 0 {
			return 1
		}
		return 0
	case "dev":
		result, ranTests, err := runDev(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		if ranTests {
			summary := result.Summary()
			if summary.Failed > 0 || summary.Errors > 0 {
				return 1
			}
		}
		return 0
	case "report":
		if err := runReport(args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "lsp":
		if err := runLSP(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "profile":
		if err := runProfile(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "package":
		if err := runPackage(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "server":
		if err := runServer(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "playground":
		if err := runPlayground(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "db":
		if err := runDB(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "compat":
		if err := runCompat(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "probe":
		if err := runProbe(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	default:
		report := diagnostic.Report{
			Diagnostics: []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     "GLADECLI001",
				Message:  fmt.Sprintf("unknown command %q", args[0]),
			}},
		}
		_ = report.WriteText(stderr)
		fmt.Fprintln(stderr)
		printHelp(stderr)
		return 2
	}
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, `glade is a clean-room local Apex runtime.

Usage:
  glade <command> [flags]

Commands:
  version        Print the glade version.
  doctor         Print environment and project configuration status.
  parse          Parse Apex source files.
  inspect        Inspect indexed project symbols and unsupported project gaps.
  schema         Load local Salesforce metadata schema.
  check          Run semantic checks over a project.
  exec           Execute anonymous Apex.
  test           Discover and run supported Apex tests.
  dev            Run the human-focused local development cockpit.
  report         List, show, export, and clean saved run reports.
  lsp            Run the Language Server Protocol server over stdio.
  profile        Analyze glade trace output.
  package        Build managed package artifacts.
  server         Start the local Salesforce-compatible API baseline.
  playground     Start the local Apex playground web UI.
  db             Seed, reset, export, and inspect a persistent local database.
  compat         Validate fixtures and report capability readiness.
  probe          Run org probes to discover gaps against a real Salesforce org.
  help           Print this help text.

Compat subcommands:
  validate           Validate compatibility fixture files.
  run                Validate and execute fixtures.
  matrix             Print the full capability matrix.
  mvp                Print MVP readiness report.
  local-tests        Report local Apex test execution readiness.
  oracle             Run deep Salesforce oracle workflows.
  examples           Scan example projects and report support status.
  post-parity        Scan a project for unsupported surfaces.
  dashboard          Generate compatibility dashboard.
  gaps               Generate known gaps document.
  stdlib             Generate standard library coverage document.
  stub-contracts     Report generated stub behavioral contract policy.
  stub-discovery     Execute generated stub probes and report implementation candidates.
  stub-behavior      Report generated platform stub behavior status.
  tooling-fixtures   Validate captured Tooling snippet oracle reports.
`)
}

func printHelpTopic(w io.Writer, args []string) {
	if len(args) == 0 {
		printHelp(w)
		return
	}
	switch args[0] {
	case "test":
		printTestHelp(w)
	case "compat":
		if len(args) > 1 && args[1] == "local-tests" {
			printCompatLocalTestsHelp(w)
			return
		}
		printCompatHelp(w)
	default:
		printHelp(w)
	}
}

func printTestHelp(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`
Run local Apex tests.

Usage:
  glade test [--project <root>] [--filter <pattern>] [--json] [--junit <path>] [--watch|--watch-once]

Common flags:
  --project <root>          Project root. Defaults to current directory.
  --filter <pattern>        Run matching test classes or methods.
  --json                    Write JSON test results.
  --junit <path>            Write JUnit XML results.
  --progress                Print bounded progress to stderr.
  --watch                   Watch source files and emit NDJSON events.
  --watch-once              Run one watch cycle and exit.
  --changed-since <ref>     Select tests affected since a git ref.
  --parallel-methods        Run test methods in parallel (default).
  --no-parallel-methods     Force serial method execution within a class.
  --parallelism <n>         Worker count (default: GOMAXPROCS).
  --test-timeout <dur>      Per-test timeout (default 5m, e.g. 30s, 2m).
  --gc-aggressive           Run with GOGC=50 for memory-constrained hosts.
  --limit-mode <mode>       Use strict or permissive governor limits.

Examples:
  glade test --project force-app
  glade test --project . --filter AccountServiceTest
  glade test --project . --watch
`)+"\n")
}

func printCompatHelp(w io.Writer) {
	fmt.Fprintf(w, "%s\n\nExamples:\n  glade compat mvp --json\n  glade compat local-tests --project . --json\n", compatUsage())
}

func printCompatLocalTestsHelp(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`
Report local Apex test execution readiness.

Usage:
  glade compat local-tests [--project <root>] [--class <name>] [--class-list <a,b>] [--class-file <path>] [--method <name>] [--json]

Common flags:
  --project <root>          Project root. Defaults to current directory.
  --class <name>            Run one Apex test class.
  --class-list <a,b>        Run a comma-separated list of test classes.
  --class-file <path>       Run classes listed in a file.
  --start-class <name>      Resume from a class name in sorted order.
  --method <name>           Run one test method when paired with --class.
  --changed-since <ref>     Select tests affected since a git ref.
  --blockers-only           Print only blocking failures.
  --top-failures <n>        Limit failure groups in human output.
  --timeout <ms-per-test>   Per-test timeout in milliseconds.
  --parallel <n|auto>       Run test classes with n workers.
  --shard-count <n|auto>    Select one shard from a balanced class plan.
  --shard-index <i|auto>    Shard index to execute.
  --duration-history <path> Optional perf JSON used to weight class sharding.
  --progress                Print progress while running.
  --json                    Write JSON readiness results.
  --check <path>            Compare results with a checked baseline.

Examples:
  glade compat local-tests --project . --json
  glade compat local-tests --project . --class AccountServiceTest
  glade compat local-tests --project . --class-file tests.txt --top-failures 10
`)+"\n")
}

func isHelpArg(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}

func runPackage(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 || args[0] != "build" {
		return errors.New("usage: glade package build --project <root> --namespace <namespace> --output <artifact> [--version <version>] [--json]")
	}

	root := "."
	namespace := ""
	version := ""
	output := ""
	jsonOut := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--namespace":
			if i+1 >= len(args) {
				return errors.New("--namespace requires a value")
			}
			namespace = strings.TrimSpace(args[i+1])
			i++
		case "--version":
			if i+1 >= len(args) {
				return errors.New("--version requires a value")
			}
			version = strings.TrimSpace(args[i+1])
			i++
		case "--output":
			if i+1 >= len(args) {
				return errors.New("--output requires a value")
			}
			output = args[i+1]
			i++
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if namespace == "" {
		return errors.New("--namespace is required")
	}
	if output == "" {
		return errors.New("--output is required")
	}

	p, err := project.Load(root)
	if err != nil {
		return err
	}
	p.Namespace = namespace
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		return err
	}
	idx := typesys.Build(p, s)
	artifact, err := packageartifact.Build(namespace, version, p, s, packageArtifactTypes(idx.Types))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	if err := packageartifact.WriteJSON(output, artifact); err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(artifact)
	}
	fmt.Fprintf(w, "package artifact: %s\n", output)
	fmt.Fprintf(w, "namespace: %s\n", artifact.Namespace)
	if artifact.Version != "" {
		fmt.Fprintf(w, "version: %s\n", artifact.Version)
	}
	fmt.Fprintf(w, "apexTypes: %d\n", len(artifact.ApexTypes))
	fmt.Fprintf(w, "objects: %d\n", len(artifact.Objects))
	fmt.Fprintf(w, "sourceHash: %s\n", artifact.SourceHash)
	return nil
}

func packageArtifactTypes(types []typesys.TypeSymbol) []packageartifact.ApexType {
	out := make([]packageartifact.ApexType, 0, len(types))
	for _, typ := range types {
		out = append(out, packageartifact.ApexType{
			Kind:       typ.Kind,
			Name:       typ.Name,
			File:       typ.File,
			Namespace:  typ.Namespace,
			SourceRoot: typ.SourceRoot,
			Version:    typ.Version,
			Dependency: typ.Dependency,
			Modifiers:  append([]string(nil), typ.Modifiers...),
			IsTest:     typ.IsTest,
			SuperClass: typ.SuperClass,
			Interfaces: append([]string(nil), typ.Interfaces...),
			Range:      typ.Range,
			Members:    packageArtifactMembers(typ.Members),
		})
	}
	return out
}

func packageArtifactMembers(members []typesys.MemberSymbol) []packageartifact.ApexMember {
	out := make([]packageartifact.ApexMember, 0, len(members))
	for _, member := range members {
		out = append(out, packageartifact.ApexMember{
			Kind:       member.Kind,
			Name:       member.Name,
			Type:       member.Type,
			Modifiers:  append([]string(nil), member.Modifiers...),
			Parameters: append([]apexast.Parameter(nil), member.Parameters...),
			Accessors:  append([]apexast.Accessor(nil), member.Accessors...),
			IsTest:     member.IsTest,
			Range:      member.Range,
		})
	}
	return out
}

func runDoctor(ctx context.Context, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, cfgPath, err := config.LoadNearest(cwd)
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return err
	}

	fmt.Fprintf(w, "glade: %s\n", Version)
	fmt.Fprintf(w, "go: %s\n", runtime.Version())
	fmt.Fprintf(w, "os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(w, "cwd: %s\n", cwd)

	if errors.Is(err, config.ErrNotFound) {
		fmt.Fprintln(w, "config: not found")
	} else {
		fmt.Fprintf(w, "config: %s\n", cfgPath)
		if cfg.Project.Root != "" {
			fmt.Fprintf(w, "project.root: %s\n", cfg.Project.Root)
		}
		if cfg.Project.DefaultNamespace != "" {
			fmt.Fprintf(w, "project.defaultNamespace: %s\n", cfg.Project.DefaultNamespace)
		}
	}

	fmt.Fprintln(w, "status: ok")
	return nil
}

func runParse(ctx context.Context, args []string, w io.Writer) (apexast.Result, error) {
	if err := ctx.Err(); err != nil {
		return apexast.Result{}, err
	}

	jsonOut := false
	paths := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) == 0 {
		return apexast.Result{}, errors.New("usage: glade parse <paths...> [--json]")
	}

	files, err := expandApexPaths(paths)
	if err != nil {
		return apexast.Result{}, err
	}

	parser := apexast.NewParser()
	result := apexast.Result{Files: make([]apexast.File, 0, len(files))}
	for _, path := range files {
		file, err := parser.ParseFile(path)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADEPARSE000",
				Message:  err.Error(),
				File:     path,
			})
			continue
		}
		result.Files = append(result.Files, file)
	}

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return result, enc.Encode(result)
	}

	for _, file := range result.Files {
		if len(file.Diagnostics) > 0 {
			_ = diagnostic.Report{Diagnostics: file.Diagnostics}.WriteText(w)
			fmt.Fprintln(w)
			continue
		}
		fmt.Fprintf(w, "%s: %s", file.Path, file.Kind)
		if len(file.Declarations) > 0 {
			fmt.Fprintf(w, " %s", file.Declarations[0].Name)
		}
		fmt.Fprintln(w)
	}
	if len(result.Diagnostics) > 0 {
		_ = diagnostic.Report{Diagnostics: result.Diagnostics}.WriteText(w)
		fmt.Fprintln(w)
	}
	return result, nil
}

func runInspect(ctx context.Context, args []string, w io.Writer) (typesys.Index, error) {
	if err := ctx.Err(); err != nil {
		return typesys.Index{}, err
	}
	if len(args) == 0 {
		return typesys.Index{}, errors.New("usage: glade inspect symbols|gaps [--project <root>] [--json]")
	}
	if args[0] == "gaps" || args[0] == "post-parity" {
		root, jsonOut, err := parseProjectFlags(args[1:])
		if err != nil {
			return typesys.Index{}, err
		}
		report, err := projectscan.Scan(root)
		if err != nil {
			return typesys.Index{}, err
		}
		if jsonOut {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			return typesys.Index{}, enc.Encode(report)
		}
		writeProjectGapInspectText(w, report)
		return typesys.Index{}, nil
	}
	if args[0] != "symbols" {
		return typesys.Index{}, errors.New("usage: glade inspect symbols|gaps [--project <root>] [--json]")
	}

	root, jsonOut, err := parseProjectFlags(args[1:])
	if err != nil {
		return typesys.Index{}, err
	}
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, err
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		return typesys.Index{}, err
	}
	index := typesys.Build(p, s)

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return index, enc.Encode(index)
	}

	fmt.Fprintf(w, "project: %s\n", index.Project.Root)
	if index.Project.Namespace != "" {
		fmt.Fprintf(w, "namespace: %s\n", index.Project.Namespace)
	}
	fmt.Fprintf(w, "types: %d\n", len(index.Types))
	fmt.Fprintf(w, "triggers: %d\n", len(index.Triggers))
	fmt.Fprintf(w, "objects: %d\n", len(index.Objects))
	for _, typ := range index.Types {
		fmt.Fprintf(w, "%s %s %s\n", typ.Kind, typ.Name, typ.File)
		for _, member := range typ.Members {
			fmt.Fprintf(w, "  %s %s", member.Kind, member.Name)
			if member.Type != "" {
				fmt.Fprintf(w, " %s", member.Type)
			}
			if member.IsTest {
				fmt.Fprint(w, " @isTest")
			}
			fmt.Fprintln(w)
		}
	}
	for _, trigger := range index.Triggers {
		fmt.Fprintf(w, "trigger %s on %s %s\n", trigger.Name, trigger.ObjectName, trigger.File)
	}
	for _, object := range index.Objects {
		fmt.Fprintf(w, "sobject %s fields=%d\n", object.Name, len(object.Fields))
		for _, field := range object.Fields {
			fmt.Fprintf(w, "  field %s %s\n", field.Name, field.Type)
		}
	}
	if len(index.Diagnostics) > 0 {
		_ = diagnostic.Report{Diagnostics: index.Diagnostics}.WriteText(w)
		fmt.Fprintln(w)
	}
	return index, nil
}

func writeProjectGapInspectText(w io.Writer, report projectscan.Report) {
	fmt.Fprintf(w, "project: %s\n", report.Project)
	fmt.Fprintf(w, "filesScanned: %d\n", report.Summary.FilesScanned)
	fmt.Fprintf(w, "surfaces: %d\n", report.Summary.Surfaces)
	fmt.Fprintf(w, "findings: %d\n", report.Summary.Findings)
	fmt.Fprintf(w, "testBlockingFindings: %d\n", report.Summary.TestBlockingFindings)
	if len(report.TopBlockers) > 0 {
		fmt.Fprintln(w, "topBlockers:")
		for _, blocker := range report.TopBlockers {
			fmt.Fprintf(w, "  %s: %d findings across %d files\n", blocker.Capability, blocker.Count, blocker.AffectedFiles)
		}
	}
	if len(report.Surfaces) > 0 {
		fmt.Fprintln(w, "surfaces:")
		for _, surface := range report.Surfaces {
			fmt.Fprintf(w, "  %s [%s/%s]: %d findings across %d files\n", surface.Capability, surface.Stage, surface.Status, surface.Count, surface.AffectedFiles)
			for _, example := range surface.Examples {
				if example.Line > 0 {
					fmt.Fprintf(w, "    - %s:%d", example.File, example.Line)
				} else {
					fmt.Fprintf(w, "    - %s", example.File)
				}
				if example.Symbol != "" {
					fmt.Fprintf(w, " %s", example.Symbol)
				}
				fmt.Fprintln(w)
			}
		}
	}
}

func runSchema(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 || args[0] != "load" {
		return errors.New("usage: glade schema load [--project <root>] [--json]")
	}

	root, jsonOut, err := parseProjectFlags(args[1:])
	if err != nil {
		return err
	}
	p, err := project.Load(root)
	if err != nil {
		return err
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(s)
	}

	fmt.Fprintf(w, "objects: %d\n", len(s.Objects))
	for _, object := range s.Objects {
		fmt.Fprintf(w, "%s fields=%d\n", object.Name, len(object.Fields))
	}
	return nil
}

func runCheck(ctx context.Context, args []string, w io.Writer) (sema.Result, error) {
	if err := ctx.Err(); err != nil {
		return sema.Result{}, err
	}

	root, jsonOut, err := parseProjectFlags(args)
	if err != nil {
		return sema.Result{}, err
	}
	index, err := loadIndex(root)
	if err != nil {
		return sema.Result{}, err
	}
	result := sema.Analyze(index)

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return result, enc.Encode(result)
	}

	fmt.Fprintf(w, "project: %s\n", result.Project.Root)
	fmt.Fprintf(w, "types: %d\n", result.Summary.Types)
	fmt.Fprintf(w, "triggers: %d\n", result.Summary.Triggers)
	fmt.Fprintf(w, "objects: %d\n", result.Summary.Objects)
	fmt.Fprintf(w, "diagnostics: %d\n", result.Summary.Diagnostics)
	if len(result.Diagnostics) > 0 {
		_ = diagnostic.Report{Diagnostics: result.Diagnostics}.WriteText(w)
		fmt.Fprintln(w)
	}
	return result, nil
}

func loadIndex(root string) (typesys.Index, error) {
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, err
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		index := typesys.Build(p, gladeschema.Schema{})
		index.Diagnostics = append(index.Diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESCHEMA001",
			Message:  fmt.Sprintf("metadata schema load failed: %v", err),
		})
		return index, nil
	}
	return typesys.Build(p, s), nil
}

func runExec(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	jsonOut := false
	debug := false
	tracePath := ""
	debugLogPath := ""
	limitMode := vm.LimitMode("")
	sourceParts := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			jsonOut = true
		case "--debug":
			debug = true
		case "--trace":
			if i+1 >= len(args) {
				return errors.New("--trace requires a path")
			}
			tracePath = args[i+1]
			i++
		case "--debug-log":
			if i+1 >= len(args) {
				return errors.New("--debug-log requires a path (use - for stdout)")
			}
			debugLogPath = args[i+1]
			i++
		case "--limit-mode":
			if i+1 >= len(args) {
				return errors.New("--limit-mode requires a value")
			}
			mode, err := parseLimitMode(args[i+1])
			if err != nil {
				return err
			}
			limitMode = mode
			i++
		default:
			sourceParts = append(sourceParts, arg)
		}
	}
	if len(sourceParts) == 0 {
		return errors.New("usage: glade exec [--json] [--trace <path>] [--debug-log <path>] '<anonymous apex>'")
	}

	program, err := vm.CompileAnonymous(strings.Join(sourceParts, " "))
	if err != nil {
		return err
	}

	stdout := w
	if jsonOut {
		stdout = nil
	}
	machine := vm.New(stdout)
	machine.SetTraceEnabled(tracePath != "" || debug || jsonOut || debugLogPath != "")
	if limitMode != "" {
		machine.SetLimitMode(limitMode)
	}
	result, execErr := machine.Execute(program)
	if debugLogPath != "" {
		log := apexlog.Format(&result, execErr, apexlog.Options{})
		if err := writeDebugLog(debugLogPath, log, w); err != nil {
			return err
		}
	}
	if execErr != nil {
		return execErr
	}
	if tracePath != "" {
		if err := writeTraceFile(tracePath, result.Trace); err != nil {
			return err
		}
	}
	if debug {
		return serveDAPSnapshot(dap.NewSnapshot(result.Trace, result.Vars), w)
	}

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	return nil
}

func serveDAPSnapshot(snapshot dap.Snapshot, w io.Writer) error {
	if file, ok := w.(*os.File); ok && file.Fd() == os.Stdout.Fd() {
		return dap.Serve(os.Stdin, w, dap.NewHandler(snapshot))
	}
	return dap.Write(w, dap.NewHandler(snapshot).Handle(dap.Request{Seq: 1, Type: dap.MessageTypeRequest, Command: dap.CommandInitialize})[0])
}

func parseLimitMode(raw string) (vm.LimitMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "permissive":
		return vm.LimitModePermissive, nil
	case "strict":
		return vm.LimitModeStrict, nil
	default:
		return "", fmt.Errorf("unsupported limit mode %q", raw)
	}
}

func runLSP(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := "."
	diagnosticsOnce := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--diagnostics-once":
			diagnosticsOnce = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	index, err := loadIndex(root)
	if err != nil {
		return err
	}
	handler := lsp.NewHandler(index)
	if diagnosticsOnce {
		for _, notification := range handler.PublishDiagnostics(sema.Analyze(index).Diagnostics) {
			if err := lsp.WriteMessage(w, notification); err != nil {
				return err
			}
		}
		return nil
	}
	return lsp.Serve(os.Stdin, w, handler)
}

func runProfile(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 || args[0] != "analyze" {
		return errors.New("usage: glade profile analyze <trace.json> [--json]")
	}
	jsonOut := false
	tracePath := ""
	for _, arg := range args[1:] {
		switch arg {
		case "--json":
			jsonOut = true
		default:
			if tracePath != "" {
				return fmt.Errorf("unexpected argument %q", arg)
			}
			tracePath = arg
		}
	}
	if tracePath == "" {
		return errors.New("usage: glade profile analyze <trace.json> [--json]")
	}
	file, err := os.Open(tracePath)
	if err != nil {
		return err
	}
	defer file.Close()
	doc, err := profile.ReadTrace(file)
	if err != nil {
		return err
	}
	report := profile.Analyze(doc)
	if jsonOut {
		return profile.WriteJSON(w, report)
	}
	return profile.WriteMarkdown(w, report)
}

func storageBaseline() storage.OrgState {
	org := storage.NewOrgState()
	org.APIVersion = "61.0"
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	storage.EnsureDeterministicPlatformData(&org)
	return org
}

func parseProjectFlags(args []string) (root string, jsonOut bool, err error) {
	root = "."
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--project":
			if i+1 >= len(args) {
				return "", false, errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		default:
			return "", false, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return root, jsonOut, nil
}

func expandApexPaths(paths []string) ([]string, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, path)
			continue
		}
		err = filepath.WalkDir(path, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".cls", ".trigger":
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}
