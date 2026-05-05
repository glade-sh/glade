package oaercli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/apexdocs"
	"github.com/open-aer/oaer/internal/apextest"
	"github.com/open-aer/oaer/internal/automation"
	"github.com/open-aer/oaer/internal/capability"
	"github.com/open-aer/oaer/internal/compat"
	"github.com/open-aer/oaer/internal/config"
	"github.com/open-aer/oaer/internal/dap"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/lsp"
	"github.com/open-aer/oaer/internal/profile"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/projectscan"
	oaerschema "github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/sema"
	"github.com/open-aer/oaer/internal/server"
	"github.com/open-aer/oaer/internal/sobject"
	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/testreport"
	"github.com/open-aer/oaer/internal/trace"
	"github.com/open-aer/oaer/internal/typesys"
	"github.com/open-aer/oaer/internal/vm"
	"github.com/open-aer/oaer/internal/watch"
)

var Version = "0.0.0-dev"

// Run executes the oaer CLI and returns a process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printHelp(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printHelp(stdout)
		return 0
	case "version", "--version":
		fmt.Fprintf(stdout, "oaer %s\n", Version)
		return 0
	case "doctor":
		if err := runDoctor(ctx, stdout); err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		return 0
	case "parse":
		result, err := runParse(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		if result.HasErrors() {
			return 1
		}
		return 0
	case "inspect":
		index, err := runInspect(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		if index.HasErrors() {
			return 1
		}
		return 0
	case "schema":
		if err := runSchema(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		return 0
	case "check":
		result, err := runCheck(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		if result.HasErrors() {
			return 1
		}
		return 0
	case "exec":
		if err := runExec(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		return 0
	case "test":
		result, err := runTest(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		summary := result.Summary()
		if summary.Failed > 0 || summary.Errors > 0 {
			return 1
		}
		return 0
	case "lsp":
		if err := runLSP(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		return 0
	case "profile":
		if err := runProfile(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		return 0
	case "server":
		if err := runServer(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		return 0
	case "db":
		if err := runDB(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		return 0
	case "compat":
		if err := runCompat(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		return 0
	default:
		report := diagnostic.Report{
			Diagnostics: []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     "OAERCLI001",
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
	fmt.Fprint(w, strings.TrimSpace(`
oaer is a clean-room local Apex runtime.

Usage:
  oaer <command> [flags]

Commands:
  version   Print the oaer version.
  doctor    Print environment and project configuration status.
  parse     Parse Apex source files.
  inspect   Inspect indexed project symbols and unsupported project gaps.
  schema    Load local Salesforce metadata schema.
  check     Run semantic checks over a project.
  exec      Execute anonymous Apex.
  test      Discover and run supported Apex tests.
  lsp       Run the Language Server Protocol server over stdio.
  profile   Analyze oaer trace output.
  server    Start the local Salesforce-compatible API baseline.
  db        Seed, reset, export, and inspect a persistent local database.
  compat    Validate fixtures and report capability readiness.
  help      Print this help text.
`)+"\n")
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

	fmt.Fprintf(w, "oaer: %s\n", Version)
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
		return apexast.Result{}, errors.New("usage: oaer parse <paths...> [--json]")
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
				Code:     "OAERPARSE000",
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
		return typesys.Index{}, errors.New("usage: oaer inspect symbols|gaps [--project <root>] [--json]")
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
		return typesys.Index{}, errors.New("usage: oaer inspect symbols|gaps [--project <root>] [--json]")
	}

	root, jsonOut, err := parseProjectFlags(args[1:])
	if err != nil {
		return typesys.Index{}, err
	}
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, err
	}
	s, err := oaerschema.LoadProject(p)
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
		return errors.New("usage: oaer schema load [--project <root>] [--json]")
	}

	root, jsonOut, err := parseProjectFlags(args[1:])
	if err != nil {
		return err
	}
	p, err := project.Load(root)
	if err != nil {
		return err
	}
	s, err := oaerschema.LoadProject(p)
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
	s, err := oaerschema.LoadProject(p)
	if err != nil {
		return typesys.Index{}, err
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
		return errors.New("usage: oaer exec [--json] [--trace <path>] '<anonymous apex>'")
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
	if limitMode != "" {
		machine.SetLimitMode(limitMode)
	}
	result, err := machine.Execute(program)
	if err != nil {
		return err
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

func runTest(ctx context.Context, args []string, w io.Writer) (testreport.Run, error) {
	if err := ctx.Err(); err != nil {
		return testreport.Run{}, err
	}

	root := "."
	filter := ""
	format := "console"
	junitPath := ""
	limitMode := vm.LimitMode("")
	watchMode := false
	watchOnce := false
	debug := false
	debounce := watch.DefaultDebounce
	backend := watch.BackendAuto
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--filter":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--filter requires a value")
			}
			filter = args[i+1]
			i++
		case "--json":
			format = "json"
		case "--junit":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--junit requires a path")
			}
			junitPath = args[i+1]
			i++
		case "--limit-mode":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--limit-mode requires a value")
			}
			mode, err := parseLimitMode(args[i+1])
			if err != nil {
				return testreport.Run{}, err
			}
			limitMode = mode
			i++
		case "--watch":
			watchMode = true
		case "--watch-once":
			watchMode = true
			watchOnce = true
		case "--debug":
			debug = true
		case "--debounce":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--debounce requires a duration")
			}
			parsed, err := time.ParseDuration(args[i+1])
			if err != nil {
				return testreport.Run{}, err
			}
			debounce = parsed
			i++
		case "--watch-backend":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--watch-backend requires a value")
			}
			parsed, err := parseWatchBackend(args[i+1])
			if err != nil {
				return testreport.Run{}, err
			}
			backend = parsed
			i++
		default:
			return testreport.Run{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}

	index, err := loadIndex(root)
	if err != nil {
		return testreport.Run{}, err
	}
	if watchMode {
		return runWatchTests(ctx, root, index, apextest.Options{Filter: filter, LimitMode: limitMode}, watch.Config{Root: root, Debounce: debounce, Backend: backend}, watchOnce, w)
	}
	result := apextest.Run(index, apextest.Options{Filter: filter, LimitMode: limitMode})
	if debug {
		return result, serveDAPSnapshot(testRunSnapshot(result), w)
	}
	if junitPath != "" {
		if err := writeJUnitFile(junitPath, result); err != nil {
			return result, err
		}
	}
	switch format {
	case "json":
		return result, testreport.WriteJSON(w, result)
	default:
		return result, testreport.WriteConsole(w, result)
	}
}

func testRunSnapshot(result testreport.Run) dap.Snapshot {
	summary := result.Summary()
	vars := map[string]vm.Value{
		"total":       vm.Int(int64(summary.Total)),
		"passed":      vm.Int(int64(summary.Passed)),
		"failed":      vm.Int(int64(summary.Failed)),
		"unsupported": vm.Int(int64(summary.Unsupported)),
	}
	frames := make([]dap.StackFrame, 0)
	id := 1
	for _, suite := range result.Suites {
		for _, testCase := range suite.Cases {
			frames = append(frames, dap.StackFrame{
				ID:     id,
				Name:   testCase.ClassName + "." + testCase.MethodName,
				Line:   1,
				Column: 1,
			})
			id++
		}
	}
	return dap.Snapshot{Frames: frames, Vars: vars}
}

func runWatchTests(ctx context.Context, root string, index typesys.Index, opts apextest.Options, cfg watch.Config, once bool, w io.Writer) (testreport.Run, error) {
	cfg = cfg.Normalized()
	if cfg.Root == "" {
		cfg.Root = root
	}
	previous, err := watch.CaptureSnapshot(root)
	if err != nil {
		return testreport.Run{}, err
	}
	watcher, backend, err := watch.NewBackendWatcher(ctx, cfg, previous)
	if err != nil {
		return testreport.Run{}, err
	}
	defer watcher.Close()
	cfg.Backend = backend
	if err := writeJSONLine(w, watch.NewWatchStartedEvent(time.Now().UTC(), cfg)); err != nil {
		return testreport.Run{}, err
	}
	runID := 1
	result := testreport.Run{Name: "oaer test"}
	initialSelection := watch.TestSelection{Mode: watch.SelectionAll, TestClasses: nil, Reason: "initial watch run"}
	activeRunID := runID
	cancelRun, runDone := startWatchRun(ctx, index, opts, initialSelection, runID)
	defer cancelRun()
	if err := writeJSONLine(w, watch.NewRunStartedEvent(time.Now().UTC(), runID, nil)); err != nil {
		return result, err
	}
	if once {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case finished := <-runDone:
			result = finished.Result
			if err := writeJSONLine(w, watch.NewRunFinishedEvent(time.Now().UTC(), finished.RunID, watchSummary(result))); err != nil {
				return result, err
			}
			return result, nil
		}
	}
	for {
		select {
		case <-ctx.Done():
			cancelRun()
			return result, ctx.Err()
		case finished := <-runDone:
			if finished.RunID != activeRunID {
				continue
			}
			result = finished.Result
			runDone = nil
			if err := writeJSONLine(w, watch.NewRunFinishedEvent(time.Now().UTC(), finished.RunID, watchSummary(result))); err != nil {
				return result, err
			}
		case err, ok := <-watcher.Errors():
			if !ok {
				return result, nil
			}
			_ = writeJSONLine(w, watch.NewErrorEvent(time.Now().UTC(), err.Error(), root))
		case changes, ok := <-watcher.Changes():
			if !ok {
				return result, nil
			}
			if err := writeJSONLine(w, watch.NewChangesEvent(time.Now().UTC(), changes)); err != nil {
				return result, err
			}
			if err := writeJSONLine(w, watch.NewDebouncedEvent(time.Now().UTC(), cfg, changes)); err != nil {
				return result, err
			}
			index, err = updateWatchIndex(root, index, changes)
			if err != nil {
				_ = writeJSONLine(w, watch.NewErrorEvent(time.Now().UTC(), err.Error(), root))
				continue
			}
			selection := watch.SelectAffectedTests(index, changes)
			if err := writeJSONLine(w, watch.NewTestsSelectedEvent(time.Now().UTC(), selection)); err != nil {
				return result, err
			}
			if selection.Mode == watch.SelectionNone {
				continue
			}
			cancelRun()
			runID++
			activeRunID = runID
			cancelRun, runDone = startWatchRun(ctx, index, opts, selection, runID)
			if err := writeJSONLine(w, watch.NewRunStartedEvent(time.Now().UTC(), runID, selection.TestClasses)); err != nil {
				return result, err
			}
		}
	}
}

type watchRunResult struct {
	RunID  int
	Result testreport.Run
}

func startWatchRun(ctx context.Context, index typesys.Index, opts apextest.Options, selection watch.TestSelection, runID int) (context.CancelFunc, <-chan watchRunResult) {
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan watchRunResult, 1)
	go func() {
		done <- watchRunResult{
			RunID:  runID,
			Result: runSelectedTestsContext(runCtx, index, opts, selection),
		}
	}()
	return cancel, done
}

func updateWatchIndex(root string, index typesys.Index, changes []watch.Change) (typesys.Index, error) {
	if !canIncrementalIndex(changes) {
		return loadIndex(root)
	}
	var changed []string
	var deleted []string
	for _, change := range changes {
		switch change.Op {
		case watch.ChangeDeleted:
			deleted = append(deleted, change.Path)
		default:
			changed = append(changed, change.Path)
		}
	}
	return typesys.UpdateApexFiles(index, changed, deleted), nil
}

func canIncrementalIndex(changes []watch.Change) bool {
	if len(changes) == 0 {
		return false
	}
	for _, change := range changes {
		switch change.Kind {
		case watch.FileKindApexClass, watch.FileKindApexTrigger:
		default:
			return false
		}
	}
	return true
}

func parseWatchBackend(value string) (watch.Backend, error) {
	switch watch.Backend(strings.ToLower(strings.TrimSpace(value))) {
	case watch.BackendAuto:
		return watch.BackendAuto, nil
	case watch.BackendNative:
		return watch.BackendNative, nil
	case watch.BackendPoll:
		return watch.BackendPoll, nil
	default:
		return "", fmt.Errorf("unknown watch backend %q (expected auto, native, or poll)", value)
	}
}

func runSelectedTests(index typesys.Index, opts apextest.Options, selection watch.TestSelection) testreport.Run {
	return runSelectedTestsContext(context.Background(), index, opts, selection)
}

func runSelectedTestsContext(ctx context.Context, index typesys.Index, opts apextest.Options, selection watch.TestSelection) testreport.Run {
	if selection.Mode == watch.SelectionDirect && len(selection.TestClasses) == 1 {
		opts.Filter = selection.TestClasses[0]
	}
	return apextest.RunContext(ctx, index, opts)
}

func watchSummary(result testreport.Run) watch.RunSummary {
	s := result.Summary()
	return watch.RunSummary{
		Total:         s.Total,
		Passed:        s.Passed,
		Failed:        s.Failed,
		CompileErrors: s.Errors,
		Unsupported:   s.Unsupported,
		PassedAll:     s.Failed == 0 && s.Errors == 0 && s.Unsupported == 0,
	}
}

func writeJSONLine(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err = io.WriteString(w, "\n")
	return err
}

func writeJUnitFile(path string, result testreport.Run) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return testreport.WriteJUnitXML(file, result)
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
		return errors.New("usage: oaer profile analyze <trace.json> [--json]")
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
		return errors.New("usage: oaer profile analyze <trace.json> [--json]")
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

func runServer(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := "127.0.0.1:8080"
	dbPath := ""
	root := "."
	projectProvided := false
	limitMode := vm.LimitMode("")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--addr":
			if i+1 >= len(args) {
				return errors.New("--addr requires a value")
			}
			addr = args[i+1]
			i++
		case "--db":
			if i+1 >= len(args) {
				return errors.New("--db requires a path")
			}
			dbPath = args[i+1]
			i++
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			projectProvided = true
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
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	var org storage.OrgState
	var handler http.Handler
	if dbPath != "" {
		store, loaded, err := openDBStore(dbPath, root)
		if err != nil {
			return err
		}
		defer store.Close()
		org = loaded
		handler = server.NewWithStore(&org, store)
	} else {
		var err error
		org, err = orgForProject(root)
		if err != nil {
			if projectProvided {
				return err
			}
			org = storageBaseline()
		}
		handler = server.New(&org)
	}
	if limitMode != "" {
		if srv, ok := handler.(*server.Server); ok {
			srv.LimitMode = limitMode
		}
	}
	if srv, ok := handler.(*server.Server); ok {
		if index, err := loadIndex(root); err == nil {
			srv.SetProjectIndex(index)
		} else if projectProvided {
			return err
		}
	}
	fmt.Fprintf(w, "oaer server: %s\n", server.URL(addr))
	return http.ListenAndServe(addr, handler)
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

func runDB(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("usage: oaer db seed|reset|export|inspect --db <path> [--project <root>] [--json] [fixture.json]")
	}
	command := args[0]
	dbPath := ""
	root := "."
	jsonOut := false
	positionals := make([]string, 0)
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--db":
			if i+1 >= len(args) {
				return errors.New("--db requires a path")
			}
			dbPath = args[i+1]
			i++
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--json":
			jsonOut = true
		default:
			positionals = append(positionals, args[i])
		}
	}
	if dbPath == "" {
		return errors.New("oaer db requires --db <path>")
	}
	store, org, err := openDBStore(dbPath, root)
	if err != nil {
		return err
	}
	defer store.Close()
	schemaVersion, err := store.SchemaVersion()
	if err != nil {
		return err
	}
	switch command {
	case "seed":
		if len(positionals) != 1 {
			return errors.New("usage: oaer db seed --db <path> [--project <root>] <fixture.json>")
		}
		file, err := os.Open(positionals[0])
		if err != nil {
			return err
		}
		defer file.Close()
		fixture, err := storage.ReadFixture(file)
		if err != nil {
			return err
		}
		if err := storage.ApplyFixture(&org, fixture); err != nil {
			return err
		}
		storage.EnsureDeterministicPlatformData(&org)
		if err := store.Save(org); err != nil {
			return err
		}
		return writeDBInspect(w, dbPath, org, jsonOut, schemaVersion)
	case "reset":
		if len(positionals) != 0 {
			return fmt.Errorf("unexpected argument %q", positionals[0])
		}
		storage.ResetData(&org)
		if err := store.Save(org); err != nil {
			return err
		}
		return writeDBInspect(w, dbPath, org, jsonOut, schemaVersion)
	case "export":
		if len(positionals) != 0 {
			return fmt.Errorf("unexpected argument %q", positionals[0])
		}
		return storage.WriteFixture(w, storage.FixtureFromOrg(org))
	case "inspect":
		if len(positionals) != 0 {
			return fmt.Errorf("unexpected argument %q", positionals[0])
		}
		return writeDBInspect(w, dbPath, org, jsonOut, schemaVersion)
	default:
		return errors.New("usage: oaer db seed|reset|export|inspect --db <path> [--project <root>] [--json] [fixture.json]")
	}
}

func openDBStore(path, root string) (*storage.SQLiteStore, storage.OrgState, error) {
	store, err := storage.OpenSQLite(path)
	if err != nil {
		return nil, storage.OrgState{}, err
	}
	org, err := store.Load()
	if err != nil {
		_ = store.Close()
		return nil, storage.OrgState{}, err
	}
	if len(org.Objects) == 0 {
		org, err = orgForProject(root)
		if err != nil {
			_ = store.Close()
			return nil, storage.OrgState{}, err
		}
		storage.EnsureDeterministicPlatformData(&org)
		if err := store.Save(org); err != nil {
			_ = store.Close()
			return nil, storage.OrgState{}, err
		}
	}
	return store, org, nil
}

func orgForProject(root string) (storage.OrgState, error) {
	index, err := loadIndex(root)
	if err != nil {
		if root == "." {
			return storageBaseline(), nil
		}
		return storage.OrgState{}, err
	}
	org := storage.NewOrgState()
	org.APIVersion = index.Project.SourceAPIVersion
	org.Namespace = index.Project.Namespace
	registry := sobject.BuildDescribeRegistry(oaerschema.Schema{Objects: append([]oaerschema.Object(nil), index.Objects...)})
	for name, describe := range registry.Objects {
		org.Objects[name] = storage.ObjectState{
			Definition: sobject.ToObjectDefinition(describe),
			Records:    make(map[storage.ID]storage.Record),
		}
	}
	if p, err := project.Load(root); err == nil {
		if automationIndex, err := automation.LoadProject(p); err == nil {
			automation.ApplyToOrg(&org, automationIndex)
		}
	}
	storage.EnsureDeterministicPlatformData(&org)
	return org, nil
}

func writeDBInspect(w io.Writer, path string, org storage.OrgState, jsonOut bool, schemaVersion int) error {
	summary := storage.InspectOrg(path, org)
	summary.SchemaVersion = schemaVersion
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}
	fmt.Fprintf(w, "db: %s\n", path)
	if schemaVersion > 0 {
		fmt.Fprintf(w, "schemaVersion: %d\n", schemaVersion)
	}
	fmt.Fprintf(w, "objects: %d\n", summary.Objects)
	fmt.Fprintf(w, "records: %d\n", summary.Records)
	fmt.Fprintf(w, "users: %d\n", summary.Users)
	fmt.Fprintf(w, "profiles: %d\n", summary.Profiles)
	fmt.Fprintf(w, "permissions: %d\n", summary.Permissions)
	objects := make([]string, 0, len(summary.ByObject))
	for object := range summary.ByObject {
		objects = append(objects, object)
	}
	sort.Strings(objects)
	for _, object := range objects {
		count := summary.ByObject[object]
		fmt.Fprintf(w, "%s: %d\n", object, count)
	}
	return nil
}

