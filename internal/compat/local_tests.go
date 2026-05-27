package compat

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
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/watch"
)

type LocalTestOptions struct {
	Project             string
	Class               string
	ClassList           []string
	ClassFile           string
	StartClass          string
	Method              string
	BlockersOnly        bool
	TraceBlocked        bool
	SlowTestThresholdMS int64
	TimeoutMS           int64
	TopFailures         int
	ProfileOnTimeout    bool
	Parallelism         int
	ProgressWriter      io.Writer
	ForceAnalysis       bool
	MaxFailureGroups    int
	ChangedSince        string
	ParallelMethods     bool
	CPUProfilePath      string
	MemProfilePath      string
	PerfJSONPath        string
	ShardCount          int
	ShardIndex          int
	WriteClassShards    string
	DurationHistoryPath string
	AutoTune            bool
	AutoShardCount      bool
	AutoShardIndex      bool
}

type LocalTestPhaseTiming struct {
	Name            string `json:"name"`
	DurationMS      int64  `json:"durationMs"`
	HeapAllocBytes  uint64 `json:"heapAllocBytes,omitempty"`
	TotalAllocBytes uint64 `json:"totalAllocBytes,omitempty"`
	Mallocs         uint64 `json:"mallocs,omitempty"`
	Frees           uint64 `json:"frees,omitempty"`
	NumGC           uint32 `json:"numGC,omitempty"`
}

type LocalTestReport struct {
	Target          string                   `json:"target"`
	Ready           bool                     `json:"ready"`
	Project         string                   `json:"project"`
	DurationMS      int64                    `json:"durationMs,omitempty"`
	CasesDiscovered int                      `json:"casesDiscovered,omitempty"`
	CasesRun        int                      `json:"casesRun,omitempty"`
	TriageStopped   bool                     `json:"triageStopped,omitempty"`
	Dependencies    []typesys.DependencyInfo `json:"dependencies,omitempty"`
	Selection       *watch.TestSelection     `json:"selection,omitempty"`
	Phases          []LocalTestPhaseTiming   `json:"phases,omitempty"`
	Summary         LocalTestSummary         `json:"summary"`
	Outcomes        []LocalTestOutcome       `json:"outcomes"`
	TopFailures     []LocalTestFailureGroup  `json:"topFailures,omitempty"`
	Diagnostics     []diagnostic.Diagnostic  `json:"diagnostics,omitempty"`
	Perf            *LocalTestPerfSummary    `json:"perf,omitempty"`
}

type LocalTestPerfSummary struct {
	GeneratedAt     string                 `json:"generatedAt"`
	Project         string                 `json:"project"`
	DurationMS      int64                  `json:"durationMs"`
	CasesDiscovered int                    `json:"casesDiscovered"`
	CasesRun        int                    `json:"casesRun"`
	Summary         LocalTestSummary       `json:"summary"`
	Phases          []LocalTestPhaseTiming `json:"phases,omitempty"`
	TopSlowClasses  []LocalTestPerfClass   `json:"topSlowClasses,omitempty"`
	TopCloneClasses []LocalTestCloneClass  `json:"topCloneClasses,omitempty"`
	CloneStats      LocalTestCloneStats    `json:"cloneStats"`
	CPUProfilePath  string                 `json:"cpuProfilePath,omitempty"`
	MemProfilePath  string                 `json:"memProfilePath,omitempty"`
}

type LocalTestPerfClass struct {
	Class      string `json:"class"`
	DurationMS int64  `json:"durationMs"`
	Tests      int    `json:"tests"`
}

type LocalTestCloneStats struct {
	CloneRuntimeOrgCalls       uint64 `json:"cloneRuntimeOrgCalls"`
	CloneRuntimeCalls          uint64 `json:"cloneRuntimeCalls"`
	CloneRollbackSnapshotCalls uint64 `json:"cloneRollbackSnapshotCalls"`
	JournalRollbacks           uint64 `json:"journalRollbacks,omitempty"`
	CloneFallbacks             uint64 `json:"cloneFallbacks,omitempty"`
}

type LocalTestCloneClass struct {
	Class       string `json:"class"`
	SetupClones uint64 `json:"setupClones,omitempty"`
	TestClones  uint64 `json:"testClones,omitempty"`
	DurationMS  int64  `json:"durationMs,omitempty"`
}

type LocalTestSummary struct {
	Total          int `json:"total"`
	Pass           int `json:"pass"`
	Fail           int `json:"fail"`
	Unsupported    int `json:"unsupported"`
	LoadErrors     int `json:"loadError"`
	CompileErrors  int `json:"compileError"`
	InternalErrors int `json:"internalError"`
	AssertFailures int `json:"assertFail,omitempty"`
	RuntimeGaps    int `json:"runtimeGap,omitempty"`
	CompileGaps    int `json:"compileGap,omitempty"`
	Timeouts       int `json:"timeout,omitempty"`
}

type LocalTestOutcome struct {
	ProjectLabel        string                 `json:"projectLabel"`
	Class               string                 `json:"class"`
	Method              string                 `json:"method"`
	Outcome             string                 `json:"outcome"`
	Phase               string                 `json:"phase,omitempty"`
	CapabilityID        string                 `json:"capabilityId,omitempty"`
	File                string                 `json:"file,omitempty"`
	Line                int                    `json:"line,omitempty"`
	TopFrame            *testreport.StackFrame `json:"topFrame,omitempty"`
	Error               string                 `json:"error,omitempty"`
	RelatedMetadataFile string                 `json:"relatedMetadataFile,omitempty"`
	DurationMS          int64                  `json:"durationMs,omitempty"`
	TraceEvents         int                    `json:"traceEvents,omitempty"`
	ProfileEvents       int                    `json:"profileEvents,omitempty"`
	ProfileCategories   map[string]int         `json:"profileCategories,omitempty"`
}

type LocalTestFailureGroup struct {
	Outcome      string   `json:"outcome"`
	Phase        string   `json:"phase,omitempty"`
	CapabilityID string   `json:"capabilityId,omitempty"`
	Error        string   `json:"error,omitempty"`
	Count        int      `json:"count"`
	Samples      []string `json:"samples,omitempty"`
}

type LocalTestCorpusBaseline struct {
	Target   string                   `json:"target"`
	Projects []LocalTestCorpusProject `json:"projects"`
}

type LocalTestCorpusProject struct {
	Project  string                     `json:"project"`
	Ready    bool                       `json:"ready"`
	Summary  LocalTestSummary           `json:"summary"`
	Outcomes []LocalTestExpectedOutcome `json:"outcomes"`
}

