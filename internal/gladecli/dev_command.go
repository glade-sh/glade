package gladecli

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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

func runDev(ctx context.Context, args []string, w io.Writer, progressW io.Writer) (testreport.Run, bool, error) {
	if len(args) > 0 {
		switch args[0] {
		case "test":
			result, err := runDevTest(ctx, args[1:], w, progressW)
			return result, true, err
		case "watch":
			result, err := runDevTest(ctx, append(args[1:], "--watch"), w, progressW)
			return result, true, err
		case "vf":
			return testreport.Run{}, false, runDevVF(ctx, args[1:], w, progressW)
		case "lwc":
			return testreport.Run{}, false, runDevLWC(ctx, args[1:], w, progressW)
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
  glade dev vf [--project <root>] [--port <port>|--addr <host:port>] [--ready-file <path>]
  glade dev lwc [--project <root>] [--db <path>] [--port <port>|--addr <host:port>] [--ready-file <path>]

Preview features:
  Visualforce local rendering and the LWC local shell are useful local previews.
  They do not promise full hosted Salesforce parity.
`)+"\n")
}

func runDevStatus(args []string, w io.Writer) error {
	root := "."
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return err
			}
			root = value
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
	fmt.Fprintln(w, "Glade dev")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Local feedback loop")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Project       %s\n", devDisplayRoot(root, p.Root))
	fmt.Fprintf(w, "Package dirs  %d\n", len(p.PackageDirectories))
	fmt.Fprintf(w, "Apex classes  %d\n", classes)
	fmt.Fprintf(w, "Apex tests    %d\n", tests)
	fmt.Fprintf(w, "Metadata      %s\n", metadata)
	fmt.Fprintf(w, "Last run      %s\n", devLastRun(filepath.Join(p.Root, ".glade", "runs")))
	fmt.Fprint(w, "\nOn change:\n")
	fmt.Fprint(w, "  run changed tests\n")
	fmt.Fprint(w, "  rerun last failures\n")
	fmt.Fprint(w, "  write artifacts to .glade/runs\n")
	fmt.Fprint(w, "\nNext:\n")
	fmt.Fprintf(w, "  glade dev test --project %s\n", shellPathArg(root))
	fmt.Fprintf(w, "  glade dev watch --project %s\n", shellPathArg(root))
	return nil
}

func devDisplayRoot(inputRoot, loadedRoot string) string {
	root := strings.TrimSpace(inputRoot)
	if root == "" || root == "." {
		return "."
	}
	if filepath.IsAbs(root) {
		return filepath.Base(filepath.Clean(root))
	}
	return filepath.ToSlash(filepath.Clean(root))
}

func shellPathArg(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "."
	}
	return filepath.ToSlash(path)
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

func runDevTest(ctx context.Context, args []string, w io.Writer, progressW io.Writer) (testreport.Run, error) {
	root := "."
	outRoot := filepath.Join(".glade", "runs")
	filter := ""
	changed := false
	failed := false
	watchMode := false
	watchOnce := false
	progressArgs := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			value, err := takeFlagValue(args, &i, "--project requires a value")
			if err != nil {
				return testreport.Run{}, err
			}
			root = value
		case "--out":
			value, err := takeFlagValue(args, &i, "--out requires a path")
			if err != nil {
				return testreport.Run{}, err
			}
			outRoot = value
		case "--all":
		case "--class":
			value, err := takeFlagValue(args, &i, "--class requires a value")
			if err != nil {
				return testreport.Run{}, err
			}
			filter = value
		case "--test":
			value, err := takeFlagValue(args, &i, "--test requires a value")
			if err != nil {
				return testreport.Run{}, err
			}
			filter = value
		case "--changed":
			changed = true
		case "--failed":
			failed = true
		case "--watch":
			watchMode = true
		case "--watch-once":
			watchMode = true
			watchOnce = true
		case "--progress", "--progress-json", "--no-progress", "--quiet":
			progressArgs = append(progressArgs, args[i])
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
	testArgs = append(testArgs, progressArgs...)
	result, err := runTest(ctx, testArgs, io.Discard, progressW)
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
	seen := map[string]bool{}
	var failures []string
	for _, suite := range result.Suites {
		for _, testCase := range suite.Cases {
			status := testCase.Status
			if status == "" {
				status = testreport.StatusPass
			}
			if status == testreport.StatusPass || status == testreport.StatusSkipped {
				continue
			}
			name := failedTestName(suite.Name, testCase)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			failures = append(failures, name)
		}
	}
	return strings.Join(failures, ","), nil
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
	p, index, err := loadProjectIndex(root)
	if err != nil {
		return testreport.Run{}, err
	}
	fmt.Fprintln(w, "Glade dev")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Watching project for Apex changes.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "On change:")
	fmt.Fprintln(w, "  run changed tests")
	fmt.Fprintln(w, "  rerun last failures")
	fmt.Fprintf(w, "  write artifacts to %s\n", filepath.ToSlash(outRoot))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Press Ctrl-C to stop.")
	fmt.Fprintln(w)
	var events bytes.Buffer
	result, err := runWatchTests(ctx, root, p, index, apextest.Options{}, watch.Config{Root: root}, once, &events)
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

func runReport(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return errors.New("usage: glade report assess|cruft|refactor-proof|list|show latest|export latest|clean")
	}
	switch args[0] {
	case "assess", "cruft", "refactor-proof":
		return runEnterpriseReport(ctx, args[0], args[1:], w, progressW)
	case "list":
		runsDir, err := parseReportRunsDirArgs(args[1:])
		if err != nil {
			return err
		}
		return runReportList(runsDir, w)
	case "show":
		if len(args) < 2 || args[1] != "latest" {
			return errors.New("usage: glade report show latest [--runs-dir <path>] [--json]")
		}
		runsDir, jsonOut, err := parseReportShowArgs(args[2:])
		if err != nil {
			return err
		}
		return runReportShowLatest(runsDir, jsonOut, w)
	case "github":
		if len(args) < 2 || args[1] != "latest" {
			return errors.New("usage: glade report github latest [--runs-dir <path>]")
		}
		runsDir, err := parseReportRunsDirArgs(args[2:])
		if err != nil {
			return err
		}
		return runReportGitHubLatest(runsDir, w)
	case "export":
		if len(args) < 2 || args[1] != "latest" {
			return errors.New("usage: glade report export latest --output <path> [--format zip|html] [--runs-dir <path>]")
		}
		runsDir, output, format, err := parseReportExportArgs(args[2:])
		if err != nil {
			return err
		}
		if output == "" {
			return errors.New("--output is required")
		}
		return runReportExportLatest(runsDir, output, format, w)
	case "clean":
		runsDir, keep, err := parseReportCleanArgs(args[1:])
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

func parseReportShowArgs(args []string) (runsDir string, jsonOut bool, err error) {
	runsDir = filepath.Join(".glade", "runs")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--runs-dir":
			value, err := takeFlagValue(args, &i, "--runs-dir requires a path")
			if err != nil {
				return "", false, err
			}
			runsDir = value
		case "--json":
			jsonOut = true
		default:
			return "", false, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return runsDir, jsonOut, nil
}

func parseReportExportArgs(args []string) (runsDir string, output string, format string, err error) {
	runsDir = filepath.Join(".glade", "runs")
	format = "zip"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--runs-dir":
			value, err := takeFlagValue(args, &i, "--runs-dir requires a path")
			if err != nil {
				return "", "", "", err
			}
			runsDir = value
		case "--output":
			value, err := takeFlagValue(args, &i, "--output requires a path")
			if err != nil {
				return "", "", "", err
			}
			output = value
		case "--format":
			value, err := takeFlagValue(args, &i, "--format requires a value")
			if err != nil {
				return "", "", "", err
			}
			format = strings.ToLower(strings.TrimSpace(value))
		default:
			return "", "", "", fmt.Errorf("unknown flag %q", args[i])
		}
	}
	switch format {
	case "zip", "html":
	default:
		return "", "", "", errors.New("--format must be zip or html")
	}
	return runsDir, output, format, nil
}

func parseReportRunsDirArgs(args []string) (runsDir string, err error) {
	runsDir = filepath.Join(".glade", "runs")
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--runs-dir":
			value, err := takeFlagValue(args, &i, "--runs-dir requires a path")
			if err != nil {
				return "", err
			}
			runsDir = value
		default:
			return "", fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return runsDir, nil
}

func parseReportCleanArgs(args []string) (runsDir string, keep int, err error) {
	runsDir = filepath.Join(".glade", "runs")
	keep = 10
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--runs-dir":
			value, err := takeFlagValue(args, &i, "--runs-dir requires a path")
			if err != nil {
				return "", 0, err
			}
			runsDir = value
		case "--keep":
			value, err := takeFlagValue(args, &i, "--keep requires a value")
			if err != nil {
				return "", 0, err
			}
			parsed, parseErr := strconv.Atoi(value)
			if parseErr != nil || parsed < 0 {
				return "", 0, errors.New("--keep must be a non-negative integer")
			}
			keep = parsed
		default:
			return "", 0, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return runsDir, keep, nil
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

func runReportShowLatest(runsDir string, jsonOut bool, w io.Writer) error {
	latest, err := readLatest(runsDir)
	if err != nil {
		return err
	}
	if jsonOut {
		envelope, err := loadLatestReportEnvelope(runsDir, latest)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(envelope)
	}
	result, err := loadLatestTestResult(latest)
	if err != nil {
		return err
	}
	summary := result.Summary()
	status := "passed"
	if summary.Failed > 0 || summary.Errors > 0 || summary.Unsupported > 0 {
		status = "failed"
	}
	fmt.Fprintln(w, "Glade report")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Run:     %s\n", filepath.ToSlash(latest.RunDir))
	fmt.Fprintf(w, "Status:  %s\n", status)
	fmt.Fprintf(w, "Tests:   %d passed, %d failed", summary.Passed, summary.Failed)
	if summary.Errors > 0 {
		fmt.Fprintf(w, ", %d errors", summary.Errors)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Artifacts:")
	fmt.Fprintf(w, "  Markdown  %s\n", filepath.ToSlash(latest.SummaryPath))
	fmt.Fprintf(w, "  JSON      %s\n", filepath.ToSlash(latest.ResultsPath))
	junitPath := filepath.Join(latest.RunDir, "junit.xml")
	if _, err := os.Stat(junitPath); err == nil {
		fmt.Fprintf(w, "  JUnit     %s\n", filepath.ToSlash(junitPath))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Open:")
	fmt.Fprintln(w, "  glade report export latest --format html --output glade-report.html")
	return nil
}

func runReportGitHubLatest(runsDir string, w io.Writer) error {
	latest, err := readLatest(runsDir)
	if err != nil {
		return err
	}
	result, err := loadLatestTestResult(latest)
	if err != nil {
		return err
	}
	return testreport.WriteGitHubAnnotations(w, result)
}

func runReportExportLatest(runsDir, output string, format string, w io.Writer) error {
	latest, err := readLatest(runsDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	if format == "html" {
		result, err := loadLatestTestResult(latest)
		if err != nil {
			return err
		}
		file, err := os.Create(output)
		if err != nil {
			return err
		}
		defer file.Close()
		if err := testreport.WriteHTML(file, result, testreport.HTMLReportOptions{Title: "Glade Test Report"}); err != nil {
			return err
		}
		fmt.Fprintf(w, "Exported %s\n", output)
		return nil
	}
	runsRoot, err := os.OpenRoot(runsDir)
	if err != nil {
		return err
	}
	defer runsRoot.Close()
	runsPath, err := filepath.Abs(runsDir)
	if err != nil {
		return err
	}
	runPath, err := filepath.Abs(latest.RunDir)
	if err != nil {
		return err
	}
	physicalRun, err := filepath.EvalSymlinks(runPath)
	if err != nil {
		return err
	}
	outputDir, err := filepath.EvalSymlinks(filepath.Dir(output))
	if err != nil {
		return err
	}
	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	outputPath := filepath.Join(outputDir, filepath.Base(output))
	if outputRel, err := filepath.Rel(physicalRun, outputPath); err == nil && filepath.IsLocal(outputRel) {
		return errors.New("report output must be outside the saved run directory")
	}
	rel, err := filepath.Rel(runsPath, runPath)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return errors.New("saved run must be inside the selected runs directory")
	}
	source, err := runsRoot.OpenRoot(rel)
	if err != nil {
		return err
	}
	defer source.Close()
	file, err := os.CreateTemp(outputDir, ".glade-report-*.zip")
	if err != nil {
		return err
	}
	defer file.Close()
	defer os.Remove(file.Name())
	zw := zip.NewWriter(file)
	err = fs.WalkDir(source.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("report export rejects non-regular entry %q", path)
		}
		data, err := source.ReadFile(path)
		if err != nil {
			return err
		}
		writer, err := zw.Create(path)
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
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), outputPath); err != nil {
		return err
	}
	fmt.Fprintf(w, "Exported %s\n", output)
	return nil
}

func loadLatestReportEnvelope(runsDir string, latest runartifact.Latest) (map[string]any, error) {
	result, err := loadLatestTestResult(latest)
	if err != nil {
		return nil, err
	}
	var runMeta map[string]any
	runData, err := os.ReadFile(filepath.Join(latest.RunDir, "run.json"))
	if err == nil {
		if err := json.Unmarshal(runData, &runMeta); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return map[string]any{
		"latest": latest,
		"run":    runMeta,
		"result": result,
	}, nil
}

func loadLatestTestResult(latest runartifact.Latest) (testreport.Run, error) {
	data, err := os.ReadFile(latest.ResultsPath)
	if err != nil {
		return testreport.Run{}, err
	}
	var result testreport.Run
	if err := json.Unmarshal(data, &result); err != nil {
		return testreport.Run{}, err
	}
	return result, nil
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

func runWatchTests(ctx context.Context, root string, p project.Project, index typesys.Index, opts apextest.Options, cfg watch.Config, once bool, w io.Writer) (testreport.Run, error) {
	if cfg.Root == "" {
		cfg.Root = root
	}
	requestedBackend := cfg.Backend
	replacement, err := prepareInitialLocalWatchReplacement(ctx, root, cfg, requestedBackend, watch.CaptureScope)
	if err != nil {
		return testreport.Run{}, err
	}
	index = replacement.index
	graph := replacement.graph
	watcher := replacement.watcher
	cfg = replacement.cfg
	defer func() { _ = watcher.Close() }()
	if err := writeJSONLine(w, watch.NewWatchStartedEvent(time.Now().UTC(), cfg)); err != nil {
		return testreport.Run{}, err
	}
	runID := 1
	result := testreport.Run{Name: "glade test"}
	initialSelection := watch.TestSelection{Mode: watch.SelectionAll, TestClasses: nil, Reason: "initial watch run"}
	coordinator := newWatchRunCoordinator(runID)
	started := coordinator.Start(initialSelection, func(runID int, selection watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
		return startWatchRun(ctx, index, opts, selection, runID)
	})
	runDone := started.Done
	defer coordinator.Stop()
	if err := writeJSONLine(w, watch.NewRunStartedEvent(time.Now().UTC(), started.RunID, nil)); err != nil {
		return result, err
	}
	if once {
		select {
		case <-ctx.Done():
			coordinator.Stop()
			return result, ctx.Err()
		case finished := <-runDone:
			result = finished.Result
			if err := writeJSONLine(w, watch.NewRunFinishedEvent(time.Now().UTC(), finished.RunID, watchSummary(result))); err != nil {
				return result, err
			}
			if summary, ok := watchProfileSummary(result); ok {
				if err := writeJSONLine(w, watch.NewProfileSummaryEvent(time.Now().UTC(), finished.RunID, summary)); err != nil {
					return result, err
				}
			}
			return result, nil
		}
	}
	for {
		select {
		case <-ctx.Done():
			coordinator.Stop()
			return result, ctx.Err()
		case finished := <-runDone:
			emit, next := coordinator.Complete(finished)
			if emit {
				result = finished.Result
				if err := writeJSONLine(w, watch.NewRunFinishedEvent(time.Now().UTC(), finished.RunID, watchSummary(result))); err != nil {
					return result, err
				}
				if summary, ok := watchProfileSummary(result); ok {
					if err := writeJSONLine(w, watch.NewProfileSummaryEvent(time.Now().UTC(), finished.RunID, summary)); err != nil {
						return result, err
					}
				}
			}
			if next.Started {
				if err := writeJSONLine(w, watch.NewRunStartedEvent(time.Now().UTC(), next.RunID, next.Selection.TestClasses)); err != nil {
					return result, err
				}
			}
			runDone = coordinator.Done()
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
			reload := watchScopeChange(changes)
			var scopeProject *project.Project
			if !reload {
				var update localWatchIndexUpdate
				allowAuthoritativeGraphRefresh := true
				for {
					update, err = tryUpdateWatchIndexStateAllowRefresh(root, cfg.Scope, index, graph, changes, allowAuthoritativeGraphRefresh)
					var drift *testdaemon.WatchStateDriftError
					if !errors.As(err, &drift) || ctx.Err() != nil {
						break
					}
					allowAuthoritativeGraphRefresh = false
				}
				if err != nil {
					_ = writeJSONLine(w, watch.NewErrorEvent(time.Now().UTC(), err.Error(), root))
					continue
				}
				reload = !update.reusable
				if update.loaded {
					scopeProject = &update.project
				}
				if update.reusable {
					index = update.index
					graph = update.graph
				}
			}
			if reload {
				var replacement localWatchReplacement
				var loadErr error
				replacement, loadErr = prepareLocalWatchReplacementRetry(ctx, root, cfg, requestedBackend, scopeProject, watch.CaptureScope)
				if loadErr != nil {
					_ = writeJSONLine(w, watch.NewErrorEvent(time.Now().UTC(), loadErr.Error(), root))
					continue
				}
				oldWatcher := watcher
				watcher = replacement.watcher
				cfg = replacement.cfg
				index = replacement.index
				graph = replacement.graph
				_ = oldWatcher.Close()
			}
			selection := watch.SelectAffectedTestsWithRefGraph(index, changes, graph)
			if err := writeJSONLine(w, watch.NewTestsSelectedEvent(time.Now().UTC(), selection)); err != nil {
				return result, err
			}
			if selection.Mode == watch.SelectionNone {
				continue
			}
			runIndex := index
			started := coordinator.Request(selection, func(runID int, selection watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
				return startWatchRun(ctx, runIndex, opts, selection, runID)
			})
			if started.Started {
				if err := writeJSONLine(w, watch.NewRunStartedEvent(time.Now().UTC(), started.RunID, started.Selection.TestClasses)); err != nil {
					return result, err
				}
				runDone = coordinator.Done()
			}
		}
	}
}

type localWatchReplacement struct {
	watcher watch.BackendWatcher
	cfg     watch.Config
	project project.Project
	index   typesys.Index
	graph   *watch.RefGraph
}

func prepareLocalWatchReplacement(ctx context.Context, root string, cfg watch.Config, requestedBackend watch.Backend) (localWatchReplacement, error) {
	return prepareLocalWatchReplacementWithCapture(ctx, root, cfg, requestedBackend, watch.CaptureScope)
}

func prepareLocalWatchReplacementRetry(ctx context.Context, root string, cfg watch.Config, requestedBackend watch.Backend, initial *project.Project, capture func(watch.Scope) (watch.Snapshot, error)) (localWatchReplacement, error) {
	for {
		if err := ctx.Err(); err != nil {
			return localWatchReplacement{}, err
		}
		var replacement localWatchReplacement
		var err error
		if initial != nil {
			replacement, err = prepareLocalWatchReplacementFromProjectWithCapture(ctx, root, cfg, requestedBackend, *initial, capture)
			initial = nil
		} else {
			replacement, err = prepareLocalWatchReplacementWithCapture(ctx, root, cfg, requestedBackend, capture)
		}
		if err == nil {
			return replacement, nil
		}
		var drift *testdaemon.WatchStateDriftError
		if !errors.As(err, &drift) {
			return localWatchReplacement{}, err
		}
	}
}

func prepareInitialLocalWatchReplacement(ctx context.Context, root string, cfg watch.Config, requestedBackend watch.Backend, capture func(watch.Scope) (watch.Snapshot, error)) (localWatchReplacement, error) {
	for {
		if err := ctx.Err(); err != nil {
			return localWatchReplacement{}, err
		}
		replacement, err := prepareLocalWatchReplacementWithCapture(ctx, root, cfg, requestedBackend, capture)
		if err == nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				_ = replacement.watcher.Close()
				return localWatchReplacement{}, ctxErr
			}
			return replacement, nil
		}
		var drift *testdaemon.WatchStateDriftError
		if !errors.As(err, &drift) {
			return localWatchReplacement{}, err
		}
	}
}

func prepareLocalWatchReplacementWithCapture(ctx context.Context, root string, cfg watch.Config, requestedBackend watch.Backend, capture func(watch.Scope) (watch.Snapshot, error)) (localWatchReplacement, error) {
	scopeProject, err := project.Load(root)
	if err != nil {
		return localWatchReplacement{}, err
	}
	return prepareLocalWatchReplacementFromProjectWithCapture(ctx, root, cfg, requestedBackend, scopeProject, capture)
}

func prepareLocalWatchReplacementFromProjectWithCapture(ctx context.Context, root string, cfg watch.Config, requestedBackend watch.Backend, scopeProject project.Project, capture func(watch.Scope) (watch.Snapshot, error)) (localWatchReplacement, error) {
	return prepareLocalWatchReplacementFromProjectWithGraphBuilder(ctx, root, cfg, requestedBackend, scopeProject, capture, watch.BuildReferenceGraph)
}

func prepareLocalWatchReplacementFromProjectWithGraphBuilder(ctx context.Context, root string, cfg watch.Config, requestedBackend watch.Backend, scopeProject project.Project, capture func(watch.Scope) (watch.Snapshot, error), buildGraph func(typesys.Index) *watch.RefGraph) (localWatchReplacement, error) {
	scope := watch.ProjectScopeWithPrevious(root, scopeProject, cfg.Scope)
	candidateCfg := cfg
	candidateCfg.Backend = requestedBackend
	candidateCfg.Scope = scope
	candidateCfg = candidateCfg.Normalized()
	baseline, err := capture(scope)
	if err != nil {
		return localWatchReplacement{}, err
	}
	candidate, candidateCfg, err := startScopedWatchBackendFromSnapshot(ctx, candidateCfg, requestedBackend, scope, baseline)
	if err != nil {
		return localWatchReplacement{}, err
	}
	replacement := localWatchReplacement{watcher: candidate}
	stable := false
	defer func() {
		if !stable {
			_ = candidate.Close()
		}
	}()
	authoritativeProject, err := project.Load(root)
	if err != nil {
		return localWatchReplacement{}, err
	}
	authoritativeScope := watch.ProjectScopeWithPrevious(root, authoritativeProject, cfg.Scope)
	if !reflect.DeepEqual(scope, authoritativeScope) {
		return localWatchReplacement{}, &testdaemon.WatchStateDriftError{}
	}
	index := buildProjectIndex(authoritativeProject)
	graph := buildGraph(index)
	proof, err := capture(scope)
	if err != nil {
		return localWatchReplacement{}, err
	}
	if changes := watch.DiffSnapshots(baseline, proof); len(changes) != 0 {
		return localWatchReplacement{}, &testdaemon.WatchStateDriftError{Path: changes[0].Path}
	}
	replacement.cfg = candidateCfg
	replacement.project = authoritativeProject
	replacement.index = index
	replacement.graph = graph
	stable = true
	return replacement, nil
}

func startScopedWatchBackend(ctx context.Context, cfg watch.Config, requestedBackend watch.Backend, scope watch.Scope) (watch.BackendWatcher, watch.Config, error) {
	initial, err := watch.CaptureScope(scope)
	if err != nil {
		return nil, cfg, err
	}
	return startScopedWatchBackendFromSnapshot(ctx, cfg, requestedBackend, scope, initial)
}

func startScopedWatchBackendFromSnapshot(ctx context.Context, cfg watch.Config, requestedBackend watch.Backend, scope watch.Scope, initial watch.Snapshot) (watch.BackendWatcher, watch.Config, error) {
	candidateCfg := cfg
	candidateCfg.Backend = requestedBackend
	candidateCfg.Scope = scope
	candidateCfg = candidateCfg.Normalized()
	candidate, backend, err := watch.NewBackendWatcher(ctx, candidateCfg, initial)
	if err != nil {
		return nil, cfg, err
	}
	candidateCfg.Backend = backend
	return candidate, candidateCfg, nil
}

func prepareInitialDaemonWatch(ctx context.Context, daemon *testdaemon.Daemon, cfg watch.Config, requestedBackend watch.Backend, capture func(watch.Scope) (watch.Snapshot, error)) (watch.BackendWatcher, watch.Config, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, cfg, err
		}
		var watcher watch.BackendWatcher
		candidateCfg := cfg
		err := daemon.ReloadPreparedStable(cfg.Scope, capture, func(_ project.Project, scope watch.Scope, baseline watch.Snapshot) error {
			var prepareErr error
			watcher, candidateCfg, prepareErr = startScopedWatchBackendFromSnapshot(ctx, cfg, requestedBackend, scope, baseline)
			return prepareErr
		})
		if err == nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				if watcher != nil {
					_ = watcher.Close()
				}
				return nil, cfg, ctxErr
			}
			return watcher, candidateCfg, nil
		}
		if watcher != nil {
			_ = watcher.Close()
		}
		var drift *testdaemon.WatchStateDriftError
		if !errors.As(err, &drift) {
			return nil, cfg, err
		}
	}
}

func watchScopeChange(changes []watch.Change) bool {
	for _, change := range changes {
		if change.Kind == watch.FileKindTopology {
			return true
		}
		switch strings.ToLower(filepath.Base(change.Path)) {
		case "sfdx-project.json", "glade.yml":
			return true
		}
	}
	return false
}

func runWatchTestsDaemon(ctx context.Context, root string, daemon *testdaemon.Daemon, opts apextest.Options, cfg watch.Config, once bool, w io.Writer) (testreport.Run, error) {
	if cfg.Root == "" {
		cfg.Root = root
	}
	requestedBackend := cfg.Backend
	watcher, cfg, err := prepareInitialDaemonWatch(ctx, daemon, cfg, requestedBackend, watch.CaptureScope)
	if err != nil {
		return testreport.Run{}, err
	}
	defer func() { _ = watcher.Close() }()
	if err := writeJSONLine(w, watch.NewWatchStartedEvent(time.Now().UTC(), cfg)); err != nil {
		return testreport.Run{}, err
	}
	runID := 1
	result := testreport.Run{Name: "glade test"}
	initialSelection := watch.TestSelection{Mode: watch.SelectionAll, TestClasses: nil, Reason: "initial watch run"}
	initialIndex := daemon.IndexSnapshot()
	coordinator := newWatchRunCoordinator(runID)
	started := coordinator.Start(initialSelection, func(runID int, selection watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
		return startWatchRun(ctx, initialIndex, opts, selection, runID)
	})
	runDone := started.Done
	defer coordinator.Stop()
	if err := writeJSONLine(w, watch.NewRunStartedEvent(time.Now().UTC(), started.RunID, nil)); err != nil {
		return result, err
	}
	if once {
		select {
		case <-ctx.Done():
			coordinator.Stop()
			return result, ctx.Err()
		case finished := <-runDone:
			result = finished.Result
			if err := writeJSONLine(w, watch.NewRunFinishedEvent(time.Now().UTC(), finished.RunID, watchSummary(result))); err != nil {
				return result, err
			}
			if summary, ok := watchProfileSummary(result); ok {
				if err := writeJSONLine(w, watch.NewProfileSummaryEvent(time.Now().UTC(), finished.RunID, summary)); err != nil {
					return result, err
				}
			}
			return result, nil
		}
	}
	for {
		select {
		case <-ctx.Done():
			coordinator.Stop()
			return result, ctx.Err()
		case finished := <-runDone:
			emit, next := coordinator.Complete(finished)
			if emit {
				result = finished.Result
				if err := writeJSONLine(w, watch.NewRunFinishedEvent(time.Now().UTC(), finished.RunID, watchSummary(result))); err != nil {
					return result, err
				}
				if summary, ok := watchProfileSummary(result); ok {
					if err := writeJSONLine(w, watch.NewProfileSummaryEvent(time.Now().UTC(), finished.RunID, summary)); err != nil {
						return result, err
					}
				}
			}
			if next.Started {
				if err := writeJSONLine(w, watch.NewRunStartedEvent(time.Now().UTC(), next.RunID, next.Selection.TestClasses)); err != nil {
					return result, err
				}
			}
			runDone = coordinator.Done()
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
			reload := watchScopeChange(changes)
			if !reload {
				exact, updateErr := daemon.TryUpdateChanges(ctx, changes, cfg.Scope)
				if updateErr != nil {
					_ = writeJSONLine(w, watch.NewErrorEvent(time.Now().UTC(), updateErr.Error(), root))
					continue
				}
				reload = !exact
			}
			if reload {
				var candidateWatcher watch.BackendWatcher
				var candidateCfg watch.Config
				var loadErr error
				for {
					candidateWatcher = nil
					loadErr = daemon.ReloadPreparedStable(cfg.Scope, watch.CaptureScope, func(_ project.Project, candidateScope watch.Scope, baseline watch.Snapshot) error {
						var prepareErr error
						candidateWatcher, candidateCfg, prepareErr = startScopedWatchBackendFromSnapshot(ctx, cfg, requestedBackend, candidateScope, baseline)
						return prepareErr
					})
					var drift *testdaemon.WatchStateDriftError
					if !errors.As(loadErr, &drift) || ctx.Err() != nil {
						break
					}
					if candidateWatcher != nil {
						_ = candidateWatcher.Close()
					}
				}
				if loadErr != nil {
					if candidateWatcher != nil {
						_ = candidateWatcher.Close()
					}
					_ = writeJSONLine(w, watch.NewErrorEvent(time.Now().UTC(), loadErr.Error(), root))
					continue
				}
				oldWatcher := watcher
				watcher = candidateWatcher
				cfg = candidateCfg
				_ = oldWatcher.Close()
			}
			selection, starter := prepareDaemonWatchRun(ctx, daemon, opts, changes)
			if err := writeJSONLine(w, watch.NewTestsSelectedEvent(time.Now().UTC(), selection)); err != nil {
				return result, err
			}
			if selection.Mode == watch.SelectionNone {
				continue
			}
			started := coordinator.Request(selection, starter)
			if started.Started {
				if err := writeJSONLine(w, watch.NewRunStartedEvent(time.Now().UTC(), started.RunID, started.Selection.TestClasses)); err != nil {
					return result, err
				}
				runDone = coordinator.Done()
			}
		}
	}
}

type watchRunResult struct {
	RunID  int
	Result testreport.Run
}

type watchRunStarter func(runID int, selection watch.TestSelection) (context.CancelFunc, <-chan watchRunResult)

type watchRunStart struct {
	RunID     int
	Selection watch.TestSelection
	Done      <-chan watchRunResult
	Started   bool
}

type pendingWatchRun struct {
	selection watch.TestSelection
	starter   watchRunStarter
}

type watchRunCoordinator struct {
	nextRunID int

	activeRunID int
	cancel      context.CancelFunc
	done        <-chan watchRunResult
	canceling   bool

	pending *pendingWatchRun
}

func newWatchRunCoordinator(firstRunID int) *watchRunCoordinator {
	return &watchRunCoordinator{nextRunID: firstRunID}
}

func (c *watchRunCoordinator) Start(selection watch.TestSelection, starter watchRunStarter) watchRunStart {
	if c.done != nil {
		return watchRunStart{}
	}
	return c.start(selection, starter)
}

func (c *watchRunCoordinator) Request(selection watch.TestSelection, starter watchRunStarter) watchRunStart {
	if c.done == nil {
		return c.start(selection, starter)
	}
	if c.pending == nil {
		c.pending = &pendingWatchRun{selection: selection, starter: starter}
	} else {
		c.pending.selection = coalesceWatchSelections(c.pending.selection, selection)
		c.pending.starter = starter
	}
	if !c.canceling && c.cancel != nil {
		c.cancel()
		c.canceling = true
	}
	return watchRunStart{}
}

func (c *watchRunCoordinator) Complete(finished watchRunResult) (bool, watchRunStart) {
	if finished.RunID != c.activeRunID {
		return false, watchRunStart{}
	}
	emit := !c.canceling
	c.activeRunID = 0
	c.cancel = nil
	c.done = nil
	c.canceling = false
	if c.pending == nil {
		return emit, watchRunStart{}
	}
	pending := c.pending
	c.pending = nil
	return emit, c.start(pending.selection, pending.starter)
}

func (c *watchRunCoordinator) Done() <-chan watchRunResult {
	return c.done
}

func (c *watchRunCoordinator) Stop() {
	if c.cancel != nil && !c.canceling {
		c.cancel()
		c.canceling = true
	}
}

func (c *watchRunCoordinator) start(selection watch.TestSelection, starter watchRunStarter) watchRunStart {
	runID := c.nextRunID
	c.nextRunID++
	cancel, done := starter(runID, selection)
	c.activeRunID = runID
	c.cancel = cancel
	c.done = done
	c.canceling = false
	return watchRunStart{RunID: runID, Selection: selection, Done: done, Started: true}
}

func coalesceWatchSelections(a, b watch.TestSelection) watch.TestSelection {
	if a.Mode == watch.SelectionNone {
		return b
	}
	if b.Mode == watch.SelectionNone {
		return a
	}
	if a.Mode == watch.SelectionAll || b.Mode == watch.SelectionAll {
		return watch.TestSelection{
			Mode:        watch.SelectionAll,
			TestClasses: unionTestClasses(a.TestClasses, b.TestClasses),
			Reason:      "coalesced watch changes require all tests",
		}
	}
	return watch.TestSelection{
		Mode:        watch.SelectionDirect,
		TestClasses: unionTestClasses(a.TestClasses, b.TestClasses),
		Reason:      "coalesced watch changes reach affected tests",
	}
}

func unionTestClasses(a, b []string) []string {
	canonical := make(map[string]string, len(a)+len(b))
	add := func(names []string) {
		for _, name := range names {
			name = strings.TrimSpace(name)
			key := strings.ToLower(name)
			if key == "" {
				continue
			}
			if _, exists := canonical[key]; exists {
				continue
			}
			canonical[key] = name
		}
	}
	add(a)
	add(b)

	keys := make([]string, 0, len(canonical))
	for key := range canonical {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, canonical[key])
	}
	return out
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

func prepareDaemonWatchRun(ctx context.Context, daemon *testdaemon.Daemon, opts apextest.Options, changes []watch.Change) (watch.TestSelection, watchRunStarter) {
	index, selection := daemon.SnapshotSelection(changes)
	return selection, func(runID int, selection watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
		return startWatchRun(ctx, index, opts, selection, runID)
	}
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
	return typesys.UpdateApexFilesChecked(index, changed, deleted)
}

func updateWatchIndexState(root string, index typesys.Index, graph *watch.RefGraph, changes []watch.Change) (typesys.Index, *watch.RefGraph, error) {
	updated, err := updateWatchIndex(root, index, changes)
	if err != nil {
		return index, graph, err
	}
	return updated, graph.Refresh(updated, changes), nil
}

type localWatchIndexUpdate struct {
	project  project.Project
	index    typesys.Index
	graph    *watch.RefGraph
	loaded   bool
	reusable bool
}

func tryUpdateWatchIndexState(root string, currentScope watch.Scope, index typesys.Index, graph *watch.RefGraph, changes []watch.Change) (localWatchIndexUpdate, error) {
	return tryUpdateWatchIndexStateAllowRefresh(root, currentScope, index, graph, changes, true)
}

func tryUpdateWatchIndexStateAllowRefresh(root string, currentScope watch.Scope, index typesys.Index, graph *watch.RefGraph, changes []watch.Change, allowAuthoritativeGraphRefresh bool) (localWatchIndexUpdate, error) {
	return tryUpdateWatchIndexStateWithFuncs(root, currentScope, index, graph, changes, project.Load, func(p project.Project) (typesys.Index, error) {
		return buildProjectIndex(p), nil
	}, watch.CaptureScope, allowAuthoritativeGraphRefresh)
}

func tryUpdateWatchIndexStateWithFuncs(root string, currentScope watch.Scope, index typesys.Index, graph *watch.RefGraph, changes []watch.Change, load func(string) (project.Project, error), build func(project.Project) (typesys.Index, error), capture func(watch.Scope) (watch.Snapshot, error), allowAuthoritativeGraphRefresh ...bool) (localWatchIndexUpdate, error) {
	allowRefresh := len(allowAuthoritativeGraphRefresh) == 0 || allowAuthoritativeGraphRefresh[0]
	return tryUpdateWatchIndexStateWithGraphRefreshAllowed(root, currentScope, index, graph, changes, load, build, capture, func(graph *watch.RefGraph, index typesys.Index, changes []watch.Change) (*watch.RefGraph, error) {
		return graph.Refreshed(index, changes), nil
	}, allowRefresh)
}

func tryUpdateWatchIndexStateWithGraphRefresh(root string, currentScope watch.Scope, index typesys.Index, graph *watch.RefGraph, changes []watch.Change, load func(string) (project.Project, error), build func(project.Project) (typesys.Index, error), capture func(watch.Scope) (watch.Snapshot, error), refreshGraph func(*watch.RefGraph, typesys.Index, []watch.Change) (*watch.RefGraph, error)) (localWatchIndexUpdate, error) {
	return tryUpdateWatchIndexStateWithGraphRefreshAllowed(root, currentScope, index, graph, changes, load, build, capture, refreshGraph, true)
}

func tryUpdateWatchIndexStateWithGraphRefreshAllowed(root string, currentScope watch.Scope, index typesys.Index, graph *watch.RefGraph, changes []watch.Change, load func(string) (project.Project, error), build func(project.Project) (typesys.Index, error), capture func(watch.Scope) (watch.Snapshot, error), refreshGraph func(*watch.RefGraph, typesys.Index, []watch.Change) (*watch.RefGraph, error), allowAuthoritativeGraphRefresh bool) (localWatchIndexUpdate, error) {
	result := localWatchIndexUpdate{index: index, graph: graph}
	if !canIncrementalIndex(changes) {
		return result, nil
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
	var baseline watch.Snapshot
	baselineCaptured := false
	if typesys.RequiresAuthoritativeApexRebuild(index, changed, deleted) {
		var err error
		baseline, err = capture(currentScope)
		if err != nil {
			return result, err
		}
		baselineCaptured = true
	}
	p, err := load(root)
	if err != nil {
		return result, err
	}
	result.project = p
	result.loaded = true
	updated, exact, err := typesys.TryUpdateApexFilesCheckedWithLoadedProject(index, changed, deleted, p)
	if err != nil {
		return result, err
	}
	if exact {
		result.index = updated
		result.graph = graph.Refresh(updated, changes)
		result.reusable = true
		return result, nil
	}
	if !baselineCaptured {
		baseline, err = capture(currentScope)
		if err != nil {
			return result, err
		}
		p, err = load(root)
		if err != nil {
			return result, err
		}
		result.project = p
	}
	if !typesys.MatchesProjectIdentity(index, p) {
		return result, nil
	}
	candidateScope := watch.ProjectScopeWithPrevious(root, p, currentScope)
	if !reflect.DeepEqual(candidateScope, currentScope) {
		return result, nil
	}
	updated, err = build(p)
	if err != nil {
		return result, err
	}
	var refreshedGraph *watch.RefGraph
	graphChanges, digestCoverage := watch.AuthoritativeApexGraphChanges(index, updated)
	if allowAuthoritativeGraphRefresh && digestCoverage && watch.CanRefreshAuthoritativeFallbackGraph(index, updated, graphChanges) {
		refreshedGraph, err = refreshGraph(graph, updated, graphChanges)
		if err != nil {
			return result, err
		}
	} else {
		refreshedGraph = watch.BuildReferenceGraph(updated)
	}
	proof, err := capture(currentScope)
	if err != nil {
		return result, err
	}
	if drift := watch.DiffSnapshots(baseline, proof); len(drift) != 0 {
		return result, &testdaemon.WatchStateDriftError{Path: drift[0].Path}
	}
	result.index = updated
	result.graph = refreshedGraph
	result.reusable = true
	return result, nil
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
	selectedOpts, ok := watch.ApplyTestSelection(opts, selection)
	if !ok {
		return testreport.Run{Name: "glade test"}
	}
	return apextest.RunContext(ctx, index, selectedOpts)
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

func watchProfileSummary(result testreport.Run) (watch.ProfileSummary, bool) {
	var summary watch.ProfileSummary
	for _, suite := range result.Suites {
		for _, testCase := range suite.Cases {
			if testCase.Profile == nil {
				continue
			}
			summary.Profiles++
			summary.Events += testCase.Profile.Events
			summary.WallClockMS += testCase.Profile.WallClockMS
			if testCase.Profile.Limits.CPUTimeMS > summary.CPUTimeMS {
				summary.CPUTimeMS = testCase.Profile.Limits.CPUTimeMS
			}
			if len(testCase.Profile.Spans) > 0 && testCase.Profile.Spans[0].DurationMS > summary.TopSpanMS {
				summary.TopSpan = testCase.Profile.Spans[0].Name
				summary.TopSpanMS = testCase.Profile.Spans[0].DurationMS
			}
			if len(testCase.Trace) > 0 {
				summary.TraceEventCount += len(testCase.Trace)
			}
			name := testCase.ClassName
			if testCase.MethodName != "" {
				if name != "" {
					name += "."
				}
				name += testCase.MethodName
			}
			if strings.TrimSpace(name) != "" {
				summary.ProfiledTests = append(summary.ProfiledTests, name)
			}
		}
	}
	return summary, summary.Profiles > 0
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
