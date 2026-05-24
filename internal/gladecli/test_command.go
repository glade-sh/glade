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
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/compat"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
	"github.com/glade-sh/glade/internal/watch"
)

func runTest(ctx context.Context, args []string, w io.Writer) (testreport.Run, error) {
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
	traceBlocked := false
	slowTestThresholdMS := int64(0)
	changedSince := ""
	parallelMethods := false
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

	testOpts := apextest.Options{Filter: filter, LimitMode: limitMode, TraceBlocked: traceBlocked, SlowTestThresholdMS: slowTestThresholdMS, ParallelMethods: parallelMethods}
	if parallelMethods {
		testOpts.Parallelism = runtime.GOMAXPROCS(0)
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
		return runWatchTests(ctx, root, index, apextest.Options{Filter: filter, LimitMode: limitMode, TraceBlocked: traceBlocked, SlowTestThresholdMS: slowTestThresholdMS}, watch.Config{Root: root, Debounce: debounce, Backend: backend}, watchOnce, w)
	}
	var result testreport.Run
	if strings.TrimSpace(changedSince) != "" {
		cases := apextest.Discover(index, testOpts)
		selection, err := changedSinceSelection(root, index, changedSince)
		if err != nil {
			return testreport.Run{}, err
		}
		cases = filterSelectedTestCases(cases, selection)
		result = apextest.RunCasesContext(ctx, index, testOpts, cases)
	} else {
		result = apextest.Run(index, testOpts)
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
