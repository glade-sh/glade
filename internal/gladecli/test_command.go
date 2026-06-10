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
	"github.com/glade-sh/glade/internal/cliui"
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
		_ = cliui.WriteTestHelp(w)
		return testreport.Run{}, nil
	}
	if len(args) > 0 && args[0] == "clear-cache" {
		rest := args[1:]
		if len(rest) > 0 && isHelpArg(rest[0]) {
			_ = cliui.WriteTestHelp(w)
			return testreport.Run{}, nil
		}
		if err := runTestClearCache(rest, w); err != nil {
			return testreport.Run{}, err
		}
		return testreport.Run{}, nil
	}
	if len(args) > 0 && args[0] == "serve" {
		rest := args[1:]
		if len(rest) > 0 && isHelpArg(rest[0]) {
			_ = cliui.WriteTestHelp(w)
			return testreport.Run{}, nil
		}
		if err := runTestServe(ctx, rest, progressW); err != nil {
			return testreport.Run{}, err
		}
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
	connectMode := false
	noServe := false
	noCache := false
	debug := false
	progressMode := cliui.ProgressAuto
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
	var compileDoneMS int64
	var discoverTimeMS int64
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
		case "--connect":
			connectMode = true
		case "--no-serve":
			noServe = true
		case "--no-cache":
			noCache = true
		case "--debug":
			debug = true
		case "--progress":
			progressMode = cliui.ProgressLine
		case "--progress-json":
			progressMode = cliui.ProgressJSON
		case "--no-progress", "--quiet":
			progressMode = cliui.ProgressOff
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
	if progressMode != cliui.ProgressOff {
		progressReporter = newCLITestProgressReporter(cliui.NewRenderer(cliui.RendererOptions{
			Stderr: progressW,
			Mode:   progressMode,
		}))
		defer progressReporter.finish()
	}
	testOpts := apextest.Options{
		Filter:              filter,
		LimitMode:           limitMode,
		TraceBlocked:        traceBlocked,
		SlowTestThresholdMS: slowTestThresholdMS,
		ParallelMethods:     parallelMethods,
		TimeoutMS:           testTimeout.Milliseconds(),
		NoDiskCache:         noCache,
	}
	if progressReporter != nil {
		testOpts.Progress = func(progress apextest.TestProgress) {
			if progress.Event == "compile_done" {
				compileDoneMS = progress.DurationMS
			}
			progressReporter.enqueue(progress)
		}
	}
	switch {
	case parallelismOverride > 0:
		testOpts.Parallelism = parallelismOverride
	case parallelMethods:
		testOpts.Parallelism = runtime.GOMAXPROCS(0)
	}
	applyTestMemoryLimits(gcAggressive)

	if noCache {
		noServe = true
	}
	if !noServe && !watchMode && !daemonMode && !debug {
		if result, used, err := tryTestServerRun(ctx, root, connectMode, filter, changedSince, format, junitPath, debug, w); used || err != nil {
			return result, err
		}
	}

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
			result.DurationMS = time.Since(progressReporter.started).Milliseconds()
			progressReporter.finish()
		}
		if stopProfile != nil {
			if stopErr := stopProfile(); stopErr != nil {
				return result, stopErr
			}
			stopProfile = nil
		}
		if err := maybeWriteRunPerfJSON(perfJSONPath, root, result, cpuProfilePath, memProfilePath, 0, 0, result.Summary().DurationMS); err != nil {
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
		default:
			return result, testreport.WriteConsole(w, result)
		}
	}
	index, err := loadIndex(root)
	if err != nil {
		return testreport.Run{}, err
	}
	if progressReporter != nil && len(index.Project.Root) > 0 {
		if root != index.Project.Root && root == "." {
			root = "."
		}
		if len(index.Types) > 12000 {
			projectPath := index.Project.Root
			progressReporter.warn(fmt.Sprintf("large project index: %d types. For faster startup, run from a scoped project path with --project.", len(index.Types)))
			if projectPath != "" {
				progressReporter.warn(fmt.Sprintf("for this workspace, try: --project %s", projectPath))
			}
		}
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
		discoverStart := time.Now()
		cases = filterSelectedTestCases(cases, selection)
		discoverTimeMS = time.Since(discoverStart).Milliseconds()
		if progressReporter != nil {
			progressReporter.setTotal(len(cases))
		}
		result = apextest.RunCasesContext(ctx, index, testOpts, cases)
	} else {
		discoverStart := time.Now()
		cases := apextest.Discover(index, testOpts)
		discoverTimeMS = time.Since(discoverStart).Milliseconds()
		if progressReporter != nil {
			progressReporter.setTotal(len(cases))
		}
		result = apextest.RunCasesContext(ctx, index, testOpts, cases)
	}
	if progressReporter != nil {
		result.DurationMS = time.Since(progressReporter.started).Milliseconds()
	}
	runDurationMS := result.Summary().DurationMS
	if progressReporter != nil && compileDoneMS > 10000 && index.Project.Root != "" && index.Project.Root != "." {
		progressReporter.warn(fmt.Sprintf("compile test harness took %dms", compileDoneMS))
		progressReporter.warn(fmt.Sprintf("for this workspace, try: --project %s", index.Project.Root))
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
	if err := maybeWriteRunPerfJSON(perfJSONPath, root, result, cpuProfilePath, memProfilePath, discoverTimeMS, compileDoneMS, runDurationMS); err != nil {
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
	default:
		return result, testreport.WriteConsole(w, result)
	}
}

