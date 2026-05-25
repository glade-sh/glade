package gladecli

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/dap"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/runartifact"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
	"github.com/glade-sh/glade/internal/watch"
)

func runDev(ctx context.Context, args []string, w io.Writer) (testreport.Run, bool, error) {
	if len(args) > 0 {
		switch args[0] {
		case "test":
			result, err := runDevTest(ctx, args[1:], w)
			return result, true, err
		case "watch":
			result, err := runDevTest(ctx, append(args[1:], "--watch"), w)
			return result, true, err
		case "help", "-h", "--help":
			printDevHelp(w)
			return testreport.Run{}, false, nil
		}
	}
	return testreport.Run{}, false, runDevStatus(args, w)
}

func printDevHelp(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`
Use the human-focused local development cockpit.

Usage:
  glade dev [--project <root>]
  glade dev test [--project <root>] [--class <name>|--test <Class.method>|--changed|--failed] [--out <runs-dir>]
  glade dev watch [--project <root>] [--out <runs-dir>]
`)+"\n")
}

func runDevStatus(args []string, w io.Writer) error {
	root := "."
	for i := 0; i < len(args); i++ {
		switch args[i] {
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
	p, err := project.Load(root)
	if err != nil {
		return err
	}
	index, err := loadIndex(root)
	if err != nil {
		return err
	}
	classes, tests := countDevApexTypes(index)
	metadata := "loaded"
	for _, diag := range index.Diagnostics {
		if diag.Code == "GLADESCHEMA001" {
			metadata = "load error"
			break
		}
	}
	fmt.Fprintf(w, "Project: %s\n", p.Root)
	fmt.Fprintf(w, "Package dirs: %d\n", len(p.PackageDirectories))
	fmt.Fprintf(w, "Apex classes: %d\n", classes)
	fmt.Fprintf(w, "Apex tests: %d\n", tests)
	fmt.Fprintf(w, "Metadata: %s\n", metadata)
	fmt.Fprintf(w, "Last run: %s\n", devLastRun(filepath.Join(p.Root, ".glade", "runs")))
	fmt.Fprint(w, "\nNext:\n")
	fmt.Fprintf(w, "  glade dev test --project %s\n", p.Root)
	fmt.Fprintf(w, "  glade dev watch --project %s\n", p.Root)
	return nil
}

func devLastRun(runsDir string) string {
	latest, err := readLatest(runsDir)
	if err != nil || latest.RunID == "" {
		return "none"
	}
	return latest.RunID
}

func countDevApexTypes(index typesys.Index) (classes int, tests int) {
	for _, typ := range index.Types {
		if typ.Dependency || typ.Kind != apexast.DeclarationClass {
			continue
		}
		classes++
		if typ.IsTest {
			tests++
		}
	}
	return classes, tests
}

func runDevTest(ctx context.Context, args []string, w io.Writer) (testreport.Run, error) {
	root := "."
	outRoot := filepath.Join(".glade", "runs")
	filter := ""
	changed := false
	failed := false
	watchMode := false
	watchOnce := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--out":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--out requires a path")
			}
			outRoot = args[i+1]
			i++
		case "--all":
		case "--class":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--class requires a value")
			}
			filter = args[i+1]
			i++
		case "--test":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--test requires a value")
			}
			filter = args[i+1]
			i++
		case "--changed":
			changed = true
		case "--failed":
			failed = true
		case "--watch":
			watchMode = true
		case "--watch-once":
			watchMode = true
			watchOnce = true
		default:
			return testreport.Run{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if watchMode {
		return runDevWatch(ctx, root, outRoot, watchOnce, w)
	}
	testArgs := []string{"--project", root}
	if filter != "" {
		testArgs = append(testArgs, "--filter", filter)
	}
	if failed {
		failedFilter, err := latestFailedFilter(outRoot)
		if err != nil {
			return testreport.Run{}, err
		}
		if failedFilter == "" {
			fmt.Fprint(w, "No failed tests in latest run.\n")
			return testreport.Run{}, nil
		}
		testArgs = append(testArgs, "--filter", failedFilter)
	}
	if changed {
		testArgs = append(testArgs, "--changed-since", "HEAD")
	}
	result, err := runTest(ctx, testArgs, io.Discard, nil)
	if err != nil {
		return result, err
	}
	run, err := writeDevTestArtifacts(outRoot, root, result, nil)
	if err != nil {
		return result, err
	}
	return result, testreport.WriteConsoleWithOptions(w, result, testreport.ConsoleOptions{ReportPath: run.Path("summary.md")})
}

