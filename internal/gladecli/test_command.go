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
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/enterprise"
	"github.com/glade-sh/glade/internal/flagparse"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/trace"
	"github.com/glade-sh/glade/internal/tui"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
	"github.com/glade-sh/glade/internal/watch"
)

func runTest(ctx context.Context, args []string, w io.Writer, progressW io.Writer) (testreport.Run, error) {
	if err := ctx.Err(); err != nil {
		return testreport.Run{}, err
	}
	if len(args) > 0 && isHelpArg(args[0]) {
		_ = writeTestHelp(w)
		return testreport.Run{}, nil
	}
	if hasUIFlag(args) {
		if err := runTUIView(ctx, args, tui.BoardTests, w, progressW); err != nil {
			return testreport.Run{}, err
		}
		return testreport.Run{}, nil
	}
	if len(args) > 0 && args[0] == "clear-cache" {
		rest := args[1:]
		if len(rest) > 0 && isHelpArg(rest[0]) {
			_ = writeTestHelp(w)
			return testreport.Run{}, nil
		}
		if err := runTestClearCache(rest, w); err != nil {
			return testreport.Run{}, err
		}
		return testreport.Run{}, nil
	}
	if len(args) > 0 && args[0] == "daemon" {
		rest := args[1:]
		if len(rest) > 0 && isHelpArg(rest[0]) {
			_ = writeTestHelp(w)
			return testreport.Run{}, nil
		}
		if err := runTestDaemonCommand(ctx, rest, w); err != nil {
			return testreport.Run{}, err
		}
		return testreport.Run{}, nil
	}
	if len(args) > 0 && args[0] == "changed" {
		rest := args[1:]
		if len(rest) > 0 && isHelpArg(rest[0]) {
			_ = writeTestHelp(w)
			return testreport.Run{}, nil
		}
		rewritten, err := rewriteChangedTestArgs(rest)
		if err != nil {
			return testreport.Run{}, err
		}
		return runTest(ctx, rewritten, w, progressW)
	}
	if len(args) > 0 && args[0] == "failed" {
		rest := args[1:]
		if len(rest) > 0 && isHelpArg(rest[0]) {
			_ = writeTestHelp(w)
			return testreport.Run{}, nil
		}
		rewritten := append([]string{}, rest...)
		rewritten = append(rewritten, "--last-failed")
		return runTest(ctx, rewritten, w, progressW)
	}
	if len(args) > 0 && args[0] == "serve" {
		rest := args[1:]
		if len(rest) > 0 && isHelpArg(rest[0]) {
			_ = writeTestHelp(w)
			return testreport.Run{}, nil
		}
		if err := runTestServe(ctx, rest, progressW); err != nil {
			return testreport.Run{}, err
		}
		return testreport.Run{}, nil
	}

	root := "."
	filter := ""
	className := ""
	methodName := ""
	classFile := ""
	var selectedClasses []string
	shardCount := 0
	shardIndex := 0
	shardIndexSet := false
	durationHistoryPath := ""
	writeClassShardsDir := ""
	format := "console"
	junitPath := ""
	limitMode := vm.LimitMode("")
	limitProfile := ""
	var limitCaps vm.LimitCaps
	limitCapsSet := false
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
	lastFailed := false
	wizard := false
	cpuProfilePath := ""
	memProfilePath := ""
	perfJSONPath := ""
	tracePath := ""
	servicesPath := ""
	debounce := watch.DefaultDebounce
	backend := watch.BackendAuto
	var compileDoneMS int64
	var perfRunStarted time.Time
	var perfSnapshot apextest.PerfCounters
	var historyWriteNS int64
	parsed, err := flagparse.New("glade test").
		String("project", "p").
		String("filter", "f").
		String("class", "").
		String("method", "").
		String("class-file", "").
		String("shard-count", "").
		String("shard-index", "").
		String("duration-history", "").
		String("write-class-shards", "").
		Bool("json", "j").
		Bool("trace-blockers", "").
		String("slow-test-ms", "").
		String("changed-since", "c").
		Bool("parallel-methods", "").
		Bool("no-parallel-methods", "").
		String("parallelism", "").
		String("test-timeout", "").
		Bool("gc-aggressive", "").
		String("cpu-profile", "").
		String("mem-profile", "").
		String("perf-json", "").
		String("trace", "").
		String("services", "").
		String("junit", "").
		String("limit-mode", "").
		String("limit-profile", "").
		String("limit-queries", "").
		String("limit-query-rows", "").
		String("limit-dml-statements", "").
		String("limit-dml-rows", "").
		String("limit-heap-size", "").
		String("limit-cpu-ms", "").
		String("limit-callouts", "").
		String("limit-email-invocations", "").
		String("limit-async-jobs", "").
		String("limit-future-calls", "").
		String("limit-queueable-jobs", "").
		String("limit-batch-jobs", "").
		String("limit-scheduled-jobs", "").
		String("limit-sosl-queries", "").
		String("limit-query-locator-rows", "").
		String("limit-run-as", "").
		String("limit-savepoints", "").
		String("limit-savepoint-rollbacks", "").
		String("limit-publish-immediate-dml", "").
		Bool("watch", "").
		Bool("watch-once", "").
		Bool("daemon", "").
		Bool("connect", "").
		Bool("no-serve", "").
		Bool("no-cache", "").
		Bool("last-failed", "").
		Bool("wizard", "").
		Bool("debug", "").
		Bool("progress", "").
		Bool("progress-json", "").
		Bool("no-progress", "").
		Bool("quiet", "q").
		String("debounce", "").
		String("watch-backend", "").
		Parse(args)
	if err != nil {
		return testreport.Run{}, err
	}
	if parsed.String("project") != "" {
		root = parsed.String("project")
	}
	filter = parsed.String("filter")
	className = strings.TrimSpace(parsed.String("class"))
	methodName = strings.TrimSpace(parsed.String("method"))
	classFile = strings.TrimSpace(parsed.String("class-file"))
	durationHistoryPath = strings.TrimSpace(parsed.String("duration-history"))
	writeClassShardsDir = strings.TrimSpace(parsed.String("write-class-shards"))
	if className != "" && classFile != "" {
		return testreport.Run{}, errors.New("--class and --class-file cannot be combined")
	}
	if methodName != "" && className == "" {
		return testreport.Run{}, errors.New("--method requires --class")
	}
	if classFile != "" {
		selectedClasses, err = readTestClassFile(classFile)
		if err != nil {
			return testreport.Run{}, err
		}
	}
	if className != "" {
		selectedClasses = []string{className}
	}
	if parsed.String("shard-count") != "" {
		parsedShardCount, err := strconv.Atoi(parsed.String("shard-count"))
		if err != nil || parsedShardCount <= 0 {
			return testreport.Run{}, errors.New("--shard-count must be a positive integer")
		}
		if parsedShardCount > testdaemon.MaxClassShardCountV1 {
			return testreport.Run{}, fmt.Errorf("--shard-count cannot exceed %d", testdaemon.MaxClassShardCountV1)
		}
		shardCount = parsedShardCount
	}
	if parsed.String("shard-index") != "" {
		parsedShardIndex, err := strconv.Atoi(parsed.String("shard-index"))
		if err != nil || parsedShardIndex < 0 {
			return testreport.Run{}, errors.New("--shard-index must be a non-negative integer")
		}
		shardIndex = parsedShardIndex
		shardIndexSet = true
	}
	if shardIndexSet && shardCount == 0 {
		return testreport.Run{}, errors.New("--shard-index requires --shard-count")
	}
	if shardCount > 0 && shardIndex >= shardCount {
		return testreport.Run{}, fmt.Errorf("--shard-index must be between 0 and %d", shardCount-1)
	}
	if parsed.Bool("json") {
		format = "json"
		progressMode = cliui.ProgressOff
	}
	traceBlocked = parsed.Bool("trace-blockers")
	if parsed.String("slow-test-ms") != "" {
		parsedMS, err := strconv.ParseInt(parsed.String("slow-test-ms"), 10, 64)
		if err != nil || parsedMS < 0 {
			return testreport.Run{}, fmt.Errorf("--slow-test-ms must be a non-negative integer")
		}
		slowTestThresholdMS = parsedMS
	}
	changedSince = parsed.String("changed-since")
	if parsed.Bool("no-parallel-methods") {
		parallelMethods = false
	} else if parsed.Bool("parallel-methods") {
		parallelMethods = true
	}
	if parsed.String("parallelism") != "" {
		parsedParallelism, err := strconv.Atoi(parsed.String("parallelism"))
		if err != nil || parsedParallelism < 0 {
			return testreport.Run{}, fmt.Errorf("--parallelism must be a non-negative integer")
		}
		parallelismOverride = parsedParallelism
	}
	if parsed.String("test-timeout") != "" {
		parsedTimeout, err := time.ParseDuration(parsed.String("test-timeout"))
		if err != nil {
			return testreport.Run{}, fmt.Errorf("--test-timeout: %w", err)
		}
		if parsedTimeout < 0 {
			return testreport.Run{}, errors.New("--test-timeout must be non-negative")
		}
		testTimeout = parsedTimeout
	}
	gcAggressive = parsed.Bool("gc-aggressive")
	cpuProfilePath = parsed.String("cpu-profile")
	memProfilePath = parsed.String("mem-profile")
	perfJSONPath = parsed.String("perf-json")
	tracePath = strings.TrimSpace(parsed.String("trace"))
	servicesPath = strings.TrimSpace(parsed.String("services"))
	junitPath = parsed.String("junit")
	progressMode = progressModeFromArgs(args, progressMode)
	if servicesPath != "" {
		if err := enterprise.ValidateServiceConfig(servicesPath); err != nil {
			return testreport.Run{}, err
		}
		cliui.NewRenderer(cliui.RendererOptions{Stderr: progressW, Mode: progressMode}).Render(cliui.Event{
			Kind:   cliui.EventInfo,
			Phase:  "test",
			Label:  "services config validated",
			Detail: "runtime virtualization hooks are not enabled yet",
		})
	}
	if parsed.String("limit-mode") != "" {
		mode, err := parseLimitMode(parsed.String("limit-mode"))
		if err != nil {
			return testreport.Run{}, err
		}
		limitMode = mode
	}
	limitProfile = strings.TrimSpace(parsed.String("limit-profile"))
	limitCaps, limitCapsSet, err = parseLimitCapsFromFlags(limitProfile, parsed.String)
	if err != nil {
		return testreport.Run{}, err
	}
	watchMode = parsed.Bool("watch")
	if parsed.Bool("watch-once") {
		watchMode = true
		watchOnce = true
	}
	if tracePath != "" && watchMode {
		return testreport.Run{}, errors.New("--trace cannot be combined with --watch or --watch-once")
	}
	daemonMode = parsed.Bool("daemon")
	connectMode = parsed.Bool("connect")
	noServe = parsed.Bool("no-serve")
	noCache = parsed.Bool("no-cache")
	lastFailed = parsed.Bool("last-failed")
	wizard = parsed.Bool("wizard")
	debug = parsed.Bool("debug")
	if parsed.String("debounce") != "" {
		parsedDebounce, err := time.ParseDuration(parsed.String("debounce"))
		if err != nil {
			return testreport.Run{}, err
		}
		debounce = parsedDebounce
	}
	if parsed.String("watch-backend") != "" {
		parsedBackend, err := parseWatchBackend(parsed.String("watch-backend"))
		if err != nil {
			return testreport.Run{}, err
		}
		backend = parsedBackend
	}
	perfEnabled := strings.TrimSpace(perfJSONPath) != ""
	if perfEnabled {
		switch {
		case connectMode:
			return testreport.Run{}, errors.New("--perf-json cannot be combined with --connect")
		case daemonMode:
			return testreport.Run{}, errors.New("--perf-json cannot be combined with --daemon")
		case watchOnce:
			return testreport.Run{}, errors.New("--perf-json cannot be combined with --watch-once")
		case watchMode:
			return testreport.Run{}, errors.New("--perf-json cannot be combined with --watch")
		case wizard:
			return testreport.Run{}, errors.New("--perf-json cannot be combined with --wizard")
		case lastFailed:
			return testreport.Run{}, errors.New("--perf-json cannot be combined with --last-failed")
		case writeClassShardsDir != "":
			return testreport.Run{}, errors.New("--perf-json cannot be combined with --write-class-shards")
		}
		noServe = true
	}
	if wizard {
		if err := writeTestWizard(ctx, root, w); err != nil {
			return testreport.Run{}, err
		}
		return testreport.Run{}, nil
	}
	if lastFailed {
		if strings.TrimSpace(filter) != "" {
			return testreport.Run{}, errors.New("--last-failed cannot be combined with --filter")
		}
		failures, err := readLastFailedTests(root)
		if err != nil {
			return testreport.Run{}, err
		}
		if len(failures) == 0 {
			fmt.Fprint(w, "No failed tests recorded.\n")
			return testreport.Run{}, nil
		}
		filter = strings.Join(failures, ",")
	}
	durationHistoryExplicit := durationHistoryPath != ""
	defaultDurationHistoryPath := defaultCLIDurationHistoryPath(root)
	if durationHistoryPath == "" {
		if _, err := os.Stat(defaultDurationHistoryPath); err == nil {
			durationHistoryPath = defaultDurationHistoryPath
		}
	}
	durationHistoryWritePath := ""
	if !durationHistoryExplicit && shouldPersistCLIDurationHistory(filter, selectedClasses, methodName, changedSince, shardCount, writeClassShardsDir) {
		durationHistoryWritePath = defaultDurationHistoryPath
	}
	historyDestination := cliArtifactDestination{}
	switch {
	case durationHistoryExplicit:
		historyDestination = cliArtifactDestination{Name: "--duration-history", Path: durationHistoryPath}
	case durationHistoryWritePath != "":
		historyDestination = cliArtifactDestination{Name: "automatic duration history", Path: durationHistoryWritePath}
	case durationHistoryPath != "":
		historyDestination = cliArtifactDestination{Name: "automatic duration history", Path: durationHistoryPath}
	}
	if err := validateDistinctCLIArtifactDestinations(
		cliArtifactDestination{Name: "--perf-json", Path: strings.TrimSpace(perfJSONPath)},
		cliArtifactDestination{Name: "--cpu-profile", Path: strings.TrimSpace(cpuProfilePath)},
		cliArtifactDestination{Name: "--mem-profile", Path: strings.TrimSpace(memProfilePath)},
		cliArtifactDestination{Name: "--trace", Path: tracePath},
		cliArtifactDestination{Name: "--junit", Path: junitPath},
		historyDestination,
	); err != nil {
		return testreport.Run{}, err
	}
	if connectMode {
		switch {
		case watchMode:
			return testreport.Run{}, errors.New("--connect cannot be combined with --watch or --watch-once")
		case daemonMode:
			return testreport.Run{}, errors.New("--connect cannot be combined with --daemon")
		case debug:
			return testreport.Run{}, errors.New("--connect cannot be combined with --debug")
		case strings.TrimSpace(cpuProfilePath) != "":
			return testreport.Run{}, errors.New("--connect cannot be combined with --cpu-profile")
		case strings.TrimSpace(memProfilePath) != "":
			return testreport.Run{}, errors.New("--connect cannot be combined with --mem-profile")
		case strings.TrimSpace(servicesPath) != "":
			return testreport.Run{}, errors.New("--connect cannot be combined with --services")
		case noServe:
			return testreport.Run{}, errors.New("--connect cannot be combined with --no-serve")
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
		SelectedClasses:     selectedClasses,
		SelectedMethod:      methodName,
		LimitMode:           limitMode,
		LimitCaps:           limitCaps,
		LimitCapsSet:        limitCapsSet,
		TraceBlocked:        traceBlocked,
		TraceAll:            tracePath != "",
		SlowTestThresholdMS: slowTestThresholdMS,
		ParallelMethods:     parallelMethods,
		TimeoutMS:           testTimeout.Milliseconds(),
		NoDiskCache:         noCache,
		PerfCounters:        perfEnabled,
	}
	durationHistory, err := loadCLIDurationHistory(durationHistoryPath)
	if err != nil {
		return testreport.Run{}, err
	}
	testOpts.ClassDurationMS = durationHistory.Classes
	testOpts.MethodDurationMS = durationHistory.Methods
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
	default:
		testOpts.Parallelism = 1
	}
	effectiveShardCount := shardCount
	if writeClassShardsDir != "" && effectiveShardCount <= 0 {
		effectiveShardCount = testOpts.Parallelism
	}
	if writeClassShardsDir != "" && effectiveShardCount > testdaemon.MaxClassShardCountV1 {
		return testreport.Run{}, fmt.Errorf("test shard count cannot exceed %d", testdaemon.MaxClassShardCountV1)
	}
	applyTestMemoryLimits(gcAggressive)

	if noCache && !connectMode {
		noServe = true
	}
	localOnlyTestOptions := perfEnabled || strings.TrimSpace(cpuProfilePath) != "" || strings.TrimSpace(memProfilePath) != "" || servicesPath != ""
	if localOnlyTestOptions {
		noServe = true
	}
	if !noServe && !watchMode && !daemonMode && !debug {
		request := testdaemon.NewRunRequestV1(testOpts, changedSince, effectiveShardCount, shardIndex, writeClassShardsDir != "")
		if response, used, err := tryTestServerRequest(ctx, root, connectMode, request); used || err != nil {
			if err != nil {
				return testreport.Run{}, err
			}
			if !response.OK {
				message := strings.TrimSpace(response.Error)
				if message == "" {
					message = "test server run failed"
				}
				return testreport.Run{}, errors.New(message)
			}
			if writeClassShardsDir != "" && (response.Run == nil || response.Run.Summary().Total == 0) {
				if response.ShardPlan == nil {
					return testreport.Run{}, errors.New("test server did not return a class shard plan")
				}
				return testreport.Run{}, writeCLIClassShardPlan(writeClassShardsDir, *response.ShardPlan, effectiveShardCount)
			}
			if response.Run == nil {
				message := strings.TrimSpace(response.Error)
				if message == "" {
					message = "test server run failed"
				}
				return testreport.Run{}, errors.New(message)
			}
			result := *response.Run
			if err := maybeWriteCLIDurationHistory(durationHistoryWritePath, result, durationHistory); err != nil {
				return result, err
			}
			if tracePath != "" {
				if err := writeTestTraceFile(tracePath, result); err != nil {
					return result, err
				}
			}
			if err := writeLastFailedTests(root, result); err != nil {
				return result, err
			}
			if junitPath != "" {
				if err := writeJUnitFile(junitPath, result); err != nil {
					return result, err
				}
			}
			switch format {
			case "json":
				return result, writeTestJSONEnvelope(w, result, junitPath)
			default:
				return result, testreport.WriteConsole(w, result)
			}
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
		request := testdaemon.NewRunRequestV1(testOpts, changedSince, effectiveShardCount, shardIndex, writeClassShardsDir != "")
		var setProgressTotal func(int)
		if progressReporter != nil {
			setProgressTotal = progressReporter.setTotal
		}
		result, _, shardPlan, err := daemon.RunRequestV1(ctx, request, testOpts.Progress, setProgressTotal)
		if err != nil {
			return testreport.Run{}, err
		}
		if writeClassShardsDir != "" && result.Summary().Total == 0 {
			if shardPlan == nil {
				return testreport.Run{}, errors.New("test daemon did not return a class shard plan")
			}
			return testreport.Run{}, writeCLIClassShardPlan(writeClassShardsDir, *shardPlan, effectiveShardCount)
		}
		if err := writeLastFailedTests(root, result); err != nil {
			return result, err
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
		if err := maybeWriteRunPerfJSON(perfJSONPath, root, result, cpuProfilePath, memProfilePath, apextest.PerfCounters{}, 0, 0); err != nil {
			return result, err
		}
		if err := maybeWriteCLIDurationHistory(durationHistoryWritePath, result, durationHistory); err != nil {
			return result, err
		}
		if tracePath != "" {
			if err := writeTestTraceFile(tracePath, result); err != nil {
				return result, err
			}
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
			return result, writeTestJSONEnvelope(w, result, junitPath)
		default:
			return result, testreport.WriteConsole(w, result)
		}
	}
	if perfEnabled {
		perfRunStarted = time.Now()
	}
	loadedProject, index, preRunPhases, err := loadTestIndexWithPerfPhases(root, perfEnabled)
	if err != nil {
		return testreport.Run{}, err
	}
	if perfEnabled {
		testOpts.PreRunPhaseDurations = preRunPhases
	}
	if progressReporter != nil && len(index.Project.Root) > 0 {
		progressReporter.warn("startup cache: " + testStartupCacheStatus(index.Project.Root))
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
		return runWatchTests(ctx, root, loadedProject, index, testOpts, watch.Config{Root: root, Debounce: debounce, Backend: backend}, watchOnce, w)
	}
	var result testreport.Run
	if strings.TrimSpace(changedSince) != "" {
		var discoverStarted time.Time
		if perfEnabled {
			discoverStarted = time.Now()
		}
		cases := apextest.Discover(index, testOpts)
		if perfEnabled {
			testOpts.PreRunPhaseDurations.Discover = time.Since(discoverStarted)
			perfSnapshot = preRunPerfSnapshot(testOpts.PreRunPhaseDurations)
		}
		selectorOpts := testOpts
		selectorOpts.Filter = ""
		selectorCases := cases
		if strings.TrimSpace(testOpts.Filter) != "" {
			selectorCases = apextest.Discover(index, selectorOpts)
		}
		if selectorRun, ok := exactTestSelectorFailureRun(selectorCases, className, methodName, func() []apextest.TestCase {
			classOpts := selectorOpts
			classOpts.SelectedMethod = ""
			return apextest.Discover(index, classOpts)
		}); ok {
			result = selectorRun
		} else {
			selection, err := changedSinceSelection(root, index, changedSince)
			if err != nil {
				return testreport.Run{}, err
			}
			cases = filterSelectedTestCases(cases, selection)
			if strings.TrimSpace(writeClassShardsDir) != "" {
				return testreport.Run{}, writeCLIClassShards(writeClassShardsDir, cases, durationHistoryPath, effectiveShardCount)
			}
			cases, err = applyCLIClassShard(cases, durationHistoryPath, shardCount, shardIndex)
			if err != nil {
				return testreport.Run{}, err
			}
			if progressReporter != nil {
				progressReporter.setTotal(len(cases))
			}
			result = apextest.RunCasesContext(ctx, index, testOpts, cases)
			if perfEnabled {
				perfSnapshot = apextest.SnapshotPerfCounters()
			}
		}
	} else {
		var discoverStarted time.Time
		if perfEnabled {
			discoverStarted = time.Now()
		}
		cases := apextest.Discover(index, testOpts)
		if perfEnabled {
			testOpts.PreRunPhaseDurations.Discover = time.Since(discoverStarted)
			perfSnapshot = preRunPerfSnapshot(testOpts.PreRunPhaseDurations)
		}
		selectorOpts := testOpts
		selectorOpts.Filter = ""
		selectorCases := cases
		if strings.TrimSpace(testOpts.Filter) != "" {
			selectorCases = apextest.Discover(index, selectorOpts)
		}
		if selectorRun, ok := exactTestSelectorFailureRun(selectorCases, className, methodName, func() []apextest.TestCase {
			classOpts := selectorOpts
			classOpts.SelectedMethod = ""
			return apextest.Discover(index, classOpts)
		}); ok {
			result = selectorRun
		} else {
			if strings.TrimSpace(writeClassShardsDir) != "" {
				return testreport.Run{}, writeCLIClassShards(writeClassShardsDir, cases, durationHistoryPath, effectiveShardCount)
			}
			cases, err = applyCLIClassShard(cases, durationHistoryPath, shardCount, shardIndex)
			if err != nil {
				return testreport.Run{}, err
			}
			if progressReporter != nil {
				progressReporter.setTotal(len(cases))
			}
			result = apextest.RunCasesContext(ctx, index, testOpts, cases)
			if perfEnabled {
				perfSnapshot = apextest.SnapshotPerfCounters()
			}
		}
	}
	if progressReporter != nil {
		result.DurationMS = time.Since(progressReporter.started).Milliseconds()
	}
	if progressReporter != nil && compileDoneMS > 10000 && index.Project.Root != "" && index.Project.Root != "." {
		progressReporter.warn(fmt.Sprintf("compile test harness took %dms", compileDoneMS))
		progressReporter.warn(fmt.Sprintf("for this workspace, try: --project %s", index.Project.Root))
	}
	if progressReporter != nil {
		progressReporter.finish()
	}
	if !perfEnabled && stopProfile != nil {
		stop := stopProfile
		stopProfile = nil
		if stopErr := stop(); stopErr != nil {
			return result, stopErr
		}
	}
	result.Dependencies = append(result.Dependencies, index.Dependencies...)
	var historyStarted time.Time
	if perfEnabled && durationHistoryWritePath != "" && result.Summary().Total > 0 {
		historyStarted = time.Now()
	}
	if err := maybeWriteCLIDurationHistory(durationHistoryWritePath, result, durationHistory); err != nil {
		return result, err
	}
	if !historyStarted.IsZero() {
		historyWriteNS = time.Since(historyStarted).Nanoseconds()
	}
	var totalMS int64
	if perfEnabled {
		totalMS = time.Since(perfRunStarted).Milliseconds()
	}
	if perfEnabled && stopProfile != nil {
		stop := stopProfile
		stopProfile = nil
		if stopErr := stop(); stopErr != nil {
			return result, stopErr
		}
	}
	if perfEnabled {
		if err := maybeWriteRunPerfJSON(perfJSONPath, root, result, cpuProfilePath, memProfilePath, perfSnapshot, totalMS, historyWriteNS); err != nil {
			return result, err
		}
	}
	if tracePath != "" {
		if err := writeTestTraceFile(tracePath, result); err != nil {
			return result, err
		}
	}
	if debug {
		return result, serveDAPSnapshot(testRunSnapshot(result), w)
	}
	if err := writeLastFailedTests(root, result); err != nil {
		return result, err
	}
	if junitPath != "" {
		if err := writeJUnitFile(junitPath, result); err != nil {
			return result, err
		}
	}
	switch format {
	case "json":
		return result, writeTestJSONEnvelope(w, result, junitPath)
	default:
		return result, testreport.WriteConsole(w, result)
	}
}

func loadTestIndexWithPerfPhases(root string, enabled bool) (project.Project, typesys.Index, apextest.PreRunPhaseDurations, error) {
	if !enabled {
		p, index, err := loadProjectIndex(root)
		return p, index, apextest.PreRunPhaseDurations{}, err
	}
	var phases apextest.PreRunPhaseDurations
	started := time.Now()
	p, err := project.Load(root)
	phases.ProjectLoad = time.Since(started)
	if err != nil {
		return project.Project{}, typesys.Index{}, phases, err
	}
	started = time.Now()
	s, schemaErr := gladeschema.LoadProject(p)
	phases.SchemaLoad = time.Since(started)
	started = time.Now()
	if schemaErr != nil {
		index := typesys.Build(p, gladeschema.Schema{})
		phases.IndexBuild = time.Since(started)
		index.Diagnostics = append(index.Diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESCHEMA001",
			Message:  fmt.Sprintf("metadata schema load failed: %v", schemaErr),
		})
		return p, index, phases, nil
	}
	index := typesys.Build(p, s)
	phases.IndexBuild = time.Since(started)
	return p, index, phases, nil
}

func preRunPerfSnapshot(phases apextest.PreRunPhaseDurations) apextest.PerfCounters {
	return apextest.PerfCounters{
		Enabled: true,
		Phases: apextest.RunnerPhasePerfCounters{
			ProjectLoadNS: phases.ProjectLoad.Nanoseconds(),
			SchemaLoadNS:  phases.SchemaLoad.Nanoseconds(),
			IndexBuildNS:  phases.IndexBuild.Nanoseconds(),
			DiscoverNS:    phases.Discover.Nanoseconds(),
		},
	}
}

func writeTestJSONEnvelope(w io.Writer, result testreport.Run, junitPath string) error {
	summary := result.Summary()
	ok := summary.Failed == 0 && summary.Errors == 0
	artifacts := []any{}
	if strings.TrimSpace(junitPath) != "" {
		artifacts = append(artifacts, map[string]string{"kind": "junit", "path": junitPath})
	}
	return writeCLIJSONEnvelope(w, cliJSONEnvelope{
		Command:     "test",
		Status:      statusForOK(ok),
		ExitCode:    exitCodeForOK(ok),
		Summary:     summary,
		Tests:       flattenTestCases(result),
		Artifacts:   artifacts,
		Suggestions: testreport.NextCommands(summary),
		Data:        result,
	})
}

func exactTestSelectorFailureRun(cases []apextest.TestCase, className, methodName string, discoverClassCases func() []apextest.TestCase) (testreport.Run, bool) {
	className = strings.TrimSpace(className)
	methodName = strings.TrimSpace(methodName)
	if className == "" {
		return testreport.Run{}, false
	}
	if len(cases) > 0 {
		return testreport.Run{}, false
	}
	if methodName != "" && len(discoverClassCases()) > 0 {
		return selectorFailureRun(
			"missing test method",
			fmt.Sprintf("no test method matched --class %q --method %q", className, methodName),
			fmt.Sprintf("Glade found test class %q, but no exact test method named %q.", className, methodName),
		), true
	}
	return selectorFailureRun(
		"missing test class",
		fmt.Sprintf("no test class matched --class %q", className),
		fmt.Sprintf("Glade did not discover an exact test class named %q.", className),
	), true
}

func selectorFailureRun(name, message, detail string) testreport.Run {
	return testreport.Run{
		Name: "glade test",
		Suites: []testreport.Suite{{
			Name: "test selector",
			Cases: []testreport.Case{{
				Name:   name,
				Status: testreport.StatusRuntimeError,
				Problem: &testreport.Problem{
					Type:    "Selector",
					Message: message,
					Detail:  detail,
				},
			}},
		}},
	}
}

func flattenTestCases(result testreport.Run) []map[string]any {
	out := []map[string]any{}
	for _, suite := range result.Suites {
		for _, testCase := range suite.Cases {
			name := testCase.Name
			if name == "" {
				switch {
				case testCase.ClassName != "" && testCase.MethodName != "":
					name = testCase.ClassName + "." + testCase.MethodName
				case testCase.ClassName != "":
					name = testCase.ClassName
				case testCase.MethodName != "":
					name = testCase.MethodName
				default:
					name = suite.Name
				}
			}
			row := map[string]any{
				"name":       name,
				"className":  testCase.ClassName,
				"methodName": testCase.MethodName,
				"status":     testCase.Status,
				"durationMs": testCase.DurationMS,
			}
			if testCase.Problem != nil {
				row["problem"] = testCase.Problem
			}
			out = append(out, row)
		}
	}
	return out
}

func writeTestTraceFile(path string, result testreport.Run) error {
	if path == "" {
		return nil
	}
	var events []trace.Event
	for _, suite := range result.Suites {
		for _, testCase := range suite.Cases {
			events = append(events, testCase.Trace...)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return trace.WriteJSON(file, trace.NewDocument(events))
}

func readTestClassFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read --class-file: %w", err)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

type cliClassShard struct {
	Index           int
	TotalDurationMS int64
	Classes         []string
}

func applyCLIClassShard(cases []apextest.TestCase, historyPath string, shardCount, shardIndex int) ([]apextest.TestCase, error) {
	if shardCount <= 0 {
		return cases, nil
	}
	durations, err := loadCLIClassDurationHistory(historyPath)
	if err != nil {
		return nil, err
	}
	shards := planCLIClassShards(cases, durations, shardCount)
	selected := map[string]bool{}
	for _, className := range shards[shardIndex].Classes {
		selected[className] = true
	}
	out := make([]apextest.TestCase, 0, len(cases))
	for _, testCase := range cases {
		if selected[testCase.ClassName] {
			out = append(out, testCase)
		}
	}
	return out, nil
}

func writeCLIClassShards(dir string, cases []apextest.TestCase, historyPath string, shardCount int) error {
	if shardCount < 1 || shardCount > testdaemon.MaxClassShardCountV1 {
		return fmt.Errorf("test shard count must be between 1 and %d", testdaemon.MaxClassShardCountV1)
	}
	durations, err := loadCLIClassDurationHistory(historyPath)
	if err != nil {
		return err
	}
	return writeCLIClassShardPlan(dir, testdaemon.PlanClassShards(cases, durations, shardCount), shardCount)
}

func writeCLIClassShardPlan(dir string, plan testdaemon.ClassShardPlanV1, expectedCount int) error {
	if err := validateCLIClassShardPlan(plan, expectedCount); err != nil {
		return err
	}
	shards := plan.Shards
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	width := len(strconv.Itoa(len(shards) - 1))
	if width < 3 {
		width = 3
	}
	for _, shard := range shards {
		classes := append([]string(nil), shard.Classes...)
		sort.Strings(classes)
		data := strings.Join(classes, "\n")
		if data != "" {
			data += "\n"
		}
		path := filepath.Join(dir, fmt.Sprintf("shard-%0*d.txt", width, shard.Index))
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func validateCLIClassShardPlan(plan testdaemon.ClassShardPlanV1, expectedCount int) error {
	if expectedCount < 1 || expectedCount > testdaemon.MaxClassShardCountV1 {
		return fmt.Errorf("test shard plan count must be between 1 and %d", testdaemon.MaxClassShardCountV1)
	}
	if len(plan.Shards) != expectedCount {
		return fmt.Errorf("test shard plan contains %d shards; expected %d", len(plan.Shards), expectedCount)
	}
	seen := make([]bool, expectedCount)
	for _, shard := range plan.Shards {
		if shard.Index < 0 || shard.Index >= expectedCount {
			return fmt.Errorf("test shard plan index %d is outside 0..%d", shard.Index, expectedCount-1)
		}
		if seen[shard.Index] {
			return fmt.Errorf("test shard plan contains duplicate index %d", shard.Index)
		}
		seen[shard.Index] = true
	}
	return nil
}

func planCLIClassShards(cases []apextest.TestCase, durations map[string]int64, shardCount int) []cliClassShard {
	plan := testdaemon.PlanClassShards(cases, durations, shardCount)
	shards := make([]cliClassShard, 0, len(plan.Shards))
	for _, shard := range plan.Shards {
		shards = append(shards, cliClassShard{
			Index:           shard.Index,
			TotalDurationMS: shard.TotalDurationMS,
			Classes:         append([]string(nil), shard.Classes...),
		})
	}
	return shards
}

type cliDurationHistory struct {
	Classes map[string]int64
	Methods map[string]int64
}

func loadCLIClassDurationHistory(path string) (map[string]int64, error) {
	history, err := loadCLIDurationHistory(path)
	if err != nil {
		return nil, err
	}
	return history.Classes, nil
}

func loadCLIDurationHistory(path string) (cliDurationHistory, error) {
	if strings.TrimSpace(path) == "" {
		return cliDurationHistory{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cliDurationHistory{}, err
	}
	var perf struct {
		ClassDurations  map[string]int64 `json:"classDurations"`
		MethodDurations map[string]int64 `json:"methodDurations"`
		TopSlowClasses  []struct {
			Class      string `json:"class"`
			DurationMS int64  `json:"durationMs"`
		} `json:"topSlowClasses"`
	}
	if err := json.Unmarshal(data, &perf); err != nil {
		return cliDurationHistory{}, err
	}
	out := cliDurationHistory{
		Classes: cleanDurationMap(perf.ClassDurations),
		Methods: cleanDurationMap(perf.MethodDurations),
	}
	for _, class := range perf.TopSlowClasses {
		if strings.TrimSpace(class.Class) != "" && class.DurationMS > 0 {
			if out.Classes == nil {
				out.Classes = map[string]int64{}
			}
			out.Classes[class.Class] = class.DurationMS
		}
	}
	if len(out.Classes) != 0 || len(out.Methods) != 0 {
		return out, nil
	}
	var direct map[string]int64
	if err := json.Unmarshal(data, &direct); err == nil {
		for className, durationMS := range direct {
			if strings.TrimSpace(className) != "" && durationMS > 0 {
				if out.Classes == nil {
					out.Classes = map[string]int64{}
				}
				out.Classes[className] = durationMS
			}
		}
	}
	return out, nil
}

func defaultCLIDurationHistoryPath(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	return filepath.Join(root, ".glade", "test-durations.json")
}

func shouldPersistCLIDurationHistory(filter string, selectedClasses []string, methodName, changedSince string, shardCount int, writeClassShardsDir string) bool {
	if strings.TrimSpace(filter) != "" || len(selectedClasses) != 0 || strings.TrimSpace(methodName) != "" || strings.TrimSpace(changedSince) != "" {
		return false
	}
	if shardCount > 0 || strings.TrimSpace(writeClassShardsDir) != "" {
		return false
	}
	return true
}

func maybeWriteCLIDurationHistory(path string, result testreport.Run, previous cliDurationHistory) error {
	if strings.TrimSpace(path) == "" || result.Summary().Total == 0 {
		return nil
	}
	return writeCLIDurationHistory(path, result, previous)
}

func writeCLIDurationHistory(path string, result testreport.Run, previous cliDurationHistory) error {
	classDurations := mergeCLIDurationMap(previous.Classes, runClassDurations(result))
	methodDurations := mergeCLIDurationMap(previous.Methods, runMethodDurations(result))
	payload := struct {
		ClassDurations  map[string]int64 `json:"classDurations,omitempty"`
		MethodDurations map[string]int64 `json:"methodDurations,omitempty"`
	}{
		ClassDurations:  classDurations,
		MethodDurations: methodDurations,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func mergeCLIDurationMap(previous, observed map[string]int64) map[string]int64 {
	merged := cleanDurationMap(previous)
	for name, durationMS := range cleanDurationMap(observed) {
		if merged == nil {
			merged = map[string]int64{}
		}
		if old := merged[name]; old > 0 {
			merged[name] = mergeCLIDurationMS(old, durationMS)
			continue
		}
		merged[name] = durationMS
	}
	return merged
}

func mergeCLIDurationMS(previous, observed int64) int64 {
	if previous <= 0 {
		return observed
	}
	if observed <= 0 {
		return previous
	}
	return (previous*3 + observed) / 4
}

func cleanDurationMap(in map[string]int64) map[string]int64 {
	if len(in) == 0 {
		return nil
	}
	out := map[string]int64{}
	for name, durationMS := range in {
		name = strings.TrimSpace(name)
		if name != "" && durationMS > 0 {
			out[name] = durationMS
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func writeTestHelp(w io.Writer) error {
	body := strings.TrimSpace(`
Run local Apex tests.

Usage:
  glade test [--project <root>] [flags]
  glade test changed [--project <root>] [--since <ref>]
  glade test failed [--project <root>]
  glade test serve [--project <root>] [serve flags]
  glade test daemon status|stop [--project <root>]
  glade test clear-cache [--project <root>]

Common flags:
  --project <root>          Project root. Defaults to current directory.
  --filter <pattern>        Run matching test classes or methods.
  --class <name>            Run one exact test class.
  --method <name>           Run one exact test method. Requires --class.
  --class-file <path>       Read exact test class names, one per line.
  --shard-count <n>         Select one deterministic class shard.
  --shard-index <i>         Shard index to run, starting at 0.
  --duration-history <path> Weight shard planning with prior class durations.
  --write-class-shards <dir> Write class shard files and exit.
  --connect                 Require a running test server (see serve).
  --no-serve                Do not auto-connect to a running test server.
  --no-cache                Skip the on-disk startup cache for this run.
  --last-failed             Rerun tests that failed in the last completed run.
  --ui                      Open the TUI on the test board.
  --wizard                  Print daily test loop command suggestions.
  --daemon                  Keep index warm in-process for --watch loops.
  --json                    Write JSON test results.
  --junit <path>            Write JUnit XML results.
  --cpu-profile <path>      Write a CPU profile for this local one-shot run.
  --mem-profile <path>      Write a heap profile after this local one-shot run.
  --perf-json <path>        Write opt-in performance counters for this local one-shot run.
  --trace <path>            Write a Chrome trace JSON document for this run.
  --services <path>         Validate a services.yml virtualization config.
  --progress                Show progress on stderr; uses a progress bar on TTY and line output when redirected.
  --progress-json           Print NDJSON progress events to stderr.
  --no-progress, --quiet    Disable progress.
  --debug                   Run one DAP snapshot session after tests.
  --watch                   Watch source files and emit NDJSON events.
  --watch-once              Run one watch cycle and exit.
  --changed-since <ref>     Select tests affected since a git ref.
  --since <ref>             Git ref for glade test changed (default HEAD).
  --parallel-methods        Run test methods in parallel (default).
  --no-parallel-methods     Force serial method execution within a class.
  --parallelism <n>         Worker count (default: GOMAXPROCS).
  --test-timeout <dur>      Per-test timeout (default 5m, e.g. 30s, 2m).
  --limit-mode <mode>       Use strict or permissive governor limits.

Serve flags:
  --no-warm                 Start the server without warming the project.

Examples:
  glade test --project . --class AccountServiceTest
  glade test --project . --class AccountServiceTest --method testCreatesAccount
  glade test --project . --class-file tests.txt
  glade test --project . --shard-count 2 --shard-index 0
`)
	_, err := fmt.Fprintln(w, body)
	return err
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

func progressModeFromArgs(args []string, fallback cliui.ProgressMode) cliui.ProgressMode {
	progress := false
	progressJSON := false
	noProgress := false
	for _, arg := range args {
		name := arg
		if before, _, ok := strings.Cut(arg, "="); ok {
			name = before
		}
		switch name {
		case "--progress":
			progress = true
		case "--progress-json":
			progressJSON = true
		case "--no-progress", "--quiet", "-q":
			noProgress = true
		}
	}
	if !progress && !progressJSON && !noProgress {
		return fallback
	}
	return progressModeForFlags(false, progress, progressJSON, noProgress)
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
		OK:       ok,
		Label:    fmt.Sprintf("%d passed, %d failed, %d errors · %s", r.passed, r.failed, r.errors, elapsed),
		ExitCode: exitCodeForOK(ok),
	})
}

type runPerfSummary struct {
	SchemaVersion   string                `json:"schemaVersion"`
	Command         string                `json:"command"`
	GeneratedAt     string                `json:"generatedAt"`
	Project         string                `json:"project"`
	Status          string                `json:"status"`
	ExitCode        int                   `json:"exitCode"`
	DurationMS      int64                 `json:"durationMs"`
	DiscoverMS      int64                 `json:"discoverMs"`
	CompileMS       int64                 `json:"compileMs"`
	StartupMS       int64                 `json:"startupMs"`
	TotalMS         int64                 `json:"totalMs"`
	HistoryWriteNS  int64                 `json:"historyWriteNs"`
	Summary         testreport.Summary    `json:"summary"`
	ApexPerf        apextest.PerfCounters `json:"apexPerf"`
	ClassDurations  map[string]int64      `json:"classDurations,omitempty"`
	MethodDurations map[string]int64      `json:"methodDurations,omitempty"`
	TopSlowClasses  []runPerfClass        `json:"topSlowClasses,omitempty"`
	CPUProfilePath  string                `json:"cpuProfilePath,omitempty"`
	MemProfilePath  string                `json:"memProfilePath,omitempty"`
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

func maybeWriteRunPerfJSON(perfJSONPath, root string, result testreport.Run, cpuProfilePath, memProfilePath string, counters apextest.PerfCounters, totalMS, historyWriteNS int64) error {
	if strings.TrimSpace(perfJSONPath) == "" {
		return nil
	}
	absRoot, _ := filepath.Abs(root)
	summary := result.Summary()
	ok := summary.Failed == 0 && summary.Errors == 0
	phases := counters.Phases
	discoverMS := phases.DiscoverNS / int64(time.Millisecond)
	compileMS := (phases.ProjectCompileNS + phases.TestCompileNS) / int64(time.Millisecond)
	startupNS := phases.ProjectLoadNS + phases.SchemaLoadNS + phases.IndexBuildNS + phases.DiscoverNS + phases.RuntimeKeyNS +
		phases.CacheValidateNS + phases.CacheDecodeNS + phases.CacheEncodeNS + phases.OrgBuildNS + phases.ProjectCompileNS + phases.TestCompileNS
	perf := runPerfSummary{
		SchemaVersion:   "1.0",
		Command:         "test",
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		Project:         absRoot,
		Status:          statusForOK(ok),
		ExitCode:        exitCodeForOK(ok),
		DurationMS:      summary.DurationMS,
		DiscoverMS:      discoverMS,
		CompileMS:       compileMS,
		StartupMS:       startupNS / int64(time.Millisecond),
		TotalMS:         totalMS,
		HistoryWriteNS:  historyWriteNS,
		Summary:         summary,
		ApexPerf:        counters,
		ClassDurations:  runClassDurations(result),
		MethodDurations: runMethodDurations(result),
		CPUProfilePath:  strings.TrimSpace(cpuProfilePath),
		MemProfilePath:  strings.TrimSpace(memProfilePath),
	}
	perf.TopSlowClasses = runTopSlowClasses(result, 15)
	if err := os.MkdirAll(filepath.Dir(perfJSONPath), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(perf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(perfJSONPath, append(data, '\n'), 0o600)
}

func runClassDurations(result testreport.Run) map[string]int64 {
	out := map[string]int64{}
	for _, suite := range result.Suites {
		for _, testCase := range suite.Cases {
			className := strings.TrimSpace(testCase.ClassName)
			if className == "" {
				className = strings.TrimSpace(suite.Name)
			}
			if className != "" {
				out[className] += testCase.DurationMS
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func runMethodDurations(result testreport.Run) map[string]int64 {
	out := map[string]int64{}
	for _, suite := range result.Suites {
		for _, testCase := range suite.Cases {
			className := strings.TrimSpace(testCase.ClassName)
			if className == "" {
				className = strings.TrimSpace(suite.Name)
			}
			methodName := strings.TrimSpace(testCase.MethodName)
			if className == "" || methodName == "" {
				continue
			}
			out[className+"."+methodName] = testCase.DurationMS
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