type cliTestProgressReporter struct {
	renderer  cliui.Renderer
	started   time.Time
	total     int
	done      int
	inflight  int
	passed    int
	failed    int
	errors    int
	active    string
	phase     string
	immediate cliui.Event
	mu        sync.Mutex
	finished  bool
	events    chan apextest.TestProgress
	closeOnce sync.Once
	wg        sync.WaitGroup
}

const progressEventBuffer = 8192

func newCLITestProgressReporter(renderer cliui.Renderer) *cliTestProgressReporter {
	r := &cliTestProgressReporter{
		renderer: renderer,
		started:  time.Now(),
		events:   make(chan apextest.TestProgress, progressEventBuffer),
	}
	r.wg.Add(1)
	go r.loop()
	r.renderer.Render(cliui.Event{
		Kind:  cliui.EventPhaseStart,
		Phase: "test",
		Label: "Discovering tests",
	})
	return r
}

func (r *cliTestProgressReporter) enqueue(progress apextest.TestProgress) {
	if r == nil {
		return
	}
	r.events <- progress
}

func (r *cliTestProgressReporter) loop() {
	defer r.wg.Done()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	dirty := false
	flush := func() {
		if r == nil || r.renderer == nil {
			dirty = false
			return
		}
		if ev, ok := r.pendingRender(); ok {
			r.renderer.Render(ev)
		}
		dirty = false
	}

	for {
		select {
		case progress, ok := <-r.events:
			if !ok {
				flush()
				return
			}
			var closed bool
			dirty, closed = r.drainProgress(progress, flush, dirty)
			if closed {
				flush()
				return
			}
		case <-ticker.C:
			if dirty {
				flush()
			}
		}
	}
}

func (r *cliTestProgressReporter) drainProgress(first apextest.TestProgress, flush func(), dirty bool) (bool, bool) {
	progress := first
	for {
		if r.apply(progress) {
			flush()
			dirty = false
		} else {
			dirty = true
		}
		select {
		case next, ok := <-r.events:
			if !ok {
				return dirty, true
			}
			progress = next
		default:
			return dirty, false
		}
	}
}

func (r *cliTestProgressReporter) setTotal(total int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.total = total
	r.phase = "Running tests"
	r.mu.Unlock()
	r.flushNow()
}