func latestFailedFilter(outRoot string) (string, error) {
	latest, err := readLatest(outRoot)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(latest.ResultsPath)
	if err != nil {
		return "", err
	}
	var result testreport.Run
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	for _, suite := range result.Suites {
		for _, testCase := range suite.Cases {
			status := testCase.Status
			if status == "" {
				status = testreport.StatusPass
			}
			if status == testreport.StatusPass || status == testreport.StatusSkipped {
				continue
			}
			if testCase.ClassName != "" && testCase.MethodName != "" {
				return testCase.ClassName + "." + testCase.MethodName, nil
			}
			if testCase.ClassName != "" {
				return testCase.ClassName, nil
			}
			if testCase.Name != "" {
				return testCase.Name, nil
			}
			return suite.Name, nil
		}
	}
	return "", nil
}

func writeDevTestArtifacts(outRoot, projectRoot string, result testreport.Run, events []byte) (runartifact.Run, error) {
	run, err := runartifact.Open(outRoot, "", time.Now())
	if err != nil {
		return runartifact.Run{}, err
	}
	summaryPath := run.Path("summary.md")
	resultsPath := run.Path("results.json")
	if err := run.WriteJSON("run.json", map[string]any{
		"project":   projectRoot,
		"runId":     run.ID,
		"createdAt": run.CreatedAt,
	}); err != nil {
		return run, err
	}
	var summary strings.Builder
	if err := testreport.WriteConsole(&summary, result); err != nil {
		return run, err
	}
	if err := run.WriteText("summary.md", summary.String()); err != nil {
		return run, err
	}
	if err := run.WriteJSON("results.json", result); err != nil {
		return run, err
	}
	var junit strings.Builder
	if err := testreport.WriteJUnitXML(&junit, result); err != nil {
		return run, err
	}
	if err := run.WriteText("junit.xml", junit.String()); err != nil {
		return run, err
	}
	if err := run.WriteJSON("selection.json", map[string]any{"project": projectRoot}); err != nil {
		return run, err
	}
	if events == nil {
		events = []byte{}
	}
	if err := run.WriteText("events.ndjson", string(events)); err != nil {
		return run, err
	}
	if err := run.WriteLatest(outRoot, runartifact.Latest{SummaryPath: summaryPath, ResultsPath: resultsPath}); err != nil {
		return run, err
	}
	return run, nil
}

func runDevWatch(ctx context.Context, root, outRoot string, once bool, w io.Writer) (testreport.Run, error) {
	index, err := loadIndex(root)
	if err != nil {
		return testreport.Run{}, err
	}
	fmt.Fprintf(w, "Watching %s.\n", watchDisplayRoot(root))
	fmt.Fprint(w, "Strategy: affected tests\n\n")
	var events bytes.Buffer
	result, err := runWatchTests(ctx, root, index, apextest.Options{}, watch.Config{Root: root}, once, &events)
	if err != nil {
		return result, err
	}
	run, err := writeDevTestArtifacts(outRoot, root, result, events.Bytes())
	if err != nil {
		return result, err
	}
	return result, testreport.WriteConsoleWithOptions(w, result, testreport.ConsoleOptions{ReportPath: run.Path("summary.md")})
}

func watchDisplayRoot(root string) string {
	if strings.TrimSpace(root) == "" || root == "." {
		return "current project"
	}
	return filepath.Base(filepath.Clean(root))
}

func runReport(args []string, w io.Writer) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return errors.New("usage: glade report list|show latest|export latest|clean [--runs-dir <path>]")
	}
	switch args[0] {
	case "list":
		runsDir, _, err := parseReportArgs(args[1:])
		if err != nil {
			return err
		}
		return runReportList(runsDir, w)
	case "show":
		if len(args) < 2 || args[1] != "latest" {
			return errors.New("usage: glade report show latest [--runs-dir <path>]")
		}
		runsDir, _, err := parseReportArgs(args[2:])
		if err != nil {
			return err
		}
		return runReportShowLatest(runsDir, w)
	case "export":
		if len(args) < 2 || args[1] != "latest" {
			return errors.New("usage: glade report export latest --output <path> [--runs-dir <path>]")
		}
		runsDir, output, err := parseReportArgs(args[2:])
		if err != nil {
			return err
		}
		if output == "" {
			return errors.New("--output is required")
		}
		return runReportExportLatest(runsDir, output, w)
	case "clean":
		runsDir, _, keep, err := parseReportCleanArgs(args[1:])
		if err != nil {
			return err
		}
		removed, err := runartifact.Clean(runsDir, keep)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "Removed %d %s.\n", removed, pluralRun(removed))
		return nil
	default:
		return fmt.Errorf("unknown report command %q", args[0])
	}
}

func parseReportArgs(args []string) (runsDir string, output string, err error) {
	runsDir = filepath.Join(".glade", "runs")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--runs-dir":
			if i+1 >= len(args) {
				return "", "", errors.New("--runs-dir requires a path")
			}
			runsDir = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return "", "", errors.New("--output requires a path")
			}
			output = args[i+1]
			i++
		default:
			return "", "", fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return runsDir, output, nil
}