func writeTraceFile(path string, events []trace.Event) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return trace.WriteJSON(file, trace.NewDocument(events))
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

func runCompat(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("usage: oaer compat validate|run <fixture.json...> | matrix|mvp [--json] [--require-ready] | post-parity [--project <root>] [--json|--output <path>|--check <path>] [--require-ready] | dashboard|gaps|stdlib [--output <path>|--check <path>] | stdlib --json | docs-inventory --source <dir> [--json|--output <path>|--check <path>|--diff <path>] | catalog --inventory <path> [--json|--output <path>|--check <path>] | product-namespaces --catalog <path> [--json|--output <path>|--check <path>] | evidence --catalog <path> <fixture.json...> [--json]")
	}
	switch args[0] {
	case "matrix", "mvp":
		return runCompatCapabilities(args[1:], w)
	case "post-parity":
		return runCompatPostParity(args[1:], w)
	case "dashboard":
		return runCompatDashboard(args[1:], w)
	case "gaps":
		return runCompatGaps(args[1:], w)
	case "stdlib":
		return runCompatStdlib(args[1:], w)
	case "docs-inventory":
		return runCompatDocsInventory(args[1:], w)
	case "catalog":
		return runCompatCatalog(args[1:], w)
	case "product-namespaces":
		return runCompatProductNamespaces(args[1:], w)
	case "evidence":
		return runCompatEvidence(args[1:], w)
	case "validate", "run":
		if len(args) < 2 {
			return errors.New("usage: oaer compat validate|run <fixture.json...>")
		}
	default:
		return errors.New("usage: oaer compat validate|run <fixture.json...> | matrix|mvp [--json] [--require-ready] | post-parity [--project <root>] [--json|--output <path>|--check <path>] [--require-ready] | dashboard|gaps|stdlib [--output <path>|--check <path>] | stdlib --json | docs-inventory --source <dir> [--json|--output <path>|--check <path>|--diff <path>] | catalog --inventory <path> [--json|--output <path>|--check <path>] | product-namespaces --catalog <path> [--json|--output <path>|--check <path>] | evidence --catalog <path> <fixture.json...> [--json]")
	}

	for _, path := range args[1:] {
		fixture, err := compat.LoadFile(path)
		if err != nil {
			return err
		}
		if err := compat.Validate(fixture); err != nil {
			return err
		}
		if args[0] == "run" {
			result, err := compat.Run(fixture)
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "%s: %s ok=%t\n", path, result.Kind, result.OK)
			continue
		}
		fmt.Fprintf(w, "%s: ok\n", path)
	}
	return nil
}