func (r *cliTestProgressReporter) apply(progress apextest.TestProgress) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	name := progress.ClassName
	if progress.MethodName != "" {
		name += "." + progress.MethodName
	}

	switch progress.Event {
	case "setup_start":
		r.active = name
		r.phase = r.leftLabel("setting up " + name)
		return r.total > 0 && r.total <= 100
	case "test_start":
		r.inflight++
		r.active = name
		r.phase = r.leftLabel(fmt.Sprintf("running %s", name))
		return r.total > 0 && r.total <= 100
	case "compile_start":
		r.active = ""
		r.phase = "compile test harness"
		return true
	case "compile_done":
		r.active = ""
		duration := fmt.Sprintf(" %dms", progress.DurationMS)
		if progress.DurationMS <= 0 {
			duration = ""
		}
		r.phase = "compile complete" + duration
		return true
	case "test_done":
		r.done++
		if r.inflight > 0 {
			r.inflight--
		}
		r.active = ""
		r.phase = r.leftLabel("")
		switch testreport.Status(progress.Status) {
		case testreport.StatusPass:
			r.passed++
			if progress.DurationMS >= 10_000 {
				r.immediate = cliui.Event{
					Kind:    cliui.EventWarn,
					Phase:   "test",
					Label:   fmt.Sprintf("slow %s (%s)", name, cliui.FormatDurationMS(progress.DurationMS)),
					Current: r.done,
					Total:   r.total,
				}
				return true
			}
			return false
		case testreport.StatusFail:
			r.failed++
			r.immediate = cliui.Event{
				Kind:    cliui.EventFail,
				Phase:   "test",
				Label:   "FAIL " + name,
				Current: r.done,
				Total:   r.total,
			}
			return true
		case testreport.StatusCompileError, testreport.StatusRuntimeError, testreport.StatusUnsupported:
			r.errors++
			r.immediate = cliui.Event{
				Kind:    cliui.EventFail,
				Phase:   "test",
				Label:   string(progress.Status) + " " + name,
				Current: r.done,
				Total:   r.total,
			}
			return true
		}
	case "setup_done":
		if progress.Status != "pass" {
			r.errors++
			r.immediate = cliui.Event{
				Kind:    cliui.EventFail,
				Phase:   "test",
				Label:   "setup failed " + name,
				Current: r.done,
				Total:   r.total,
			}
			return true
		}
	}
	return false
}

func (r *cliTestProgressReporter) leftLabel(prefix string) string {
	left := r.total - r.done
	if r.inflight > 0 {
		if prefix != "" {
			return fmt.Sprintf("%s · %d running · %d left", prefix, r.inflight, left)
		}
		return fmt.Sprintf("%d running · %d left", r.inflight, left)
	}
	if prefix != "" {
		if left > 0 {
			return fmt.Sprintf("%s · %d left", prefix, left)
		}
		return prefix
	}
	if left > 0 {
		return fmt.Sprintf("%d left", left)
	}
	return ""
}

func (r *cliTestProgressReporter) pendingRender() (cliui.Event, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.immediate.Kind != "" {
		ev := r.immediate
		r.immediate = cliui.Event{}
		return ev, true
	}
	label := strings.TrimSpace(r.phase)
	if label == "" {
		label = r.leftLabel("")
	}
	return cliui.Event{
		Kind:    cliui.EventPhaseTick,
		Phase:   "test",
		Label:   label,
		Current: r.done,
		Total:   r.total,
	}, true
}

func (r *cliTestProgressReporter) flushNow() {
	if ev, ok := r.pendingRender(); ok && r.renderer != nil {
		r.renderer.Render(ev)
	}
}

func (r *cliTestProgressReporter) warn(message string) {
	if r == nil || r.renderer == nil {
		return
	}
	r.renderer.Render(cliui.Event{
		Kind:  cliui.EventWarn,
		Phase: "test",
		Label: message,
	})
}

func (r *cliTestProgressReporter) finish() {
	if r == nil || r.renderer == nil {
		return
	}
	r.closeOnce.Do(func() { close(r.events) })
	r.wg.Wait()

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return
	}
	r.finished = true
	ok := r.failed == 0 && r.errors == 0
	current := r.done
	if r.total > 0 && current < r.total {
		current = r.total
	}
	elapsed := cliui.FormatDuration(time.Since(r.started))
	r.renderer.Render(cliui.Event{
		Kind:    cliui.EventPhaseTick,
		Phase:   "test",
		Label:   "tests complete",
		Current: current,
		Total:   r.total,
	})
	r.renderer.Finish(cliui.Result{
		OK:    ok,
		Label: fmt.Sprintf("%d passed, %d failed, %d errors · %s", r.passed, r.failed, r.errors, elapsed),
	})
}

type runPerfSummary struct {
	GeneratedAt    string             `json:"generatedAt"`
	Project        string             `json:"project"`
	DurationMS     int64              `json:"durationMs"`
	DiscoverMS     int64              `json:"discoverMs"`
	CompileMS      int64              `json:"compileMs"`
	TotalMS        int64              `json:"totalMs"`
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

func maybeWriteRunPerfJSON(perfJSONPath, root string, result testreport.Run, cpuProfilePath, memProfilePath string, discoverMS, compileMS, totalMS int64) error {
	if strings.TrimSpace(perfJSONPath) == "" {
		return nil
	}
	absRoot, _ := filepath.Abs(root)
	perf := runPerfSummary{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		Project:        absRoot,
		DurationMS:     result.Summary().DurationMS,
		DiscoverMS:     discoverMS,
		CompileMS:      compileMS,
		TotalMS:        totalMS,
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
