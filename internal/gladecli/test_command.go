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
	"runtime/debug"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/compat"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
	"github.com/glade-sh/glade/internal/watch"
)

func runTest(ctx context.Context, args []string, w io.Writer, progressW io.Writer) (testreport.Run, error) {
	if err := ctx.Err(); err != nil {
		return testreport.Run{}, err
	}
	if len(args) > 0 && isHelpArg(args[0]) {
		printTestHelp(w)
		return testreport.Run{}, nil
	}

	root := "."
	filter := ""
	format := "console"
	junitPath := ""
	limitMode := vm.LimitMode("")
	watchMode := false
	watchOnce := false
	daemonMode := false
	debug := false
	progress := false
	traceBlocked := false
	slowTestThresholdMS := int64(0)
	changedSince := ""
	parallelMethods := true
	parallelismOverride := 0
	testTimeout := 5 * time.Minute
	gcAggressive := false
	cpuProfilePath := ""
	memProfilePath := ""
	perfJSONPath := ""
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
		case "--compat-json":
			format = "compat-json"
			traceBlocked = true
		case "--trace-blockers":
			traceBlocked = true
		case "--slow-test-ms":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--slow-test-ms requires a value")
			}
			parsed, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil || parsed < 0 {
				return testreport.Run{}, fmt.Errorf("--slow-test-ms must be a non-negative integer")
			}
			slowTestThresholdMS = parsed
			i++
		case "--changed-since":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--changed-since requires a value")
			}
			changedSince = args[i+1]
			i++
		case "--parallel-methods":
			parallelMethods = true
		case "--no-parallel-methods":
			parallelMethods = false
		case "--parallelism":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--parallelism requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed < 0 {
				return testreport.Run{}, fmt.Errorf("--parallelism must be a non-negative integer")
			}
			parallelismOverride = parsed
			i++
		case "--test-timeout":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--test-timeout requires a duration")
			}
			parsed, err := time.ParseDuration(args[i+1])
			if err != nil {
				return testreport.Run{}, fmt.Errorf("--test-timeout: %w", err)
			}
			if parsed < 0 {
				return testreport.Run{}, errors.New("--test-timeout must be non-negative")
			}
			testTimeout = parsed
			i++
		case "--gc-aggressive":
			gcAggressive = true
		case "--cpu-profile":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--cpu-profile requires a path")
			}
			cpuProfilePath = args[i+1]
			i++
		case "--mem-profile":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--mem-profile requires a path")
			}
			memProfilePath = args[i+1]
			i++
		case "--perf-json":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--perf-json requires a path")
			}
			perfJSONPath = args[i+1]
			i++
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
		case "--daemon":
			daemonMode = true
		case "--debug":
			debug = true
		case "--progress":
			progress = true
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
	stopProfile, err := startCLIProfiler(cpuProfilePath, memProfilePath)
	if err != nil {
		return testreport.Run{}, err
	}
	defer func() {
		if stopProfile != nil {
			_ = stopProfile()
		}
	}()

	var progressReporter *cliTestProgressReporter
	if progress {
		progressReporter = newCLITestProgressReporter(progressW)
		defer progressReporter.finish()
	}
	testOpts := apextest.Options{Filter: filter, LimitMode: limitMode, TraceBlocked: traceBlocked, SlowTestThresholdMS: slowTestThresholdMS, ParallelMethods: parallelMethods, TimeoutMS: testTimeout.Milliseconds()}
	if progress {
		testOpts.Progress = progressReporter.handle
	}
	switch {
	case parallelismOverride > 0:
		testOpts.Parallelism = parallelismOverride
	case parallelMethods:
		testOpts.Parallelism = runtime.GOMAXPROCS(0)
	}
	applyTestMemoryLimits(gcAggressive)
	if daemonMode {
		daemon, err := testdaemon.New(root)
		if err != nil {
			return testreport.Run{}, err
		}
		if watchMode {
			return runWatchTestsDaemon(ctx, root, daemon, testOpts, watch.Config{Root: root, Debounce: debounce, Backend: backend}, watchOnce, w)
		}
		var result testreport.Run
		if strings.TrimSpace(changedSince) != "" {
			run, _, err := daemon.RunChangedSinceOptions(changedSince, testOpts)
			if err != nil {
				return testreport.Run{}, err
			}
			result = run
		} else {
			result = daemon.RunOptions(testOpts)
		}
		if progressReporter != nil {
			progressReporter.finish()
		}
		if stopProfile != nil {
			if stopErr := stopProfile(); stopErr != nil {
				return result, stopErr
			}
			stopProfile = nil
		}
		if err := maybeWriteRunPerfJSON(perfJSONPath, root, result, cpuProfilePath, memProfilePath); err != nil {
			return result, err
		}
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
		case "compat-json":
			return result, compat.WriteLocalTestJSON(w, compat.LocalTestReportFromRun(root, result))
		default:
			return result, testreport.WriteConsole(w, result)
		}
	}
	index, err := loadIndex(root)
	if err != nil {
		return testreport.Run{}, err
	}
	if watchMode {
		return runWatchTests(ctx, root, index, testOpts, watch.Config{Root: root, Debounce: debounce, Backend: backend}, watchOnce, w)
	}
	var result testreport.Run
	if strings.TrimSpace(changedSince) != "" {
		cases := apextest.Discover(index, testOpts)
		selection, err := changedSinceSelection(root, index, changedSince)
		if err != nil {
			return testreport.Run{}, err
		}
		cases = filterSelectedTestCases(cases, selection)
		if progressReporter != nil {
			progressReporter.setTotal(len(cases))
		}
		result = apextest.RunCasesContext(ctx, index, testOpts, cases)
	} else {
		cases := apextest.Discover(index, testOpts)
		if progressReporter != nil {
			progressReporter.setTotal(len(cases))
		}
		result = apextest.RunCasesContext(ctx, index, testOpts, cases)
	}
	if progressReporter != nil {
		progressReporter.finish()
	}
	if stopProfile != nil {
		if stopErr := stopProfile(); stopErr != nil {
			return result, stopErr
		}
		stopProfile = nil
	}
	result.Dependencies = append(result.Dependencies, index.Dependencies...)
	if err := maybeWriteRunPerfJSON(perfJSONPath, root, result, cpuProfilePath, memProfilePath); err != nil {
		return result, err
	}
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
	case "compat-json":
		return result, compat.WriteLocalTestJSON(w, compat.LocalTestReportFromRun(root, result))
	default:
		return result, testreport.WriteConsole(w, result)
	}
}