type postParityReadiness struct {
	Target       string                   `json:"target"`
	Ready        bool                     `json:"ready"`
	Project      string                   `json:"project"`
	Summary      projectscan.Summary      `json:"summary"`
	StageCounts  []postParityCount        `json:"stageCounts"`
	StatusCounts []postParityCount        `json:"statusCounts"`
	Areas        []postParityArea         `json:"areas"`
	Surfaces     []projectscan.Surface    `json:"surfaces"`
	TopBlockers  []projectscan.TopBlocker `json:"topBlockers"`
}

type postParityCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type postParityArea struct {
	Area     string                `json:"area"`
	Surfaces []projectscan.Surface `json:"surfaces"`
}

func runCompatPostParity(args []string, w io.Writer) error {
	root := "."
	jsonOut := false
	requireReady := false
	outputPath := ""
	checkPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--require-ready":
			requireReady = true
		case "--output":
			if i+1 >= len(args) {
				return errors.New("usage: oaer compat post-parity [--project <root>] [--json|--output <path>|--check <path>] [--require-ready]")
			}
			outputPath = args[i+1]
			i++
		case "--check":
			if i+1 >= len(args) {
				return errors.New("usage: oaer compat post-parity [--project <root>] [--json|--output <path>|--check <path>] [--require-ready]")
			}
			checkPath = args[i+1]
			i++
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, or --check")
	}

	report, err := projectscan.Scan(root)
	if err != nil {
		return err
	}
	readiness := postParityReadiness{
		Target:       "legacy-project local test readiness",
		Ready:        report.Summary.TestBlockingFindings == 0,
		Project:      report.Project,
		Summary:      report.Summary,
		StageCounts:  countPostParitySurfaceField(report.Surfaces, func(surface projectscan.Surface) string { return surface.Stage }, nil),
		StatusCounts: countPostParitySurfaceField(report.Surfaces, func(surface projectscan.Surface) string { return surface.Status }, []string{"supported", "partial", "stub", "unsupported", "unknown"}),
		Areas:        groupPostParitySurfacesByArea(report.Surfaces),
		Surfaces:     report.Surfaces,
		TopBlockers:  report.TopBlockers,
	}
	switch {
	case jsonOut:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(readiness); err != nil {
			return err
		}
	case outputPath != "":
		var buf strings.Builder
		if err := writePostParityReadinessMarkdown(&buf, readiness); err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, []byte(buf.String()), 0o644); err != nil {
			return err
		}
	case checkPath != "":
		var buf strings.Builder
		if err := writePostParityReadinessMarkdown(&buf, readiness); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("post-parity readiness drift: run `oaer compat post-parity --project %s --output %s`", root, checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
	default:
		writePostParityReadinessText(w, readiness)
	}
	if requireReady && !readiness.Ready {
		return fmt.Errorf("post-parity readiness gate failed: %d test-blocking findings", readiness.Summary.TestBlockingFindings)
	}
	return nil
}