type LocalTestExpectedOutcome struct {
	Class        string `json:"class"`
	Method       string `json:"method"`
	Outcome      string `json:"outcome"`
	CapabilityID string `json:"capabilityId,omitempty"`
}

type LocalTestCorpusReport struct {
	Target   string                         `json:"target"`
	Ready    bool                           `json:"ready"`
	Baseline string                         `json:"baseline"`
	Projects []LocalTestCorpusProjectResult `json:"projects"`
	Failures []string                       `json:"failures,omitempty"`
}

type LocalTestCorpusProjectResult struct {
	Project  string                     `json:"project"`
	Ready    bool                       `json:"ready"`
	Summary  LocalTestSummary           `json:"summary"`
	Outcomes []LocalTestExpectedOutcome `json:"outcomes"`
}

func RunLocalTests(options LocalTestOptions) (LocalTestReport, error) {
	started := time.Now()
	resetLocalTestPerfCounters(options)
	root := options.Project
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	absRoot, _ := filepath.Abs(root)
	report := LocalTestReport{
		Target:  "local Apex test execution readiness",
		Project: absRoot,
	}
	stopProfile, err := startLocalTestProfiler(options)
	if err != nil {
		return LocalTestReport{}, err
	}
	defer func() {
		if stopProfile != nil {
			_ = stopProfile()
		}
	}()
	projectLabel := filepath.Base(absRoot)
	if projectLabel == "." || projectLabel == string(filepath.Separator) {
		projectLabel = absRoot
	}

	recordLocalTestPhase(&report, options, "load_start", started)
	index, loadDiagnostics, err := loadLocalTestIndex(root)
	recordLocalTestPhase(&report, options, "load_done", started)
	if err != nil {
		outcome := LocalTestOutcome{
			ProjectLabel: projectLabel,
			Outcome:      "load_error",
			Phase:        "load",
			CapabilityID: localTestCapabilityID("load", "", err.Error()),
			Error:        err.Error(),
		}
		report.Diagnostics = loadDiagnostics
		report.Outcomes = append(report.Outcomes, outcome)
		finalizeLocalTestReport(&report, options, started)
		if stopProfile != nil {
			if stopErr := stopProfile(); stopErr != nil {
				return LocalTestReport{}, stopErr
			}
			stopProfile = nil
		}
		if err := maybeWriteLocalTestPerfJSON(report, options); err != nil {
			return LocalTestReport{}, err
		}
		return report, nil
	}
	options, err = loadLocalTestClassFile(options)
	if err != nil {
		outcome := LocalTestOutcome{
			ProjectLabel: projectLabel,
			Outcome:      "load_error",
			Phase:        "load",
			CapabilityID: localTestCapabilityID("load", "", err.Error()),
			Error:        err.Error(),
		}
		report.Outcomes = append(report.Outcomes, outcome)
		finalizeLocalTestReport(&report, options, started)
		if stopProfile != nil {
			if stopErr := stopProfile(); stopErr != nil {
				return LocalTestReport{}, stopErr
			}
			stopProfile = nil
		}
		if err := maybeWriteLocalTestPerfJSON(report, options); err != nil {
			return LocalTestReport{}, err
		}
		return report, nil
	}
	report.Dependencies = append(report.Dependencies, index.Dependencies...)
	report.Diagnostics = append(report.Diagnostics, loadDiagnostics...)

	progress := newLocalTestProgressReporter(options.ProgressWriter)
	recordLocalTestPhase(&report, options, "discover_start", started)
	parallelism := localTestParallelism(options)
	testOpts := apextest.Options{
		Filter:              localTestFilter(options),
		TraceBlocked:        shouldTraceFocusedLocalTests(options),
		SlowTestThresholdMS: options.SlowTestThresholdMS,
		TimeoutMS:           options.TimeoutMS,
		Parallelism:         parallelism,
		Progress:            progress.handle,
	}
	cases := apextest.Discover(index, testOpts)
	cases = filterLocalTestCases(cases, options)
	cases = selectChangedLocalTestCases(&report, index, cases, root, options)
	testOpts.ParallelMethods = shouldParallelizeMethods(options, parallelism, len(cases))
	sort.SliceStable(cases, func(i, j int) bool {
		if cases[i].ClassName == cases[j].ClassName {
			return cases[i].MethodName < cases[j].MethodName
		}
		return cases[i].ClassName < cases[j].ClassName
	})
	report.CasesDiscovered = len(cases)
	recordLocalTestPhase(&report, options, fmt.Sprintf("discover_done total=%d", len(cases)), started)
	durations, err := loadLocalTestDurationHistory(options.DurationHistoryPath)
	if err != nil {
		return LocalTestReport{}, err
	}
	options, parallelism = autoTuneLocalTestOptions(options, len(cases), parallelism)
	testOpts.Parallelism = parallelism
	testOpts.ParallelMethods = shouldParallelizeMethods(options, parallelism, len(cases))
	testOpts.ClassDurationMS = durations
	if options.AutoTune {
		recordLocalTestPhase(&report, options, fmt.Sprintf("auto_tune parallel=%d shardIndex=%d shardCount=%d methodParallel=%t", parallelism, options.ShardIndex, options.ShardCount, testOpts.ParallelMethods), started)
	}
	if strings.TrimSpace(options.WriteClassShards) != "" {
		recordLocalTestPhase(&report, options, "shard_write_start", started)
		shardCount := options.ShardCount
		if shardCount <= 0 {
			shardCount = parallelism
		}
		if shardCount <= 0 {
			shardCount = 1
		}
		if err := writeLocalTestClassShardFiles(options.WriteClassShards, planLocalTestClassShards(cases, durations, shardCount)); err != nil {
			return LocalTestReport{}, err
		}
		recordLocalTestPhase(&report, options, fmt.Sprintf("shard_write_done shards=%d", shardCount), started)
		finalizeLocalTestReport(&report, options, started)
		if stopProfile != nil {
			if stopErr := stopProfile(); stopErr != nil {
				return LocalTestReport{}, stopErr
			}
			stopProfile = nil
		}
		if err := maybeWriteLocalTestPerfJSON(report, options); err != nil {
			return LocalTestReport{}, err
		}
		return report, nil
	}
	if options.ShardCount > 0 {
		sharded, err := selectLocalTestShard(cases, durations, options.ShardCount, options.ShardIndex)
		if err != nil {
			return LocalTestReport{}, err
		}
		cases = sharded
		recordLocalTestPhase(&report, options, fmt.Sprintf("shard_select index=%d count=%d cases=%d", options.ShardIndex, options.ShardCount, len(cases)), started)
	}
	progress.setTotal(len(cases))

	if shouldAnalyzeLocalTests(options, len(cases)) {
		recordLocalTestPhase(&report, options, "analyze_start", started)
		semaResult := sema.Analyze(index)
		recordLocalTestPhase(&report, options, fmt.Sprintf("analyze_done diagnostics=%d", len(semaResult.Diagnostics)), started)
		report.Diagnostics = append(report.Diagnostics, semaResult.Diagnostics...)
		if firstError, ok := firstLocalTestError(semaResult.Diagnostics); ok {
			for _, testCase := range cases {
				report.Outcomes = append(report.Outcomes, localTestDiagnosticOutcome(projectLabel, testCase, "compile_gap", "compile", firstError))
			}
			finalizeLocalTestReport(&report, options, started)
			if stopProfile != nil {
				if stopErr := stopProfile(); stopErr != nil {
					return LocalTestReport{}, stopErr
				}
				stopProfile = nil
			}
			if err := maybeWriteLocalTestPerfJSON(report, options); err != nil {
				return LocalTestReport{}, err
			}
			return report, nil
		}
	} else if strings.TrimSpace(options.Class) == "" && strings.TrimSpace(options.Method) == "" {
		recordLocalTestPhase(&report, options, fmt.Sprintf("analyze_skip total=%d", len(cases)), started)
	}

	if options.ProfileOnTimeout {
		testOpts.TraceBlocked = true
	}
	recordLocalTestPhase(&report, options, "run_start", started)
	runOutcomes, casesRun, triageStopped := runLocalTestCases(index, testOpts, cases, projectLabel, options, started)
	progress.finish()
	report.Outcomes = append(report.Outcomes, runOutcomes...)
	report.CasesRun = casesRun
	report.TriageStopped = triageStopped
	recordLocalTestPhase(&report, options, "run_done", started)
	finalizeLocalTestReport(&report, options, started)
	if stopProfile != nil {
		if stopErr := stopProfile(); stopErr != nil {
			return LocalTestReport{}, stopErr
		}
		stopProfile = nil
	}
	if err := maybeWriteLocalTestPerfJSON(report, options); err != nil {
		return LocalTestReport{}, err
	}
	return report, nil
}