type cliTestProgressReporter struct {
	w        io.Writer
	started  time.Time
	last     time.Time
	total    int
	done     int
	passed   int
	failed   int
	errors   int
	active   string
	terminal bool
	printed  bool
	mu       sync.Mutex
	finished bool
}

func newCLITestProgressReporter(w io.Writer) *cliTestProgressReporter {
	return &cliTestProgressReporter{
		w:        w,
		started:  time.Now(),
		terminal: isTerminalWriter(w),
	}
}

func (r *cliTestProgressReporter) setTotal(total int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.total = total
	r.mu.Unlock()
}

func (r *cliTestProgressReporter) handle(progress apextest.TestProgress) {
	if r == nil || r.w == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started.IsZero() {
		r.started = time.Now()
	}
	important := false
	name := progress.ClassName
	if progress.MethodName != "" {
		name += "." + progress.MethodName
	}
	switch progress.Event {
	case "test_start", "setup_start":
		r.active = name
	case "test_done":
		r.done++
		switch testreport.Status(progress.Status) {
		case testreport.StatusPass:
			r.passed++
		case testreport.StatusFail:
			r.failed++
			important = r.failed <= 10
		case testreport.StatusCompileError, testreport.StatusRuntimeError, testreport.StatusUnsupported:
			r.errors++
			important = r.errors <= 10
		}
		r.active = name
	case "setup_done":
		if progress.Status != "pass" {
			r.errors++
			important = r.errors <= 10
		}
		r.active = name
	}
	if r.terminal {
		r.writeLine(true)
		return
	}
	now := time.Now()
	if !r.printed || r.done == r.total || important || r.done%25 == 0 || now.Sub(r.last) >= 2*time.Second {
		r.writeLine(false)
	}
}

func (r *cliTestProgressReporter) finish() {
	if r == nil || r.w == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return
	}
	r.finished = true
	if r.printed && r.terminal {
		fmt.Fprint(r.w, "\n")
		return
	}
	if r.printed {
		return
	}
	r.writeLine(false)
}

func (r *cliTestProgressReporter) writeLine(redraw bool) {
	line := fmt.Sprintf("Progress: %s elapsed=%s eta=%s pass=%d fail=%d error=%d",
		r.countText(), formatProgressDuration(time.Since(r.started)), r.etaText(), r.passed, r.failed, r.errors)
	if r.active != "" {
		line += " running=" + r.active
	}
	if redraw {
		fmt.Fprintf(r.w, "\r\x1b[K%s", line)
	} else {
		fmt.Fprintln(r.w, line)
	}
	r.printed = true
	r.last = time.Now()
}

func (r *cliTestProgressReporter) countText() string {
	if r.total > 0 {
		return fmt.Sprintf("%d/%d", r.done, r.total)
	}
	return fmt.Sprintf("%d done", r.done)
}