func countPostParitySurfaceField(surfaces []projectscan.Surface, value func(projectscan.Surface) string, seed []string) []postParityCount {
	counts := map[string]int{}
	for _, name := range seed {
		counts[name] = 0
	}
	for _, surface := range surfaces {
		name := value(surface)
		if name == "" {
			name = "unknown"
		}
		counts[name]++
	}
	seen := map[string]struct{}{}
	names := make([]string, 0, len(counts))
	for _, name := range seed {
		if _, ok := seen[name]; ok {
			continue
		}
		if _, ok := counts[name]; ok {
			names = append(names, name)
			seen[name] = struct{}{}
		}
	}
	extras := make([]string, 0, len(counts))
	for name := range counts {
		if _, ok := seen[name]; ok {
			continue
		}
		extras = append(extras, name)
	}
	sort.Strings(extras)
	names = append(names, extras...)
	out := make([]postParityCount, 0, len(names))
	for _, name := range names {
		out = append(out, postParityCount{Name: name, Count: counts[name]})
	}
	return out
}

func groupPostParitySurfacesByArea(surfaces []projectscan.Surface) []postParityArea {
	grouped := map[string][]projectscan.Surface{}
	for _, surface := range surfaces {
		area := surface.Area
		if area == "" {
			area = "unknown"
		}
		grouped[area] = append(grouped[area], surface)
	}
	areas := make([]string, 0, len(grouped))
	for area := range grouped {
		areas = append(areas, area)
	}
	sort.Strings(areas)
	out := make([]postParityArea, 0, len(areas))
	for _, area := range areas {
		surfaces := grouped[area]
		sort.Slice(surfaces, func(i, j int) bool {
			return surfaces[i].Capability < surfaces[j].Capability
		})
		out = append(out, postParityArea{Area: area, Surfaces: surfaces})
	}
	return out
}