func resetLocalTestPerfCounters(options LocalTestOptions) {
	if strings.TrimSpace(options.PerfJSONPath) == "" {
		return
	}
	apextest.ResetPerfCounters()
	storage.ResetCloneStats()
}

func startLocalTestProfiler(options LocalTestOptions) (func() error, error) {
	if strings.TrimSpace(options.CPUProfilePath) == "" && strings.TrimSpace(options.MemProfilePath) == "" {
		return nil, nil
	}
	var cpuFile *os.File
	if path := strings.TrimSpace(options.CPUProfilePath); path != "" {
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
		if path := strings.TrimSpace(options.MemProfilePath); path != "" {
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

func maybeWriteLocalTestPerfJSON(report LocalTestReport, options LocalTestOptions) error {
	if strings.TrimSpace(options.PerfJSONPath) == "" {
		return nil
	}
	perf := localTestPerfSummary(report, options)
	if err := os.MkdirAll(filepath.Dir(options.PerfJSONPath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(perf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(options.PerfJSONPath, append(data, '\n'), 0o644)
}

func localTestPerfSummary(report LocalTestReport, options LocalTestOptions) LocalTestPerfSummary {
	apexStats := apextest.SnapshotPerfCounters()
	storageStats := storage.SnapshotCloneStats()
	perf := LocalTestPerfSummary{
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		Project:         report.Project,
		DurationMS:      report.DurationMS,
		CasesDiscovered: report.CasesDiscovered,
		CasesRun:        report.CasesRun,
		Summary:         report.Summary,
		Phases:          append([]LocalTestPhaseTiming(nil), report.Phases...),
		CloneStats: LocalTestCloneStats{
			CloneRuntimeOrgCalls:       apexStats.CloneRuntimeOrgCalls,
			CloneRuntimeCalls:          storageStats.CloneRuntimeCalls,
			CloneRollbackSnapshotCalls: storageStats.CloneRollbackSnapshotCalls,
			JournalRollbacks:           apexStats.JournalRollbacks,
			CloneFallbacks:             apexStats.CloneFallbacks,
		},
		CPUProfilePath: strings.TrimSpace(options.CPUProfilePath),
		MemProfilePath: strings.TrimSpace(options.MemProfilePath),
	}
	perf.TopSlowClasses = localTestTopSlowClasses(report.Outcomes, 15)
	perf.TopCloneClasses = localTestTopCloneClasses(apexStats.CloneClasses, report.Outcomes, 15)
	return perf
}

func localTestTopCloneClasses(classes []apextest.PerfCloneClass, outcomes []LocalTestOutcome, limit int) []LocalTestCloneClass {
	if limit <= 0 || len(classes) == 0 {
		return nil
	}
	durations := make(map[string]int64, len(classes))
	for _, outcome := range outcomes {
		if strings.TrimSpace(outcome.Class) == "" {
			continue
		}
		durations[outcome.Class] += outcome.DurationMS
	}
	out := make([]LocalTestCloneClass, 0, len(classes))
	for _, class := range classes {
		out = append(out, LocalTestCloneClass{
			Class:       class.Class,
			SetupClones: class.SetupClones,
			TestClones:  class.TestClones,
			DurationMS:  durations[class.Class],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		left := out[i].SetupClones + out[i].TestClones
		right := out[j].SetupClones + out[j].TestClones
		if left == right {
			if out[i].DurationMS == out[j].DurationMS {
				return out[i].Class < out[j].Class
			}
			return out[i].DurationMS > out[j].DurationMS
		}
		return left > right
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func localTestTopSlowClasses(outcomes []LocalTestOutcome, limit int) []LocalTestPerfClass {
	if limit <= 0 {
		return nil
	}
	totals := map[string]*LocalTestPerfClass{}
	for _, outcome := range outcomes {
		if strings.TrimSpace(outcome.Class) == "" {
			continue
		}
		entry := totals[outcome.Class]
		if entry == nil {
			entry = &LocalTestPerfClass{Class: outcome.Class}
			totals[outcome.Class] = entry
		}
		entry.DurationMS += outcome.DurationMS
		entry.Tests++
	}
	if len(totals) == 0 {
		return nil
	}
	classes := make([]LocalTestPerfClass, 0, len(totals))
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

func reportLocalTestPhase(options LocalTestOptions, event string, started time.Time) {
	if options.ProgressWriter != nil {
		elapsed := time.Since(started)
		fmt.Fprintf(options.ProgressWriter, "Phase: %s elapsed=%s\n", event, formatLocalTestProgressDuration(elapsed))
	}
}

func recordLocalTestPhase(report *LocalTestReport, options LocalTestOptions, event string, started time.Time) {
	duration := time.Since(started).Milliseconds()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	report.Phases = append(report.Phases, LocalTestPhaseTiming{
		Name:            event,
		DurationMS:      duration,
		HeapAllocBytes:  mem.HeapAlloc,
		TotalAllocBytes: mem.TotalAlloc,
		Mallocs:         mem.Mallocs,
		Frees:           mem.Frees,
		NumGC:           mem.NumGC,
	})
	if options.ProgressWriter != nil {
		fmt.Fprintf(options.ProgressWriter, "Phase: %s elapsed=%s\n", event, formatLocalTestProgressDuration(time.Duration(duration)*time.Millisecond))
	}
}

const largeLocalTestAnalysisThreshold = 5000
const localTestTriageClassBatchSize = 1

func runLocalTestCases(index typesys.Index, testOpts apextest.Options, cases []apextest.TestCase, projectLabel string, options LocalTestOptions, started time.Time) ([]LocalTestOutcome, int, bool) {
	if options.MaxFailureGroups <= 0 {
		run := apextest.RunCasesContext(context.Background(), index, testOpts, cases)
		outcomes := localTestOutcomesFromRun(projectLabel, run, options)
		outcomes = retryParallelTimeoutOutcomes(index, testOpts, cases, projectLabel, options, outcomes, started)
		return outcomes, len(cases), false
	}
	outcomes := make([]LocalTestOutcome, 0)
	casesRun := 0
	for _, batch := range localTestCaseBatches(cases, localTestTriageBatchClassSize(options)) {
		reportLocalTestPhase(options, fmt.Sprintf("triage_batch_start cases=%d", len(batch)), started)
		run := apextest.RunCasesContext(context.Background(), index, testOpts, batch)
		outcomes = append(outcomes, localTestOutcomesFromRun(projectLabel, run, options)...)
		casesRun += len(batch)
		groups := localTestFailureGroupCount(outcomes)
		reportLocalTestPhase(options, fmt.Sprintf("triage_batch_done casesRun=%d failureGroups=%d", casesRun, groups), started)
		if groups >= options.MaxFailureGroups {
			reportLocalTestPhase(options, fmt.Sprintf("triage_stop casesRun=%d failureGroups=%d", casesRun, groups), started)
			return outcomes, casesRun, true
		}
	}
	return outcomes, casesRun, false
}

func retryParallelTimeoutOutcomes(index typesys.Index, testOpts apextest.Options, cases []apextest.TestCase, projectLabel string, options LocalTestOptions, outcomes []LocalTestOutcome, started time.Time) []LocalTestOutcome {
	if options.Parallelism <= 1 || testOpts.TimeoutMS <= 0 {
		return outcomes
	}
	timeoutCases := localTestTimeoutCases(cases, outcomes)
	if len(timeoutCases) == 0 {
		return outcomes
	}
	reportLocalTestPhase(options, fmt.Sprintf("timeout_retry_start cases=%d", len(timeoutCases)), started)
	retryOpts := testOpts
	retryOpts.Parallelism = 1
	retryOpts.ParallelMethods = false
	retryOpts.Progress = nil
	retryRun := apextest.RunCasesContext(context.Background(), index, retryOpts, timeoutCases)
	retryOutcomes := localTestOutcomesFromRun(projectLabel, retryRun, options)
	reportLocalTestPhase(options, fmt.Sprintf("timeout_retry_done cases=%d", len(retryOutcomes)), started)
	return replaceLocalTestOutcomes(outcomes, retryOutcomes)
}

func localTestTimeoutCases(cases []apextest.TestCase, outcomes []LocalTestOutcome) []apextest.TestCase {
	timeouts := make(map[string]bool)
	for _, outcome := range outcomes {
		if outcome.Outcome != "timeout" {
			continue
		}
		timeouts[localTestCaseKey(outcome.Class, outcome.Method)] = true
	}
	if len(timeouts) == 0 {
		return nil
	}
	retry := make([]apextest.TestCase, 0, len(timeouts))
	for _, testCase := range cases {
		if timeouts[localTestCaseKey(testCase.ClassName, testCase.MethodName)] {
			retry = append(retry, testCase)
		}
	}
	return retry
}

func replaceLocalTestOutcomes(outcomes, replacements []LocalTestOutcome) []LocalTestOutcome {
	if len(replacements) == 0 {
		return outcomes
	}
	byCase := make(map[string]LocalTestOutcome, len(replacements))
	for _, outcome := range replacements {
		byCase[localTestCaseKey(outcome.Class, outcome.Method)] = outcome
	}
	out := append([]LocalTestOutcome(nil), outcomes...)
	for i, outcome := range out {
		if replacement, ok := byCase[localTestCaseKey(outcome.Class, outcome.Method)]; ok {
			out[i] = replacement
		}
	}
	return out
}

func localTestCaseKey(className, methodName string) string {
	return className + "\x00" + methodName
}

func localTestTriageBatchClassSize(options LocalTestOptions) int {
	if options.Parallelism > 1 {
		return options.Parallelism
	}
	return localTestTriageClassBatchSize
}

func localTestOutcomesFromRun(projectLabel string, run testreport.Run, options LocalTestOptions) []LocalTestOutcome {
	outcomes := make([]LocalTestOutcome, 0)
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if !matchesLocalTestCase(testCase.ClassName, testCase.MethodName, options) {
				continue
			}
			outcomes = append(outcomes, localTestRunOutcome(projectLabel, testCase))
		}
	}
	return outcomes
}

func localTestCaseBatches(cases []apextest.TestCase, classBatchSize int) [][]apextest.TestCase {
	if classBatchSize <= 0 || len(cases) == 0 {
		return nil
	}
	var batches [][]apextest.TestCase
	var batch []apextest.TestCase
	classesInBatch := 0
	lastClass := ""
	for _, testCase := range cases {
		if testCase.ClassName != lastClass {
			if classesInBatch >= classBatchSize && len(batch) > 0 {
				batches = append(batches, batch)
				batch = nil
				classesInBatch = 0
			}
			classesInBatch++
			lastClass = testCase.ClassName
		}
		batch = append(batch, testCase)
	}
	if len(batch) > 0 {
		batches = append(batches, batch)
	}
	return batches
}

func selectChangedLocalTestCases(report *LocalTestReport, index typesys.Index, cases []apextest.TestCase, root string, options LocalTestOptions) []apextest.TestCase {
	if strings.TrimSpace(options.ChangedSince) == "" {
		return cases
	}
	changes, err := watch.GitChangesSince(root, options.ChangedSince)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "changed_since",
			Message:  err.Error(),
		})
		return cases
	}
	selection := watch.SelectAffectedTests(index, changes)
	report.Selection = &selection
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

func shouldAnalyzeLocalTests(options LocalTestOptions, totalCases int) bool {
	if strings.TrimSpace(options.Class) != "" || strings.TrimSpace(options.Method) != "" {
		return false
	}
	if options.ForceAnalysis {
		return true
	}
	if options.Parallelism > 0 || options.ProgressWriter != nil {
		return false
	}
	if options.BlockersOnly || options.TopFailures > 0 {
		return false
	}
	return totalCases <= largeLocalTestAnalysisThreshold
}

func localTestParallelism(options LocalTestOptions) int {
	if options.Parallelism > 0 {
		return options.Parallelism
	}
	if strings.TrimSpace(options.Method) != "" {
		return 1
	}
	if strings.TrimSpace(options.Class) != "" {
		procs := runtime.GOMAXPROCS(0)
		if procs < 2 {
			return procs
		}
		if procs < 8 {
			return procs
		}
		return 8
	}
	procs := runtime.GOMAXPROCS(0)
	if procs < 1 {
		return 1
	}
	if procs > 8 {
		return 8
	}
	return procs
}

func shouldParallelizeMethods(options LocalTestOptions, parallelism int, totalCases int) bool {
	if options.ParallelMethods {
		return true
	}
	if options.AutoTune && strings.TrimSpace(options.Method) == "" && strings.TrimSpace(options.Class) != "" && parallelism > 1 && totalCases >= 24 {
		return true
	}
	return false
}

func autoTuneLocalTestOptions(options LocalTestOptions, totalCases int, parallelism int) (LocalTestOptions, int) {
	if !options.AutoTune {
		return options, parallelism
	}
	if options.Parallelism == 0 {
		parallelism = autoParallelismForCases(totalCases)
	}
	if options.ShardCount == 0 && options.AutoShardCount {
		if shardCount, ok := localTestEnvInt("GLADE_SHARD_COUNT"); ok && shardCount > 0 {
			options.ShardCount = shardCount
		}
	}
	if options.ShardIndex == 0 && options.AutoShardIndex && options.ShardCount > 0 {
		if shardIndex, ok := localTestEnvInt("GLADE_SHARD_INDEX"); ok && shardIndex >= 0 {
			options.ShardIndex = shardIndex
		}
	}
	if options.ShardCount > 0 && options.ShardIndex >= options.ShardCount {
		options.ShardIndex = 0
	}
	return options, parallelism
}

func autoParallelismForCases(totalCases int) int {
	procs := runtime.GOMAXPROCS(0)
	if procs < 1 {
		procs = 1
	}
	if procs > 12 {
		procs = 12
	}
	switch {
	case totalCases <= 16:
		if procs > 2 {
			return 2
		}
	case totalCases <= 200:
		if procs > 4 {
			return 4
		}
	case totalCases <= 1200:
		if procs > 8 {
			return 8
		}
	default:
		if procs > 4 {
			return 4
		}
	}
	return procs
}

func localTestEnvInt(name string) (int, bool) {
	raw, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(raw) == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	return parsed, true
}

type localTestProgressReporter struct {
	w        io.Writer
	started  time.Time
	last     time.Time
	total    int
	done     int
	passed   int
	failed   int
	errors   int
	active   string
	printed  bool
	finished bool
	mu       sync.Mutex
}

func newLocalTestProgressReporter(w io.Writer) *localTestProgressReporter {
	return &localTestProgressReporter{
		w:       w,
		started: time.Now(),
	}
}

func (r *localTestProgressReporter) setTotal(total int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.total = total
	r.mu.Unlock()
}

func (r *localTestProgressReporter) handle(progress apextest.TestProgress) {
	if r == nil || r.w == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started.IsZero() {
		r.started = time.Now()
	}
	important := false
	completed := false
	name := progress.ClassName
	if progress.MethodName != "" {
		name += "." + progress.MethodName
	}
	switch progress.Event {
	case "compile_start":
		r.active = "compile"
	case "compile_done":
		r.active = "compile"
		important = progress.Status != "" && progress.Status != "pass"
	case "setup_start", "test_start":
		r.active = name
	case "setup_done":
		r.active = name
		if progress.Status != "" && progress.Status != "pass" {
			r.errors++
			important = r.errors <= 10
		}
	case "test_done":
		r.done++
		completed = true
		r.active = name
		switch testreport.Status(progress.Status) {
		case testreport.StatusPass:
			r.passed++
		case testreport.StatusFail:
			r.failed++
			important = r.failed <= 10
		default:
			r.errors++
			important = r.errors <= 10
		}
	}
	now := time.Now()
	if !r.printed || r.done == r.total || important || (completed && r.done > 0 && r.done%25 == 0) || now.Sub(r.last) >= 2*time.Second {
		r.writeLine()
	}
}

func (r *localTestProgressReporter) finish() {
	if r == nil || r.w == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return
	}
	r.finished = true
	if !r.printed || r.done < r.total {
		r.writeLine()
	}
}

func (r *localTestProgressReporter) writeLine() {
	line := fmt.Sprintf("Progress: %s elapsed=%s eta=%s pass=%d fail=%d error=%d",
		r.countText(), formatLocalTestProgressDuration(time.Since(r.started)), r.etaText(), r.passed, r.failed, r.errors)
	if r.active != "" {
		line += " running=" + r.active
	}
	fmt.Fprintln(r.w, line)
	r.printed = true
	r.last = time.Now()
}

func (r *localTestProgressReporter) countText() string {
	if r.total > 0 {
		return fmt.Sprintf("%d/%d", r.done, r.total)
	}
	return fmt.Sprintf("%d done", r.done)
}

func (r *localTestProgressReporter) etaText() string {
	if r.total <= 0 || r.done <= 0 || r.done >= r.total {
		return "0s"
	}
	elapsed := time.Since(r.started)
	remaining := time.Duration(int64(elapsed) * int64(r.total-r.done) / int64(r.done))
	return formatLocalTestProgressDuration(remaining)
}

func formatLocalTestProgressDuration(d time.Duration) string {
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

func shouldTraceFocusedLocalTests(options LocalTestOptions) bool {
	return options.TraceBlocked || options.ProfileOnTimeout
}

func LocalTestReportFromRun(projectRoot string, run testreport.Run) LocalTestReport {
	root := projectRoot
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	absRoot, _ := filepath.Abs(root)
	projectLabel := filepath.Base(absRoot)
	if projectLabel == "." || projectLabel == string(filepath.Separator) {
		projectLabel = absRoot
	}
	report := LocalTestReport{
		Target:  "local Apex test execution readiness",
		Project: absRoot,
	}
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			report.Outcomes = append(report.Outcomes, localTestRunOutcome(projectLabel, testCase))
		}
	}
	finalizeLocalTestReport(&report, LocalTestOptions{}, time.Now())
	report.DurationMS = run.Summary().DurationMS
	return report
}

func CheckLocalTestCorpus(path string) (LocalTestCorpusReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LocalTestCorpusReport{}, err
	}
	var baseline LocalTestCorpusBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return LocalTestCorpusReport{}, err
	}
	absPath, _ := filepath.Abs(path)
	report := LocalTestCorpusReport{
		Target:   baseline.Target,
		Ready:    true,
		Baseline: absPath,
	}
	baseDir := filepath.Dir(absPath)
	for _, expected := range baseline.Projects {
		projectPath := expected.Project
		if !filepath.IsAbs(projectPath) {
			projectPath = filepath.Clean(filepath.Join(baseDir, projectPath))
		}
		actual, err := RunLocalTests(LocalTestOptions{Project: projectPath})
		if err != nil {
			return report, err
		}
		actualProject := LocalTestCorpusProjectResult{
			Project:  expected.Project,
			Ready:    actual.Ready,
			Summary:  actual.Summary,
			Outcomes: stableLocalTestOutcomes(actual.Outcomes),
		}
		report.Projects = append(report.Projects, actualProject)
		if actual.Ready != expected.Ready {
			report.Failures = append(report.Failures, fmt.Sprintf("%s ready = %t, want %t", expected.Project, actual.Ready, expected.Ready))
		}
		if actual.Summary != expected.Summary {
			report.Failures = append(report.Failures, fmt.Sprintf("%s summary = %+v, want %+v", expected.Project, actual.Summary, expected.Summary))
		}
		compareLocalTestOutcomes(expected.Project, actualProject.Outcomes, expected.Outcomes, &report.Failures)
	}
	report.Ready = len(report.Failures) == 0
	if !report.Ready {
		return report, fmt.Errorf("local-tests corpus baseline mismatch: %s", strings.Join(report.Failures, "; "))
	}
	return report, nil
}

func WriteLocalTestJSON(w io.Writer, report LocalTestReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteLocalTestCorpusJSON(w io.Writer, report LocalTestCorpusReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteLocalTestText(w io.Writer, report LocalTestReport) {
	state := "ready"
	if !report.Ready {
		state = "not ready"
	}
	fmt.Fprintf(w, "Local test readiness: %s\n", state)
	fmt.Fprintf(w, "project: %s\n", report.Project)
	if report.CasesDiscovered != 0 || report.CasesRun != 0 || report.TriageStopped {
		fmt.Fprintf(w, "cases: discovered=%d run=%d triage_stopped=%t duration_ms=%d\n", report.CasesDiscovered, report.CasesRun, report.TriageStopped, report.DurationMS)
	}
	fmt.Fprintf(w, "summary: pass=%d fail=%d unsupported=%d load_error=%d compile_error=%d internal_error=%d assert_fail=%d runtime_gap=%d compile_gap=%d timeout=%d total=%d\n",
		report.Summary.Pass,
		report.Summary.Fail,
		report.Summary.Unsupported,
		report.Summary.LoadErrors,
		report.Summary.CompileErrors,
		report.Summary.InternalErrors,
		report.Summary.AssertFailures,
		report.Summary.RuntimeGaps,
		report.Summary.CompileGaps,
		report.Summary.Timeouts,
		report.Summary.Total,
	)
	for _, group := range report.TopFailures {
		fmt.Fprintf(w, "* %s", group.Outcome)
		if group.CapabilityID != "" {
			fmt.Fprintf(w, " [%s]", group.CapabilityID)
		}
		fmt.Fprintf(w, ": %d", group.Count)
		if group.Error != "" {
			fmt.Fprintf(w, " - %s", group.Error)
		}
		if len(group.Samples) != 0 {
			fmt.Fprintf(w, " samples=%s", strings.Join(group.Samples, ","))
		}
		fmt.Fprintln(w)
	}
	for _, outcome := range report.Outcomes {
		if outcome.Outcome == "pass" {
			continue
		}
		fmt.Fprintf(w, "- %s.%s: %s", outcome.Class, outcome.Method, outcome.Outcome)
		if outcome.CapabilityID != "" {
			fmt.Fprintf(w, " [%s]", outcome.CapabilityID)
		}
		if outcome.Error != "" {
			fmt.Fprintf(w, ": %s", outcome.Error)
		}
		fmt.Fprintln(w)
	}
}

func WriteLocalTestCorpusText(w io.Writer, report LocalTestCorpusReport) {
	state := "ready"
	if !report.Ready {
		state = "not ready"
	}
	fmt.Fprintf(w, "Local test corpus: %s\n", state)
	fmt.Fprintf(w, "baseline: %s\n", report.Baseline)
	for _, project := range report.Projects {
		fmt.Fprintf(w, "- %s: ready=%t pass=%d fail=%d unsupported=%d load_error=%d compile_error=%d internal_error=%d assert_fail=%d runtime_gap=%d compile_gap=%d timeout=%d total=%d\n",
			project.Project,
			project.Ready,
			project.Summary.Pass,
			project.Summary.Fail,
			project.Summary.Unsupported,
			project.Summary.LoadErrors,
			project.Summary.CompileErrors,
			project.Summary.InternalErrors,
			project.Summary.AssertFailures,
			project.Summary.RuntimeGaps,
			project.Summary.CompileGaps,
			project.Summary.Timeouts,
			project.Summary.Total,
		)
	}
	for _, failure := range report.Failures {
		fmt.Fprintf(w, "! %s\n", failure)
	}
}

func stableLocalTestOutcomes(outcomes []LocalTestOutcome) []LocalTestExpectedOutcome {
	stable := make([]LocalTestExpectedOutcome, 0, len(outcomes))
	for _, outcome := range outcomes {
		stable = append(stable, LocalTestExpectedOutcome{
			Class:        outcome.Class,
			Method:       outcome.Method,
			Outcome:      outcome.Outcome,
			CapabilityID: outcome.CapabilityID,
		})
	}
	sort.SliceStable(stable, func(i, j int) bool {
		if stable[i].Class == stable[j].Class {
			return stable[i].Method < stable[j].Method
		}
		return stable[i].Class < stable[j].Class
	})
	return stable
}

func compareLocalTestOutcomes(project string, actual, expected []LocalTestExpectedOutcome, failures *[]string) {
	if len(actual) != len(expected) {
		*failures = append(*failures, fmt.Sprintf("%s outcomes length = %d, want %d", project, len(actual), len(expected)))
		return
	}
	for i := range expected {
		if actual[i] != expected[i] {
			*failures = append(*failures, fmt.Sprintf("%s outcome[%d] = %+v, want %+v", project, i, actual[i], expected[i]))
		}
	}
}

func loadLocalTestIndex(root string) (typesys.Index, []diagnostic.Diagnostic, error) {
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, nil, err
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		index := typesys.Build(p, schema.Schema{})
		diag := diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESCHEMA001",
			Message:  fmt.Sprintf("metadata schema load failed: %v", err),
		}
		index.Diagnostics = append(index.Diagnostics, diag)
		return index, []diagnostic.Diagnostic{diag}, nil
	}
	return typesys.Build(p, s), nil, nil
}

func localTestFilter(options LocalTestOptions) string {
	switch {
	case options.Class != "" && options.Method != "":
		return options.Class + "." + options.Method
	case options.Class != "":
		return options.Class
	case options.Method != "":
		return options.Method
	default:
		return ""
	}
}

func loadLocalTestClassFile(options LocalTestOptions) (LocalTestOptions, error) {
	if strings.TrimSpace(options.ClassFile) == "" {
		return options, nil
	}
	data, err := os.ReadFile(options.ClassFile)
	if err != nil {
		return options, fmt.Errorf("read --class-file: %w", err)
	}
	for _, line := range strings.FieldsFunc(string(data), func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';'
	}) {
		if className := strings.TrimSpace(line); className != "" && !strings.HasPrefix(className, "#") {
			options.ClassList = append(options.ClassList, className)
		}
	}
	return options, nil
}

func filterLocalTestCases(cases []apextest.TestCase, options LocalTestOptions) []apextest.TestCase {
	out := make([]apextest.TestCase, 0, len(cases))
	for _, testCase := range cases {
		if matchesLocalTestCase(testCase.ClassName, testCase.MethodName, options) {
			out = append(out, testCase)
		}
	}
	return out
}

func matchesLocalTestCase(className, methodName string, options LocalTestOptions) bool {
	if options.Class != "" && !strings.EqualFold(className, options.Class) {
		return false
	}
	if strings.TrimSpace(options.StartClass) != "" && strings.ToLower(className) < strings.ToLower(strings.TrimSpace(options.StartClass)) {
		return false
	}
	if len(options.ClassList) != 0 && !localTestClassListContains(options.ClassList, className) {
		return false
	}
	if options.Method != "" && !strings.EqualFold(methodName, options.Method) {
		return false
	}
	return true
}

func localTestClassListContains(classes []string, className string) bool {
	for _, candidate := range classes {
		if strings.EqualFold(strings.TrimSpace(candidate), className) {
			return true
		}
	}
	return false
}

func firstLocalTestError(diagnostics []diagnostic.Diagnostic) (diagnostic.Diagnostic, bool) {
	for _, diag := range diagnostics {
		if diag.Severity == diagnostic.Error {
			return diag, true
		}
	}
	return diagnostic.Diagnostic{}, false
}

func localTestDiagnosticOutcome(projectLabel string, testCase apextest.TestCase, outcome, phase string, diag diagnostic.Diagnostic) LocalTestOutcome {
	line := testCase.Range.Start.Line
	if diag.Range != nil && diag.Range.Start.Line > 0 {
		line = diag.Range.Start.Line
	}
	file := testCase.File
	if diag.File != "" {
		file = diag.File
	}
	return LocalTestOutcome{
		ProjectLabel: projectLabel,
		Class:        testCase.ClassName,
		Method:       testCase.MethodName,
		Outcome:      outcome,
		Phase:        phase,
		CapabilityID: localTestCapabilityID(phase, diag.Code, diag.Message),
		File:         file,
		Line:         line,
		Error:        diag.Message,
	}
}

func localTestRunOutcome(projectLabel string, testCase testreport.Case) LocalTestOutcome {
	outcome := "pass"
	phase := ""
	if testCase.Status != testreport.StatusPass {
		phase = "execute"
	}
	switch testCase.Status {
	case testreport.StatusPass:
		outcome = "pass"
	case testreport.StatusUnsupported:
		outcome = "unsupported"
	case testreport.StatusCompileError:
		outcome = "compile_gap"
		phase = "compile"
	case testreport.StatusRuntimeError:
		outcome = "internal_error"
	default:
		outcome = "runtime_gap"
	}
	if testCase.Problem != nil && strings.EqualFold(testCase.Problem.Type, "UnsupportedFeature") {
		outcome = "unsupported"
	}
	if testCase.Problem != nil && isLocalTestTimeoutProblem(testCase.Problem.Type, testCase.Problem.Message) {
		outcome = "timeout"
		phase = "timeout"
	}
	if testCase.Problem != nil && strings.Contains(strings.ToLower(testCase.Problem.Type+" "+testCase.Problem.Message), "assert") {
		outcome = "assert_fail"
		phase = "assert"
	}
	out := LocalTestOutcome{
		ProjectLabel: projectLabel,
		Class:        testCase.ClassName,
		Method:       testCase.MethodName,
		Outcome:      outcome,
		Phase:        phase,
		DurationMS:   testCase.DurationMS,
	}
	if len(testCase.Trace) > 0 {
		out.TraceEvents = len(testCase.Trace)
	}
	if testCase.Profile != nil {
		out.ProfileEvents = testCase.Profile.Events
		out.ProfileCategories = testCase.Profile.Categories
	}
	if testCase.Problem != nil {
		out.Error = testCase.Problem.Message
		out.CapabilityID = localTestCapabilityID(phase, testCase.Problem.Type, testCase.Problem.Message)
		if len(testCase.Problem.Stack) > 0 {
			frame := testCase.Problem.Stack[0]
			out.TopFrame = &frame
			out.File = frame.File
			out.Line = frame.Line
		}
	}
	return out
}

func localTestCapabilityID(phase, code, message string) string {
	code = strings.TrimSpace(code)
	if strings.EqualFold(code, "UnsupportedFeature") {
		return "apex.test.unsupported"
	}
	if isLocalTestTimeoutProblem(code, message) {
		return "apex.test.timeout"
	}
	if strings.EqualFold(code, "Canceled") {
		return "apex.test.canceled"
	}
	if code != "" {
		return strings.ToLower(code)
	}
	text := strings.ToLower(message)
	switch {
	case strings.Contains(text, "unsupported"):
		return "apex.test.unsupported"
	case strings.Contains(text, "metadata"):
		return "metadata.load"
	case strings.Contains(text, "parse"):
		return "apex.parse"
	case strings.Contains(text, "panic") || strings.Contains(text, "internal"):
		return "glade.internal"
	case phase != "":
		return "apex.test." + strings.ReplaceAll(phase, "_", "-")
	default:
		return "apex.test.unknown"
	}
}

func isLocalTestTimeoutProblem(code, message string) bool {
	text := strings.ToLower(strings.TrimSpace(code + " " + message))
	return strings.Contains(text, "deadline exceeded") || strings.Contains(text, "context canceled")
}

func finalizeLocalTestReport(report *LocalTestReport, options LocalTestOptions, started time.Time) {
	if options.BlockersOnly {
		outcomes := report.Outcomes[:0]
		for _, outcome := range report.Outcomes {
			if outcome.Outcome != "pass" {
				outcomes = append(outcomes, outcome)
			}
		}
		report.Outcomes = outcomes
	}
	for _, outcome := range report.Outcomes {
		report.Summary.Total++
		switch outcome.Outcome {
		case "pass":
			report.Summary.Pass++
		case "fail":
			report.Summary.Fail++
		case "unsupported":
			report.Summary.Unsupported++
		case "load_error":
			report.Summary.LoadErrors++
		case "compile_error":
			report.Summary.CompileErrors++
		case "internal_error":
			report.Summary.InternalErrors++
		case "assert_fail":
			report.Summary.AssertFailures++
		case "runtime_gap":
			report.Summary.RuntimeGaps++
		case "compile_gap":
			report.Summary.CompileGaps++
		case "timeout":
			report.Summary.Timeouts++
		}
	}
	if options.TopFailures > 0 {
		report.TopFailures = localTestTopFailures(report.Outcomes, options.TopFailures)
	}
	report.Ready = report.Summary.Fail == 0 &&
		report.Summary.Unsupported == 0 &&
		report.Summary.LoadErrors == 0 &&
		report.Summary.CompileErrors == 0 &&
		report.Summary.InternalErrors == 0 &&
		report.Summary.AssertFailures == 0 &&
		report.Summary.RuntimeGaps == 0 &&
		report.Summary.CompileGaps == 0 &&
		report.Summary.Timeouts == 0
	report.DurationMS = time.Since(started).Milliseconds()
}

func localTestTopFailures(outcomes []LocalTestOutcome, limit int) []LocalTestFailureGroup {
	if limit <= 0 {
		return nil
	}
	counts := localTestFailureGroupStats(outcomes)
	groups := make([]LocalTestFailureGroup, 0, len(counts))
	for key, stat := range counts {
		groups = append(groups, LocalTestFailureGroup{
			Outcome:      key.outcome,
			Phase:        key.phase,
			CapabilityID: key.capabilityID,
			Error:        key.error,
			Count:        stat.count,
			Samples:      stat.samples,
		})
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Count != groups[j].Count {
			return groups[i].Count > groups[j].Count
		}
		if groups[i].Outcome != groups[j].Outcome {
			return groups[i].Outcome < groups[j].Outcome
		}
		if groups[i].CapabilityID != groups[j].CapabilityID {
			return groups[i].CapabilityID < groups[j].CapabilityID
		}
		return groups[i].Error < groups[j].Error
	})
	if len(groups) > limit {
		groups = groups[:limit]
	}
	return groups
}

type localTestFailureGroupKey struct {
	outcome      string
	phase        string
	capabilityID string
	error        string
}

func localTestFailureGroupCount(outcomes []LocalTestOutcome) int {
	return len(localTestFailureGroups(outcomes))
}

func localTestFailureGroups(outcomes []LocalTestOutcome) map[localTestFailureGroupKey]int {
	stats := localTestFailureGroupStats(outcomes)
	counts := make(map[localTestFailureGroupKey]int)
	for key, stat := range stats {
		counts[key] = stat.count
	}
	return counts
}

type localTestFailureGroupStat struct {
	count   int
	samples []string
}

func localTestFailureGroupStats(outcomes []LocalTestOutcome) map[localTestFailureGroupKey]localTestFailureGroupStat {
	counts := make(map[localTestFailureGroupKey]localTestFailureGroupStat)
	for _, outcome := range outcomes {
		if outcome.Outcome == "pass" {
			continue
		}
		key := localTestFailureGroupKey{
			outcome:      outcome.Outcome,
			phase:        outcome.Phase,
			capabilityID: outcome.CapabilityID,
			error:        normalizeLocalTestFailureError(outcome.Error),
		}
		stat := counts[key]
		stat.count++
		if len(stat.samples) < 3 {
			stat.samples = append(stat.samples, outcome.Class+"."+outcome.Method)
		}
		counts[key] = stat
	}
	return counts
}

func normalizeLocalTestFailureError(errText string) string {
	errText = strings.TrimSpace(errText)
	if errText == "" {
		return ""
	}
	lines := strings.Split(errText, "\n")
	errText = strings.TrimSpace(lines[0])
	if strings.Contains(errText, "context deadline exceeded") {
		return "context deadline exceeded"
	}
	if strings.Contains(errText, "HTTP transport is not available") {
		return "HTTP transport is not available"
	}
	for _, prefix := range []string{
		"System.AssertException:",
		"Assertion Failed:",
		"Database.insert failed:",
		"Database.update failed:",
		"Database.delete failed:",
		"soql:",
	} {
		if strings.HasPrefix(errText, prefix) {
			errText = prefix + " " + strings.TrimSpace(strings.TrimPrefix(errText, prefix))
			break
		}
	}
	errText = normalizeLocalTestDynamicText(errText)
	if len(errText) > 180 {
		errText = errText[:180]
	}
	return errText
}

func normalizeLocalTestDynamicText(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch >= '0' && ch <= '9' {
			j := i + 1
			for j < len(text) && text[j] >= '0' && text[j] <= '9' {
				j++
			}
			out.WriteByte('#')
			i = j - 1
			continue
		}
		out.WriteByte(ch)
	}
	return out.String()
}

func IsLocalTestTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}