func parseReportCleanArgs(args []string) (runsDir string, output string, keep int, err error) {
	runsDir = filepath.Join(".glade", "runs")
	keep = 10
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--runs-dir":
			if i+1 >= len(args) {
				return "", "", 0, errors.New("--runs-dir requires a path")
			}
			runsDir = args[i+1]
			i++
		case "--keep":
			if i+1 >= len(args) {
				return "", "", 0, errors.New("--keep requires a value")
			}
			parsed, parseErr := strconv.Atoi(args[i+1])
			if parseErr != nil || parsed < 0 {
				return "", "", 0, errors.New("--keep must be a non-negative integer")
			}
			keep = parsed
			i++
		default:
			return "", "", 0, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return runsDir, output, keep, nil
}

func runReportList(runsDir string, w io.Writer) error {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprint(w, "No runs.\n")
			return nil
		}
		return err
	}
	latest, _ := readLatest(runsDir)
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		label := entry.Name()
		if latest.RunID == entry.Name() {
			label += " latest"
		}
		fmt.Fprintln(w, label)
		count++
	}
	if count == 0 {
		fmt.Fprint(w, "No runs.\n")
	}
	return nil
}

func runReportShowLatest(runsDir string, w io.Writer) error {
	latest, err := readLatest(runsDir)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(latest.SummaryPath)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func runReportExportLatest(runsDir, output string, w io.Writer) error {
	latest, err := readLatest(runsDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer file.Close()
	zw := zip.NewWriter(file)
	err = filepath.WalkDir(latest.RunDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(latest.RunDir, path)
		if err != nil {
			return err
		}
		writer, err := zw.Create(rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	})
	if closeErr := zw.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Exported %s\n", output)
	return nil
}

func readLatest(runsDir string) (runartifact.Latest, error) {
	data, err := os.ReadFile(filepath.Join(runsDir, "latest.json"))
	if err != nil {
		return runartifact.Latest{}, err
	}
	var latest runartifact.Latest
	if err := json.Unmarshal(data, &latest); err != nil {
		return runartifact.Latest{}, err
	}
	return latest, nil
}

func pluralRun(n int) string {
	if n == 1 {
		return "run"
	}
	return "runs"
}

func changedSinceSelection(root string, index typesys.Index, ref string) (watch.TestSelection, error) {
	changes, err := watch.GitChangesSince(root, ref)
	if err != nil {
		return watch.TestSelection{}, err
	}
	return watch.SelectAffectedTests(index, changes), nil
}

func filterSelectedTestCases(cases []apextest.TestCase, selection watch.TestSelection) []apextest.TestCase {
	if selection.Mode == watch.SelectionAll {
		return cases
	}
	if selection.Mode == watch.SelectionNone {
		return cases[:0]
	}
	selected := make(map[string]bool, len(selection.TestClasses))
	for _, className := range selection.TestClasses {
		selected[className] = true
	}
	out := cases[:0]
	for _, testCase := range cases {
		if selected[testCase.ClassName] {
			out = append(out, testCase)
		}
	}
	return out
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
	result := testreport.Run{Name: "glade test"}
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

func runWatchTestsDaemon(ctx context.Context, root string, daemon *testdaemon.Daemon, opts apextest.Options, cfg watch.Config, once bool, w io.Writer) (testreport.Run, error) {
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
	result := testreport.Run{Name: "glade test"}
	initialSelection := watch.TestSelection{Mode: watch.SelectionAll, TestClasses: nil, Reason: "initial watch run"}
	activeRunID := runID
	cancelRun, runDone := startDaemonWatchRun(ctx, daemon, opts, initialSelection, runID)
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
			if err := daemon.UpdateChanges(changes); err != nil {
				_ = writeJSONLine(w, watch.NewErrorEvent(time.Now().UTC(), err.Error(), root))
				continue
			}
			selection := daemon.SelectAffected(changes)
			if err := writeJSONLine(w, watch.NewTestsSelectedEvent(time.Now().UTC(), selection)); err != nil {
				return result, err
			}
			if selection.Mode == watch.SelectionNone {
				continue
			}
			cancelRun()
			runID++
			activeRunID = runID
			cancelRun, runDone = startDaemonWatchRun(ctx, daemon, opts, selection, runID)
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

func startDaemonWatchRun(ctx context.Context, daemon *testdaemon.Daemon, opts apextest.Options, selection watch.TestSelection, runID int) (context.CancelFunc, <-chan watchRunResult) {
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan watchRunResult, 1)
	go func() {
		done <- watchRunResult{
			RunID:  runID,
			Result: daemon.RunSelectionContext(runCtx, opts, selection),
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