func writePostParityReadinessText(w io.Writer, readiness postParityReadiness) {
	status := "ready"
	if !readiness.Ready {
		status = "not ready"
	}
	fmt.Fprintf(w, "Post-parity readiness: %s\n", status)
	fmt.Fprintf(w, "Target: %s\n", readiness.Target)
	fmt.Fprintf(w, "Project: %s\n", readiness.Project)
	fmt.Fprintf(w, "Files scanned: %d\n", readiness.Summary.FilesScanned)
	fmt.Fprintf(w, "Surfaces: %d\n", readiness.Summary.Surfaces)
	fmt.Fprintf(w, "Findings: %d\n", readiness.Summary.Findings)
	fmt.Fprintf(w, "Test-blocking findings: %d\n", readiness.Summary.TestBlockingFindings)
	writePostParityCountsText(w, "Status counts", readiness.StatusCounts)
	writePostParityCountsText(w, "Stage counts", readiness.StageCounts)
	if len(readiness.TopBlockers) > 0 {
		fmt.Fprintln(w, "Top blockers:")
		for _, blocker := range readiness.TopBlockers {
			fmt.Fprintf(w, "- %s: %d findings across %d files\n", blocker.Capability, blocker.Count, blocker.AffectedFiles)
		}
	}
	if len(readiness.Areas) > 0 {
		fmt.Fprintln(w, "Surfaces by area:")
		for _, area := range readiness.Areas {
			fmt.Fprintf(w, "- %s:\n", area.Area)
			for _, surface := range area.Surfaces {
				fmt.Fprintf(w, "  - %s [%s/%s]: %d findings across %d files; next %s\n", surface.Capability, surface.Stage, surface.Status, surface.Count, surface.AffectedFiles, surface.SuggestedCapability)
			}
		}
	}
}