func (r *cliTestProgressReporter) etaText() string {
	if r.total <= 0 || r.done <= 0 || r.done >= r.total {
		return "0s"
	}
	elapsed := time.Since(r.started)
	remaining := time.Duration(int64(elapsed) * int64(r.total-r.done) / int64(r.done))
	return formatProgressDuration(remaining)
}

func formatProgressDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d/time.Minute), int((d%time.Minute)/time.Second))
	}
	return fmt.Sprintf("%dh%02dm", int(d/time.Hour), int((d%time.Hour)/time.Minute))
}

func isTerminalWriter(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok || file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

type runPerfSummary struct {
	GeneratedAt    string             `json:"generatedAt"`
	Project        string             `json:"project"`
	DurationMS     int64              `json:"durationMs"`
	Summary        testreport.Summary `json:"summary"`
	TopSlowClasses []runPerfClass     `json:"topSlowClasses,omitempty"`
	CPUProfilePath string             `json:"cpuProfilePath,omitempty"`
	MemProfilePath string             `json:"memProfilePath,omitempty"`
}

type runPerfClass struct {
	Class      string `json:"class"`
	DurationMS int64  `json:"durationMs"`
	Tests      int    `json:"tests"`
}

func startCLIProfiler(cpuProfilePath, memProfilePath string) (func() error, error) {
	if strings.TrimSpace(cpuProfilePath) == "" && strings.TrimSpace(memProfilePath) == "" {
		return nil, nil
	}
	var cpuFile *os.File
	if path := strings.TrimSpace(cpuProfilePath); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		file, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		if err := pprof.StartCPUProfile(file); err != nil {
			_ = file.Close()
			return nil, err
		}
		cpuFile = file
	}
	return func() error {
		if cpuFile != nil {
			pprof.StopCPUProfile()
			if err := cpuFile.Close(); err != nil {
				return err
			}
		}
		if path := strings.TrimSpace(memProfilePath); path != "" {
			runtime.GC()
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			file, err := os.Create(path)
			if err != nil {
				return err
			}
			if err := pprof.WriteHeapProfile(file); err != nil {
				_ = file.Close()
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
		}
		return nil
	}, nil
}

func maybeWriteRunPerfJSON(perfJSONPath, root string, result testreport.Run, cpuProfilePath, memProfilePath string) error {
	if strings.TrimSpace(perfJSONPath) == "" {
		return nil
	}
	absRoot, _ := filepath.Abs(root)
	perf := runPerfSummary{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		Project:        absRoot,
		DurationMS:     result.Summary().DurationMS,
		Summary:        result.Summary(),
		CPUProfilePath: strings.TrimSpace(cpuProfilePath),
		MemProfilePath: strings.TrimSpace(memProfilePath),
	}
	perf.TopSlowClasses = runTopSlowClasses(result, 15)
	if err := os.MkdirAll(filepath.Dir(perfJSONPath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(perf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(perfJSONPath, append(data, '\n'), 0o644)
}

func runTopSlowClasses(result testreport.Run, limit int) []runPerfClass {
	if limit <= 0 {
		return nil
	}
	totals := map[string]*runPerfClass{}
	for _, suite := range result.Suites {
		for _, testCase := range suite.Cases {
			className := strings.TrimSpace(testCase.ClassName)
			if className == "" {
				className = strings.TrimSpace(suite.Name)
			}
			if className == "" {
				continue
			}
			entry := totals[className]
			if entry == nil {
				entry = &runPerfClass{Class: className}
				totals[className] = entry
			}
			entry.DurationMS += testCase.DurationMS
			entry.Tests++
		}
	}
	if len(totals) == 0 {
		return nil
	}
	classes := make([]runPerfClass, 0, len(totals))
	for _, entry := range totals {
		classes = append(classes, *entry)
	}
	sort.Slice(classes, func(i, j int) bool {
		if classes[i].DurationMS == classes[j].DurationMS {
			return classes[i].Class < classes[j].Class
		}
		return classes[i].DurationMS > classes[j].DurationMS
	})
	if len(classes) > limit {
		classes = classes[:limit]
	}
	return classes
}

// applyTestMemoryLimits sets a default soft memory ceiling for `glade test`
// so the runtime returns memory under pressure instead of growing without
// bound on the long-tail of a clone-heavy workload. Honours an existing
// GOMEMLIMIT environment variable: if the user set one, the runtime will
// already have picked it up and we skip the override.
func applyTestMemoryLimits(aggressive bool) {
if os.Getenv("GOMEMLIMIT") == "" {
// 4 GiB soft cap. Workloads that need more should set GOMEMLIMIT.
debug.SetMemoryLimit(4 << 30)
}
if aggressive {
debug.SetGCPercent(50)
}
}