func writePostParityCountsText(w io.Writer, title string, counts []postParityCount) {
	if len(counts) == 0 {
		return
	}
	fmt.Fprintf(w, "%s:\n", title)
	for _, count := range counts {
		fmt.Fprintf(w, "- %s: %d\n", count.Name, count.Count)
	}
}

func writePostParityReadinessMarkdown(w io.Writer, readiness postParityReadiness) error {
	status := "ready"
	if !readiness.Ready {
		status = "not ready"
	}
	if _, err := fmt.Fprintf(w, "# Post-Parity Readiness\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Post-parity readiness is **%s** for `%s`.\n\n", status, readiness.Project); err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, "This dashboard is separate from the MVP readiness gate. Scanner discovery does not promote a surface to supported without explicit status plumbing and tests.\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, "## Summary\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Metric | Count |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | ---: |"); err != nil {
		return err
	}
	rows := []struct {
		label string
		count int
	}{
		{"Files scanned", readiness.Summary.FilesScanned},
		{"Detected surfaces", readiness.Summary.Surfaces},
		{"Findings", readiness.Summary.Findings},
		{"Test-blocking findings", readiness.Summary.TestBlockingFindings},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(w, "| %s | %d |\n", row.label, row.count); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if err := writePostParityCountsMarkdown(w, "Status Counts", readiness.StatusCounts); err != nil {
		return err
	}
	if err := writePostParityCountsMarkdown(w, "Stage Counts", readiness.StageCounts); err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, "## Top Blockers\n\n"); err != nil {
		return err
	}
	if len(readiness.TopBlockers) == 0 {
		if _, err := fmt.Fprint(w, "No test-blocking post-parity findings were detected.\n\n"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(w, "| Capability | Title | Findings | Files |"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "| --- | --- | ---: | ---: |"); err != nil {
			return err
		}
		for _, blocker := range readiness.TopBlockers {
			if _, err := fmt.Fprintf(w, "| `%s` | %s | %d | %d |\n", blocker.Capability, blocker.Title, blocker.Count, blocker.AffectedFiles); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(w, "## Surfaces By Area\n\n"); err != nil {
		return err
	}
	if len(readiness.Areas) == 0 {
		_, err := fmt.Fprint(w, "No post-parity surfaces were detected.\n\n")
		return err
	}
	for _, area := range readiness.Areas {
		if _, err := fmt.Fprintf(w, "### %s\n\n", area.Area); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "| Capability | Stage | Status | Findings | Files | Suggested next capability |"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "| --- | --- | --- | ---: | ---: | --- |"); err != nil {
			return err
		}
		for _, surface := range area.Surfaces {
			if _, err := fmt.Fprintf(w, "| `%s` | %s | %s | %d | %d | `%s` |\n", surface.Capability, surface.Stage, surface.Status, surface.Count, surface.AffectedFiles, surface.SuggestedCapability); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}

func writePostParityCountsMarkdown(w io.Writer, title string, counts []postParityCount) error {
	if _, err := fmt.Fprintf(w, "## %s\n\n", title); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Name | Count |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | ---: |"); err != nil {
		return err
	}
	for _, count := range counts {
		if _, err := fmt.Fprintf(w, "| %s | %d |\n", count.Name, count.Count); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func runCompatCapabilities(args []string, w io.Writer) error {
	jsonOut := false
	requireReady := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--require-ready":
			requireReady = true
		default:
			return fmt.Errorf("unknown flag %q", arg)
		}
	}
	report := capability.MVPReport()
	if jsonOut {
		if err := capability.WriteJSON(w, report); err != nil {
			return err
		}
	} else if err := capability.WriteText(w, report); err != nil {
		return err
	}
	if requireReady && !report.Ready {
		return fmt.Errorf("MVP readiness gate failed: %d required capabilities incomplete", report.Incomplete)
	}
	return nil
}

func runCompatDashboard(args []string, w io.Writer) error {
	return runCompatGeneratedMarkdown(args, w, "dashboard", "compatibility dashboard", capability.WriteMarkdown)
}

func runCompatGaps(args []string, w io.Writer) error {
	return runCompatGeneratedMarkdown(args, w, "gaps", "known gaps", capability.WriteKnownGapsMarkdown)
}

func runCompatStdlib(args []string, w io.Writer) error {
	jsonOut := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonOut = true
			continue
		}
		filtered = append(filtered, arg)
	}
	if jsonOut {
		if len(filtered) != 0 {
			return errors.New("usage: oaer compat stdlib [--json|--output <path>|--check <path>]")
		}
		return capability.WriteStdlibJSON(w)
	}
	return runCompatStaticMarkdown(filtered, w, "stdlib", "standard library coverage", capability.WriteStdlibMarkdown)
}

func runCompatDocsInventory(args []string, w io.Writer) error {
	source := ""
	outputPath := ""
	checkPath := ""
	diffPath := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source":
			i++
			if i >= len(args) {
				return errors.New("usage: oaer compat docs-inventory --source <dir> [--json|--output <path>|--check <path>|--diff <path>]")
			}
			source = args[i]
		case "--json":
			jsonOut = true
		case "--output":
			i++
			if i >= len(args) {
				return errors.New("usage: oaer compat docs-inventory --source <dir> [--json|--output <path>|--check <path>|--diff <path>]")
			}
			outputPath = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return errors.New("usage: oaer compat docs-inventory --source <dir> [--json|--output <path>|--check <path>|--diff <path>]")
			}
			checkPath = args[i]
		case "--diff":
			i++
			if i >= len(args) {
				return errors.New("usage: oaer compat docs-inventory --source <dir> [--json|--output <path>|--check <path>|--diff <path>]")
			}
			diffPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if source == "" {
		return errors.New("usage: oaer compat docs-inventory --source <dir> [--json|--output <path>|--check <path>|--diff <path>]")
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != "", diffPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, --check, or --diff")
	}

	inv, err := apexdocs.BuildInventory(source)
	if err != nil {
		return err
	}

	switch {
	case jsonOut:
		return apexdocs.WriteJSON(w, inv)
	case outputPath != "":
		var buf strings.Builder
		if err := apexdocs.WriteJSON(&buf, inv); err != nil {
			return err
		}
		return os.WriteFile(outputPath, []byte(buf.String()), 0o644)
	case checkPath != "":
		var buf strings.Builder
		if err := apexdocs.WriteJSON(&buf, inv); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("docs inventory drift: run `oaer compat docs-inventory --source %s --output %s`", source, checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
		return nil
	case diffPath != "":
		oldInv, err := apexdocs.ReadInventory(diffPath)
		if err != nil {
			return err
		}
		diff := apexdocs.DiffInventories(oldInv, inv)
		return apexdocs.WriteDiffJSON(w, diff)
	default:
		writeDocsInventorySummary(w, inv)
		return nil
	}
}

func writeDocsInventorySummary(w io.Writer, inv apexdocs.Inventory) {
	fmt.Fprintf(w, "schemaVersion: %d\n", inv.SchemaVersion)
	fmt.Fprintf(w, "documents: %d\n", inv.TotalFiles)
	fmt.Fprintf(w, "members: %d\n", inv.TotalMembers)
	fmt.Fprintf(w, "namespaces: %d\n", len(inv.Namespaces))
	if len(inv.Namespaces) == 0 {
		return
	}
	fmt.Fprintln(w, "namespace summary:")
	for _, summary := range inv.Namespaces {
		fmt.Fprintf(w, "  %s: documents=%d members=%d", summary.Namespace, summary.Documents, summary.Members)
		if summary.Classes > 0 {
			fmt.Fprintf(w, " classes=%d", summary.Classes)
		}
		if summary.Interfaces > 0 {
			fmt.Fprintf(w, " interfaces=%d", summary.Interfaces)
		}
		if summary.Enums > 0 {
			fmt.Fprintf(w, " enums=%d", summary.Enums)
		}
		if summary.Inputs > 0 {
			fmt.Fprintf(w, " inputs=%d", summary.Inputs)
		}
		if summary.Outputs > 0 {
			fmt.Fprintf(w, " outputs=%d", summary.Outputs)
		}
		fmt.Fprintln(w)
	}
}

func runCompatCatalog(args []string, w io.Writer) error {
	inventoryPath := ""
	outputPath := ""
	checkPath := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--inventory":
			i++
			if i >= len(args) {
				return errors.New("usage: oaer compat catalog --inventory <path> [--json|--output <path>|--check <path>]")
			}
			inventoryPath = args[i]
		case "--json":
			jsonOut = true
		case "--output":
			i++
			if i >= len(args) {
				return errors.New("usage: oaer compat catalog --inventory <path> [--json|--output <path>|--check <path>]")
			}
			outputPath = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return errors.New("usage: oaer compat catalog --inventory <path> [--json|--output <path>|--check <path>]")
			}
			checkPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if inventoryPath == "" {
		return errors.New("usage: oaer compat catalog --inventory <path> [--json|--output <path>|--check <path>]")
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, or --check")
	}
	inv, err := apexdocs.ReadInventory(inventoryPath)
	if err != nil {
		return err
	}
	catalog := capability.BuildCatalog(inv)
	switch {
	case jsonOut:
		return capability.WriteCatalogJSON(w, catalog)
	case outputPath != "":
		var buf strings.Builder
		if err := capability.WriteCatalogJSON(&buf, catalog); err != nil {
			return err
		}
		return os.WriteFile(outputPath, []byte(buf.String()), 0o644)
	case checkPath != "":
		var buf strings.Builder
		if err := capability.WriteCatalogJSON(&buf, catalog); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("capability catalog drift: run `oaer compat catalog --inventory %s --output %s`", inventoryPath, checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
		return nil
	default:
		writeCatalogSummary(w, catalog)
		return nil
	}
}

func writeCatalogSummary(w io.Writer, catalog capability.Catalog) {
	fmt.Fprintf(w, "schemaVersion: %d\n", catalog.SchemaVersion)
	fmt.Fprintf(w, "sourceDocuments: %d\n", catalog.SourceDocuments)
	fmt.Fprintf(w, "sourceMembers: %d\n", catalog.SourceMembers)
	fmt.Fprintf(w, "entries: %d\n", len(catalog.Entries))
	if len(catalog.Summary) == 0 {
		return
	}
	fmt.Fprintln(w, "summary:")
	for _, summary := range catalog.Summary {
		fmt.Fprintf(w, "  %s [%s/%s]: entries=%d documents=%d members=%d\n", summary.Area, summary.Target, summary.Status, summary.Entries, summary.Documents, summary.Members)
	}
}

func runCompatProductNamespaces(args []string, w io.Writer) error {
	catalogPath := ""
	outputPath := ""
	checkPath := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--catalog":
			i++
			if i >= len(args) {
				return errors.New("usage: oaer compat product-namespaces --catalog <path> [--json|--output <path>|--check <path>]")
			}
			catalogPath = args[i]
		case "--json":
			jsonOut = true
		case "--output":
			i++
			if i >= len(args) {
				return errors.New("usage: oaer compat product-namespaces --catalog <path> [--json|--output <path>|--check <path>]")
			}
			outputPath = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return errors.New("usage: oaer compat product-namespaces --catalog <path> [--json|--output <path>|--check <path>]")
			}
			checkPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if catalogPath == "" {
		return errors.New("usage: oaer compat product-namespaces --catalog <path> [--json|--output <path>|--check <path>]")
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, or --check")
	}
	catalog, err := capability.ReadCatalog(catalogPath)
	if err != nil {
		return err
	}
	report := capability.BuildProductNamespaceReport(catalog)
	switch {
	case jsonOut:
		return capability.WriteProductNamespaceJSON(w, report)
	case outputPath != "":
		var buf strings.Builder
		if err := capability.WriteProductNamespaceJSON(&buf, report); err != nil {
			return err
		}
		return os.WriteFile(outputPath, []byte(buf.String()), 0o644)
	case checkPath != "":
		var buf strings.Builder
		if err := capability.WriteProductNamespaceJSON(&buf, report); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("product namespace report drift: run `oaer compat product-namespaces --catalog %s --output %s`", catalogPath, checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
		return nil
	default:
		return capability.WriteProductNamespaceText(w, report)
	}
}

func runCompatEvidence(args []string, w io.Writer) error {
	catalogPath := ""
	jsonOut := false
	fixturePaths := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--catalog":
			i++
			if i >= len(args) {
				return errors.New("usage: oaer compat evidence --catalog <path> <fixture.json...> [--json]")
			}
			catalogPath = args[i]
		case "--json":
			jsonOut = true
		default:
			fixturePaths = append(fixturePaths, args[i])
		}
	}
	if catalogPath == "" || len(fixturePaths) == 0 {
		return errors.New("usage: oaer compat evidence --catalog <path> <fixture.json...> [--json]")
	}
	catalog, err := capability.ReadCatalog(catalogPath)
	if err != nil {
		return err
	}
	fixtures := make([]compat.Fixture, 0, len(fixturePaths))
	for _, path := range fixturePaths {
		fixture, err := compat.LoadFile(path)
		if err != nil {
			return err
		}
		if err := compat.Validate(fixture); err != nil {
			return err
		}
		fixtures = append(fixtures, fixture)
	}
	report := compat.BuildEvidenceReport(catalog, fixtures)
	if jsonOut {
		return compat.WriteEvidenceJSON(w, report)
	}
	writeEvidenceSummary(w, report)
	return nil
}

func writeEvidenceSummary(w io.Writer, report compat.EvidenceReport) {
	fmt.Fprintf(w, "catalogEntries: %d\n", report.CatalogEntries)
	fmt.Fprintf(w, "fixtures: %d\n", report.Fixtures)
	fmt.Fprintf(w, "evidence: %d\n", report.Evidence)
	fmt.Fprintf(w, "covered: %d\n", len(report.Covered))
	fmt.Fprintf(w, "unmatchedEvidence: %d\n", len(report.UnmatchedEvidence))
	fmt.Fprintf(w, "ungatedPromoted: %d\n", len(report.UngatedPromoted))
	if len(report.Summary) == 0 {
		return
	}
	fmt.Fprintln(w, "summary:")
	for _, summary := range report.Summary {
		fmt.Fprintf(w, "  %s [%s/%s]: covered=%d entries=%d", summary.Area, summary.Target, summary.Status, summary.Covered, summary.Entries)
		if summary.Ungated > 0 {
			fmt.Fprintf(w, " ungated=%d", summary.Ungated)
		}
		fmt.Fprintln(w)
	}
}

func runCompatGeneratedMarkdown(args []string, w io.Writer, command, label string, write func(io.Writer, capability.Report) error) error {
	return runCompatStaticMarkdown(args, w, command, label, func(w io.Writer) error {
		return write(w, capability.MVPReport())
	})
}

func runCompatStaticMarkdown(args []string, w io.Writer, command, label string, write func(io.Writer) error) error {
	outputPath := ""
	checkPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--output":
			i++
			if i >= len(args) {
				return fmt.Errorf("usage: oaer compat %s [--output <path>|--check <path>]", command)
			}
			outputPath = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return fmt.Errorf("usage: oaer compat %s [--output <path>|--check <path>]", command)
			}
			checkPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if outputPath != "" && checkPath != "" {
		return errors.New("use only one of --output or --check")
	}

	var buf strings.Builder
	if err := write(&buf); err != nil {
		return err
	}
	content := buf.String()

	switch {
	case outputPath != "":
		return os.WriteFile(outputPath, []byte(content), 0o644)
	case checkPath != "":
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != content {
			return fmt.Errorf("%s drift: run `oaer compat %s --output %s`", label, command, checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
		return nil
	default:
		_, err := io.WriteString(w, content)
		return err
	}
}
