package apextest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/profile"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/semanticcache"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/trace"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

type Options struct {
	Filter              string
	SelectedClasses     []string
	SelectedMethod      string
	LimitMode           vm.LimitMode
	LimitCaps           vm.LimitCaps
	LimitCapsSet        bool
	TraceBlocked        bool
	TraceAll            bool
	SlowTestThresholdMS int64
	TimeoutMS           int64
	Parallelism         int
	ParallelMethods     bool
	NoDiskCache         bool
	// RestoredRuntimeMultiWorker is an internal, default-off experiment that
	// permits disk-restored runtimes with parallel method workers. Product
	// configuration and CLI surfaces intentionally do not expose it.
	RestoredRuntimeMultiWorker bool
	ClassDurationMS            map[string]int64
	MethodDurationMS           map[string]int64
	PerfCounters               bool
	PreRunPhaseDurations       PreRunPhaseDurations
	BuildArtifacts             *typesys.BuildArtifacts
	SourceDigests              *typesys.SourceDigestSet
	Progress                   func(TestProgress)
}

// DiskRuntimeCachePolicyReason identifies the reason a test run will or will
// not read and write the on-disk runtime cache.
type DiskRuntimeCachePolicyReason string

const (
	DiskRuntimeCacheEnabled              DiskRuntimeCachePolicyReason = "enabled"
	DiskRuntimeCacheNoDiskCache          DiskRuntimeCachePolicyReason = "no_disk_cache"
	DiskRuntimeCacheDisabledEnvironment  DiskRuntimeCachePolicyReason = "disabled_environment"
	DiskRuntimeCacheParallelMethodBypass DiskRuntimeCachePolicyReason = "parallel_method_bypass"
)

// DiskRuntimeCachePolicy is the effective on-disk runtime-cache decision for
// one test invocation. A persistent test server keeps an in-process runtime
// independently of this policy.
type DiskRuntimeCachePolicy struct {
	Enabled bool
	Reason  DiskRuntimeCachePolicyReason
}

// ResolveDiskRuntimeCachePolicy returns the effective cache policy after
// applying command options, process configuration, and the default-off
// multi-worker restored-runtime guard.
func ResolveDiskRuntimeCachePolicy(opts Options) DiskRuntimeCachePolicy {
	if opts.NoDiskCache {
		return DiskRuntimeCachePolicy{Reason: DiskRuntimeCacheNoDiskCache}
	}
	if !diskCacheEnabled() {
		return DiskRuntimeCachePolicy{Reason: DiskRuntimeCacheDisabledEnvironment}
	}
	if opts.ParallelMethods && opts.Parallelism > 1 && !opts.RestoredRuntimeMultiWorker {
		return DiskRuntimeCachePolicy{Reason: DiskRuntimeCacheParallelMethodBypass}
	}
	return DiskRuntimeCachePolicy{Enabled: true, Reason: DiskRuntimeCacheEnabled}
}

type PreRunPhaseDurations struct {
	ProjectLoad time.Duration
	SchemaLoad  time.Duration
	IndexBuild  time.Duration
	Discover    time.Duration
}

type permissionSetMetadata struct {
	Label                  string                           `xml:"label"`
	CustomPermission       []permissionSetCustomPermission  `xml:"customPermissions"`
	FieldPermissions       []permissionSetFieldPermission   `xml:"fieldPermissions"`
	ObjectPermission       []permissionSetObjectPermission  `xml:"objectPermissions"`
	RecordTypeVisibilities []profileRecordTypeVisibilityXML `xml:"recordTypeVisibilities"`
}

type permissionSetCustomPermission struct {
	Enabled bool   `xml:"enabled"`
	Name    string `xml:"name"`
}

type permissionSetFieldPermission struct {
	Editable bool   `xml:"editable"`
	Field    string `xml:"field"`
	Readable bool   `xml:"readable"`
}

type permissionSetObjectPermission struct {
	AllowCreate      bool   `xml:"allowCreate"`
	AllowDelete      bool   `xml:"allowDelete"`
	AllowEdit        bool   `xml:"allowEdit"`
	AllowRead        bool   `xml:"allowRead"`
	ModifyAllRecords bool   `xml:"modifyAllRecords"`
	Object           string `xml:"object"`
	ViewAllRecords   bool   `xml:"viewAllRecords"`
}

type TestCase struct {
	ClassName  string
	MethodName string
	File       string
	Range      diagnostic.Range
	Body       string
	SeeAllData bool
	CostHint   int64 // generic, history-free cost signal used by the priority dispatcher
	// ReturnType and Modifiers preserve the indexed method declaration shape
	// so compilation does not force a void/static signature onto a
	// Salesforce-accepted value-returning or differently-modified test method.
	ReturnType string
	Modifiers  []string
}

type TestProgress struct {
	Event      string
	ClassName  string
	MethodName string
	DurationMS int64
	Status     string
}

func Discover(index typesys.Index, opts Options) []TestCase {
	var out []TestCase
	filters := testFilters(opts.Filter)
	selectedClasses := selectedClassSet(opts.SelectedClasses)
	selectedMethod := strings.TrimSpace(opts.SelectedMethod)
	for _, typ := range index.Types {
		if typ.Dependency {
			continue
		}
		if typ.Kind != apexast.DeclarationClass {
			continue
		}
		if len(selectedClasses) != 0 && !selectedClasses[strings.ToLower(typ.Name)] {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod {
				continue
			}
			if !member.IsTest {
				continue
			}
			if isTestSetup(member.Modifiers) {
				continue
			}
			testName := typ.Name + "." + member.Name
			if len(filters) > 0 && !matchesTestFilter(testName, filters) {
				continue
			}
			if selectedMethod != "" && !strings.EqualFold(member.Name, selectedMethod) {
				continue
			}
			out = append(out, TestCase{
				ClassName:  typ.Name,
				MethodName: member.Name,
				File:       typ.File,
				Range:      member.Range,
				SeeAllData: isSeeAllDataTest(member.Modifiers),
				CostHint:   testCaseCostHint(typ.File),
				ReturnType: member.Type,
				Modifiers:  member.Modifiers,
			})
		}
	}
	return out
}

func selectedClassSet(classes []string) map[string]bool {
	out := map[string]bool{}
	for _, className := range classes {
		className = strings.TrimSpace(className)
		if className == "" {
			continue
		}
		out[strings.ToLower(className)] = true
	}
	return out
}

func testFilters(filter string) []string {
	parts := strings.Split(filter, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func matchesTestFilter(testName string, filters []string) bool {
	testName = strings.ToLower(testName)
	for _, filter := range filters {
		if strings.Contains(testName, filter) {
			return true
		}
	}
	return false
}

func Run(index typesys.Index, opts Options) testreport.Run {
	return RunContext(context.Background(), index, opts)
}

func RunContext(ctx context.Context, index typesys.Index, opts Options) testreport.Run {
	return RunCasesContext(ctx, index, opts, nil)
}

type runExecution struct {
	diskCacheEnabled bool
	counters         *runPerfCounters
	vmPerf           *vm.PerfRecorder
}

func RunCasesContext(ctx context.Context, index typesys.Index, opts Options, cases []TestCase) testreport.Run {
	return runCasesContextWithSemanticGateHooks(ctx, index, opts, cases, semanticGateHooks{})
}

// semanticGateHooks exists only to make the semantic identity-to-analysis
// boundary deterministic in package tests. It is passed with one run and is
// deliberately not process-global, so concurrent callers cannot observe a
// test hook from another run.
type semanticGateHooks struct {
	afterIdentity func()
	afterAnalysis func()
}

func runCasesContextWithSemanticGateHooks(ctx context.Context, index typesys.Index, opts Options, cases []TestCase, hooks semanticGateHooks) testreport.Run {
	runState := runExecution{
		diskCacheEnabled: useDiskRuntimeCache(opts),
		counters:         newRunPerfCounters(opts.PerfCounters),
	}
	if opts.PerfCounters {
		runState.vmPerf = vm.NewPerfRecorder()
	}
	defer func() {
		if runState.vmPerf != nil {
			runState.counters.captureVMPerf(runState.vmPerf.Snapshot())
		}
		publishPerfCounters(runState.counters)
	}()
	if runState.counters.enabled {
		runState.counters.phases.projectLoadNS.Store(opts.PreRunPhaseDurations.ProjectLoad.Nanoseconds())
		runState.counters.phases.schemaLoadNS.Store(opts.PreRunPhaseDurations.SchemaLoad.Nanoseconds())
		runState.counters.phases.indexBuildNS.Store(opts.PreRunPhaseDurations.IndexBuild.Nanoseconds())
		runState.counters.phases.discoverNS.Store(opts.PreRunPhaseDurations.Discover.Nanoseconds())
	}
	started := time.Now()
	if snapshotErr := validateBuildArtifacts(index, opts.BuildArtifacts); snapshotErr != nil {
		return compileErrorRun(cases, snapshotErr, started, opts)
	}
	if opts.BuildArtifacts != nil {
		// An attached artifact is authoritative. Do not let a caller provide a
		// digest set from another build, which would key semantic or runtime
		// caches with a source generation different from the index.
		opts.SourceDigests = opts.BuildArtifacts.SourceDigests
	}
	emitProgress := opts.Progress != nil
	if cases == nil {
		cases = Discover(index, opts)
	}
	sources := newSourceCache()
	if err := sources.seedBuildArtifacts(index, opts.BuildArtifacts); err != nil {
		return compileErrorRun(cases, err, started, opts)
	}
	var generation runtimeGeneration
	var generationErr error
	opts.SourceDigests, generationErr = authoritativeRuntimeSourceDigests(index, opts.SourceDigests)
	if generationErr == nil {
		generation, generationErr = prepareRuntimeGeneration(index, opts.SourceDigests, sources)
	}
	if generationErr != nil {
		return compileErrorRun(cases, generationErr, started, opts)
	}
	if emitProgress {
		reportProgress(opts, TestProgress{Event: "compile_start"})
	}
	if compileErr := semanticCompileErrorWithHooks(ctx, index, opts.BuildArtifacts, sources, generation, hooks, !opts.NoDiskCache && diskCacheEnabled(), runState.counters); compileErr != nil {
		return compileErrorRun(cases, compileErr, started, opts)
	}
	var runtimeKey runtimeCacheKey
	var runtime runtimeExecutionView
	var runtimeErr error
	if runState.counters.enabled {
		runtimeKey, runtime, runtimeErr = runtimeFromIndexWithPreparedGenerationAndPerfProjected(index, opts.SourceDigests, sources, &generation, runState.diskCacheEnabled, runState.counters, runtimeExecutionProjection)
	} else {
		runtimeKey, runtime, runtimeErr = runtimeFromIndexWithPreparedGenerationProjected(index, opts.SourceDigests, sources, &generation, runtimeExecutionProjection, runState.diskCacheEnabled)
	}
	if runtimeErr != nil {
		return compileErrorRun(cases, runtimeErr, started, opts)
	}
	var testCompileStarted time.Time
	if runState.counters.enabled {
		testCompileStarted = time.Now()
	}
	setups, setupErrors, setupInvokePrograms, setupInvokeErrors, setupSourceErr := compileTestSetupsCached(index, opts.SourceDigests, runtimeKey, testCaseClassSet(cases), sources)
	if setupSourceErr != nil {
		return compileErrorRun(cases, setupSourceErr, started, opts)
	}
	triggerErrors := runtime.TriggerErrors
	testMethods, testMethodErrors, testInvokePrograms, testInvokeErrors, testSourceErr := compileTestsCached(index, opts.SourceDigests, runtimeKey, cases, sources)
	if testSourceErr != nil {
		return compileErrorRun(cases, testSourceErr, started, opts)
	}
	if runState.counters.enabled {
		runState.counters.phases.testCompileNS.Add(time.Since(testCompileStarted).Nanoseconds())
	}
	if emitProgress {
		reportProgress(opts, TestProgress{Event: "compile_done", DurationMS: time.Since(started).Milliseconds(), Status: "pass"})
	}
	var orgSetupStarted time.Time
	if runState.counters.enabled {
		orgSetupStarted = time.Now()
	}
	recordStorageCloneRuntime(runState.counters)
	recordCloneReason("", "run-base", "org-template", runState.counters)
	org := runtime.restored.CloneOrg()
	initializeTestOrg(&org)
	if runState.counters.enabled {
		runState.counters.phases.orgBuildNS.Add(time.Since(orgSetupStarted).Nanoseconds())
	}
	recordCloneReason("", "run-base", "vm-runtime", runState.counters)
	machineCloneStarted := time.Now()
	baseMachine := runtime.restored.CloneMachine(nil)
	recordCloneRuntimeMachineDuration(time.Since(machineCloneStarted), runState.counters)
	baseMachine.SetPerfRecorder(runState.vmPerf)
	baseMachine.SetTraceEnabled(false)
	baseMachine.EnableTestContext()
	// Prime the schema stamp so per-test clones inherit it and reuse the shared
	// schema-describe caches instead of rebuilding them on every clone.
	if runState.counters.enabled {
		orgSetupStarted = time.Now()
	}
	baseMachine.PrimeMetadataSchema(&org)
	if runState.counters.enabled {
		runState.counters.phases.orgBuildNS.Add(time.Since(orgSetupStarted).Nanoseconds())
	}
	baseRuntimeErr := runtime.BaseErr
	if baseRuntimeErr == nil {
		if runState.counters.enabled {
			testCompileStarted = time.Now()
		}
		baseRuntimeErr = registerTestRuntime(baseMachine, append(flattenSetupMethods(setups), methodMapValues(testMethods)...))
		if runState.counters.enabled {
			runState.counters.phases.testCompileNS.Add(time.Since(testCompileStarted).Nanoseconds())
		}
	}
	// Freeze the alias/class lookup into a shared immutable index now that all
	// classes and test methods are registered. Per-test clones then share it by
	// pointer instead of rebuilding it on every CloneRuntime.
	var freezeStarted time.Time
	if runState.counters.enabled {
		freezeStarted = time.Now()
	}
	baseMachine.FreezeClassLookup()
	if runState.counters.enabled {
		runState.counters.phases.projectCompileNS.Add(time.Since(freezeStarted).Nanoseconds())
	}
	suites := make(map[string][]testreport.Case)
	order := make([]string, 0)
	classSeen := make(map[string]bool)
	suiteIndexes := make(map[string][]int)
	planned := make([]testCasePlan, len(cases))
	results := make([]testreport.Case, len(cases))
	for i, testCase := range cases {
		if !classSeen[testCase.ClassName] {
			classSeen[testCase.ClassName] = true
			order = append(order, testCase.ClassName)
			suites[testCase.ClassName] = nil
		}
		suiteIndexes[testCase.ClassName] = append(suiteIndexes[testCase.ClassName], i)
		if err := ctx.Err(); err != nil {
			results[i] = canceledCase(testCase, err)
			continue
		}
		planned[i] = testCasePlan{
			TestCase:      testCase,
			TestMethodErr: testMethodErrors[testCaseKey(testCase)],
			InvokeProgram: testInvokePrograms[testCaseKey(testCase)],
			InvokeProgErr: testInvokeErrors[testCaseKey(testCase)],
		}
	}
	if emitProgress {
		for i, testCase := range cases {
			if results[i].Status == "" {
				continue
			}
			reportProgress(opts, TestProgress{
				Event:      "test_done",
				ClassName:  testCase.ClassName,
				MethodName: testCase.MethodName,
				DurationMS: results[i].DurationMS,
				Status:     string(results[i].Status),
			})
		}
	}
	if noSetupFastPath(setups, setupErrors, setupInvokeErrors) && allClassesHaveSingleMethod(suiteIndexes) {
		for i := range planned {
			if results[i].Status != "" {
				continue
			}
			planned[i].SetupOrg = org
			planned[i].SetupShared = true
		}
		if len(baseMachine.Triggers) == 0 && allPlansSupportJournalIsolation(planned) {
			runNoSetupJournalPlans(ctx, planned, results, baseMachine, baseRuntimeErr, triggerErrors, org, opts, runState.counters)
			goto assemble
		}
		runTestPlans(ctx, planned, results, baseMachine, baseRuntimeErr, triggerErrors, opts, runState.counters)
		goto assemble
	}
	runTestPlansWithSetups(ctx, order, suiteIndexes, planned, results, baseMachine, baseRuntimeErr, setups, setupErrors, setupInvokePrograms, setupInvokeErrors, triggerErrors, org, opts, runState.counters)
assemble:
	for className, indexes := range suiteIndexes {
		for _, index := range indexes {
			suites[className] = append(suites[className], results[index])
		}
	}
	var reportStarted time.Time
	if runState.counters.enabled {
		reportStarted = time.Now()
	}

	run := testreport.Run{
		Name:       "glade test",
		DurationMS: time.Since(started).Milliseconds(),
	}
	for _, name := range order {
		run.Suites = append(run.Suites, testreport.Suite{Name: name, Cases: suites[name]})
	}
	if runState.counters.enabled {
		runState.counters.phases.reportAssemblyNS.Add(time.Since(reportStarted).Nanoseconds())
	}
	return run
}

func compileErrorRun(cases []TestCase, compileErr error, started time.Time, opts Options) testreport.Run {
	run := testreport.Run{
		Name:       "glade test",
		DurationMS: time.Since(started).Milliseconds(),
	}
	if len(cases) == 0 {
		cases = []TestCase{{
			ClassName:  "project",
			MethodName: "compile",
		}}
	}
	suiteIndexes := make(map[string]int)
	for _, testCase := range cases {
		index, ok := suiteIndexes[testCase.ClassName]
		if !ok {
			index = len(run.Suites)
			suiteIndexes[testCase.ClassName] = index
			run.Suites = append(run.Suites, testreport.Suite{Name: testCase.ClassName})
		}
		result := testreport.Case{
			ClassName:  testCase.ClassName,
			MethodName: testCase.MethodName,
			Status:     testreport.StatusCompileError,
			Problem:    problem("CompileError", compileErr.Error(), testCase),
		}
		run.Suites[index].Cases = append(run.Suites[index].Cases, result)
	}
	if opts.Progress != nil {
		reportProgress(opts, TestProgress{Event: "compile_done", DurationMS: time.Since(started).Milliseconds(), Status: "fail"})
		for _, testCase := range cases {
			reportProgress(opts, TestProgress{
				Event:      "test_done",
				ClassName:  testCase.ClassName,
				MethodName: testCase.MethodName,
				Status:     string(testreport.StatusCompileError),
			})
		}
	}
	return run
}

func indexCompileError(diagnostics []diagnostic.Diagnostic) error {
	var messages []string
	for _, diag := range diagnostics {
		if diag.Severity != diagnostic.Error {
			continue
		}
		var message strings.Builder
		if diag.File != "" {
			message.WriteString(diag.File)
			if diag.Range != nil && diag.Range.Start.Line > 0 {
				_, _ = fmt.Fprintf(&message, ":%d:%d", diag.Range.Start.Line, diag.Range.Start.Column)
			}
			message.WriteString(": ")
		}
		if diag.Code != "" {
			message.WriteString(diag.Code)
			message.WriteString(": ")
		}
		message.WriteString(diag.Message)
		messages = append(messages, message.String())
	}
	if len(messages) == 0 {
		return nil
	}
	return errors.New(strings.Join(messages, "\n"))
}

var (
	semaDiagnosticsCacheMu   sync.RWMutex
	semaDiagnosticsCache     = make(map[runtimeCacheKey][]diagnostic.Diagnostic)
	semanticResults          = semanticcache.NewManager(semanticcache.Limits{MaxEntries: 8, MaxBytes: 512 << 20})
	snapshotValidationHookMu sync.RWMutex
	snapshotValidationHook   func(string)
)

func setSnapshotValidationHookForTesting(hook func(string)) func() {
	snapshotValidationHookMu.Lock()
	previous := snapshotValidationHook
	snapshotValidationHook = hook
	snapshotValidationHookMu.Unlock()
	return func() {
		snapshotValidationHookMu.Lock()
		snapshotValidationHook = previous
		snapshotValidationHookMu.Unlock()
	}
}

func invokeSnapshotValidationHook(stage string) {
	snapshotValidationHookMu.RLock()
	hook := snapshotValidationHook
	snapshotValidationHookMu.RUnlock()
	if hook != nil {
		hook(stage)
	}
}

// semanticCompileError runs the same non-performance semantic analysis used
// by `glade check` (sema.AnalyzeWithOptions with SuppressPerformanceDiagnostics)
// before any test discovery, caching, or method compilation, so invalid Apex
// never executes as a test. index.Diagnostics (parse/index errors) are
// already included in the analyzer's Result.Diagnostics, so this is the one
// validator for both index and semantic error diagnostics; there is no
// separate/divergent check. The result is cached by the same source-digest
// identity used by the runtime compile caches, since semantic analysis is
// otherwise repeated on every call with unchanged sources.

func semanticCompileErrorWithHooks(ctx context.Context, index typesys.Index, artifacts *typesys.BuildArtifacts, sources *sourceCache, generation runtimeGeneration, hooks semanticGateHooks, cacheAllowed bool, counters ...*runPerfCounters) error {
	validateGeneration := func() error {
		if artifacts != nil {
			return validateCapturedBuildGeneration(index, artifacts)
		}
		return validateRuntimeGeneration(sources)
	}
	if err := validateGeneration(); err != nil {
		key := generation.key
		rememberRuntimeCacheRoot(index, key)
		evictSnapshotCaches(key)
		return err
	}
	if artifacts != nil {
		invokeSnapshotValidationHook("semantic_after_initial_validation")
	}
	perfCounters := perfCounterFor(counters)
	var keyStarted time.Time
	if perfCounters.enabled {
		keyStarted = time.Now()
	}
	key := generation.key
	rememberRuntimeCacheRoot(index, key)
	if hooks.afterIdentity != nil {
		hooks.afterIdentity()
	}
	if perfCounters.enabled {
		perfCounters.phases.semanticKeyNS.Add(time.Since(keyStarted).Nanoseconds())
	}
	var gateStarted time.Time
	if perfCounters.enabled {
		gateStarted = time.Now()
	}
	analyzeOptions := sema.AnalyzeOptions{
		Diagnostics:                    true,
		ExportTypes:                    true,
		SuppressPerformanceDiagnostics: true,
		BuildArtifacts:                 artifacts,
	}
	if artifacts == nil {
		analyzeOptions.CapturedSource = generation.source.capturedSource
	}
	analysisIndex := semanticAnalysisIndex(index)
	var diagnostics []diagnostic.Diagnostic
	if artifacts == nil {
		semaDiagnosticsCacheMu.RLock()
		cached, ok := semaDiagnosticsCache[key]
		semaDiagnosticsCacheMu.RUnlock()
		if ok && cacheAllowed {
			diagnostics = cached
			if perfCounters.enabled {
				perfCounters.phases.semanticMemoryCacheHits.Add(1)
			}
		} else {
			if perfCounters.enabled {
				perfCounters.phases.semanticCacheMisses.Add(1)
				perfCounters.phases.semanticBuilds.Add(1)
				perfCounters.phases.semanticAnalyses.Add(1)
			}
			result := sema.AnalyzeWithOptions(analysisIndex, analyzeOptions)
			if hooks.afterAnalysis != nil {
				hooks.afterAnalysis()
			}
			diagnostics = result.Diagnostics
			if err := validateGeneration(); err != nil {
				evictSnapshotCaches(key)
				return err
			}
			if cacheAllowed {
				semaDiagnosticsCacheMu.Lock()
				semaDiagnosticsCache[key] = diagnostics
				semaDiagnosticsCacheMu.Unlock()
			}
		}
	} else {
		identity, identityErr := semanticcache.IdentityForBuild(analysisIndex, artifacts, analyzeOptions)
		if identityErr != nil {
			return identityErr
		}
		cachePath := semanticResultCachePath(identity)
		result, access, cacheErr := semanticResults.GetOrCompute(ctx, semanticcache.Request{
			Identity:     identity,
			ProjectRoot:  index.Project.Root,
			RelativePath: cachePath,
			NoDisk:       !cacheAllowed,
			BypassMemory: !cacheAllowed,
		}, func() (sema.Result, error) {
			if perfCounters.enabled {
				perfCounters.phases.semanticCacheMisses.Add(1)
				perfCounters.phases.semanticBuilds.Add(1)
				perfCounters.phases.semanticAnalyses.Add(1)
			}
			analyzed := sema.AnalyzeWithOptions(analysisIndex, analyzeOptions)
			if hooks.afterAnalysis != nil {
				hooks.afterAnalysis()
			}
			invokeSnapshotValidationHook("semantic_before_cache_publication")
			if err := validateGeneration(); err != nil {
				return sema.Result{}, err
			}
			return analyzed, nil
		})
		if cacheErr != nil {
			if perfCounters.enabled {
				perfCounters.phases.semanticErrors.Add(1)
			}
			semanticResults.Evict(identity)
			evictSnapshotCaches(key)
			return cacheErr
		}
		if perfCounters.enabled {
			perfCounters.recordSemanticCache(identity, access)
			switch access.Source {
			case semanticcache.SourceMemory:
				perfCounters.phases.semanticMemoryCacheHits.Add(1)
			case semanticcache.SourceDisk:
				perfCounters.phases.semanticDiskCacheHits.Add(1)
			}
		}
		diagnostics = result.Diagnostics
		if cacheAllowed {
			semaDiagnosticsCacheMu.Lock()
			semaDiagnosticsCache[key] = append([]diagnostic.Diagnostic(nil), diagnostics...)
			semaDiagnosticsCacheMu.Unlock()
		}
	}
	if artifacts != nil {
		invokeSnapshotValidationHook("semantic_before_cache_return")
	}
	if err := validateGeneration(); err != nil {
		evictSnapshotCaches(key)
		return err
	}
	if perfCounters.enabled {
		perfCounters.phases.semanticGateNS.Add(time.Since(gateStarted).Nanoseconds())
	}
	return indexCompileError(diagnostics)
}

func semanticResultCachePath(identity semanticcache.Identity) string {
	return filepath.Join(".glade", "semantic", "result-"+identity.OptionsFingerprint+".json")
}

func rememberRuntimeCacheRoot(index typesys.Index, key runtimeCacheKey) {
	root := strings.TrimSpace(index.Project.Root)
	if root == "" {
		return
	}
	runtimeCacheRootMu.Lock()
	runtimeCacheRoots[key] = filepath.Clean(root)
	runtimeCacheRootMu.Unlock()
}

func evictSnapshotCaches(key runtimeCacheKey) {
	runtimeCacheMu.Lock()
	delete(runtimeCache, key)
	runtimeCacheMu.Unlock()
	semaDiagnosticsCacheMu.Lock()
	delete(semaDiagnosticsCache, key)
	semaDiagnosticsCacheMu.Unlock()
	prefix := string(key) + "|"
	setupCacheMu.Lock()
	for cacheKey := range setupCache {
		if strings.HasPrefix(cacheKey, prefix) {
			delete(setupCache, cacheKey)
		}
	}
	setupCacheMu.Unlock()
	testCacheMu.Lock()
	for cacheKey := range testCache {
		if strings.HasPrefix(cacheKey, prefix) {
			delete(testCache, cacheKey)
		}
	}
	testCacheMu.Unlock()
	runtimeCacheRootMu.Lock()
	root := runtimeCacheRoots[key]
	delete(runtimeCacheRoots, key)
	runtimeCacheRootMu.Unlock()
	if root != "" {
		_ = startupcache.Clear(root, startupcache.SubdirTest)
	}
}

// validateBuildArtifacts proves that an attached source arena and its digest
// set describe this exact index. A caller that supplies an artifact requests a
// closed snapshot: missing or differently-resolved logical occurrences must
// never fall back to the current filesystem.
func validateBuildArtifacts(index typesys.Index, artifacts *typesys.BuildArtifacts) error {
	if artifacts == nil {
		return nil
	}
	if artifacts.Sources == nil {
		return incompleteSourceSnapshotError("build artifacts are missing sources")
	}
	if artifacts.SourceDigests == nil {
		return incompleteSourceSnapshotError("build artifacts are missing digests")
	}
	if artifacts.ApexMetadataInputs == nil {
		return incompleteSourceSnapshotError("build artifacts are missing Apex metadata inputs")
	}
	seen := make(map[string]bool)
	for _, typ := range index.Types {
		if !typ.HasSourceSnapshot() {
			continue
		}
		key := sourceSnapshotValidationKey(typ.File, typ.SourceRoot, typ.Namespace, typ.Version, typ.Dependency, typ.SourceNamespaceRemaps)
		if seen[key] {
			continue
		}
		seen[key] = true
		if source, ok := artifacts.SourceForType(typ); !ok {
			return incompleteSourceSnapshotError("missing type source " + typ.File)
		} else if err := validateArtifactDigest(typ.File, source.Digest(), artifacts.SourceDigests); err != nil {
			return err
		}
		if _, ok := artifacts.ApexMetadataForType(typ); !ok {
			return incompleteSourceSnapshotError("missing Apex metadata input " + typ.File + "-meta.xml")
		}
	}
	for _, trigger := range index.Triggers {
		if !trigger.HasSourceSnapshot() {
			continue
		}
		key := sourceSnapshotValidationKey(trigger.File, trigger.SourceRoot, trigger.Namespace, trigger.Version, trigger.Dependency, trigger.SourceNamespaceRemaps)
		if seen[key] {
			continue
		}
		seen[key] = true
		if source, ok := artifacts.SourceForTrigger(trigger); !ok {
			return incompleteSourceSnapshotError("missing trigger source " + trigger.File)
		} else if err := validateArtifactDigest(trigger.File, source.Digest(), artifacts.SourceDigests); err != nil {
			return err
		}
		if _, ok := artifacts.ApexMetadataForTrigger(trigger); !ok {
			return incompleteSourceSnapshotError("missing Apex metadata input " + trigger.File + "-meta.xml")
		}
	}
	return nil
}

// validateCapturedBuildGeneration validates all source and Apex metadata
// inputs against an artifact snapshot. It is intentionally separate from the
// compiler source cache: callers use it immediately before cache returns and
// publications, while compilation consumes the captured arena bytes.
func validateCapturedBuildGeneration(index typesys.Index, artifacts *typesys.BuildArtifacts) error {
	if artifacts == nil {
		return nil
	}
	if err := validateBuildArtifacts(index, artifacts); err != nil {
		return err
	}
	seen := make(map[string]bool)
	validate := func(file, occurrence string, metadata typesys.ApexMetadataInput, metadataOK bool) error {
		if seen[occurrence] {
			return nil
		}
		seen[occurrence] = true
		if !metadataOK {
			return incompleteSourceSnapshotError("missing Apex metadata input " + file + "-meta.xml")
		}
		expected, ok := artifacts.SourceDigests.Digest(file)
		if !ok {
			return incompleteSourceSnapshotError("missing digest for " + file)
		}
		data, err := os.ReadFile(file) // #nosec G304 -- source path is bound to a BuildArtifacts occurrence.
		if err != nil {
			return sourceSnapshotMismatch(file, expected, nil, err)
		}
		actual := sha256.Sum256(data)
		if actual != expected {
			return sourceSnapshotMismatch(file, expected, &actual, nil)
		}
		return validateApexMetadataInput(file+"-meta.xml", metadata)
	}
	for _, typ := range index.Types {
		if typ.HasSourceSnapshot() {
			metadata, ok := artifacts.ApexMetadataForType(typ)
			occurrence := sourceSnapshotValidationKey(typ.File, typ.SourceRoot, typ.Namespace, typ.Version, typ.Dependency, typ.SourceNamespaceRemaps)
			if err := validate(typ.File, occurrence, metadata, ok); err != nil {
				return err
			}
		}
	}
	for _, trigger := range index.Triggers {
		if trigger.HasSourceSnapshot() {
			metadata, ok := artifacts.ApexMetadataForTrigger(trigger)
			occurrence := sourceSnapshotValidationKey(trigger.File, trigger.SourceRoot, trigger.Namespace, trigger.Version, trigger.Dependency, trigger.SourceNamespaceRemaps)
			if err := validate(trigger.File, occurrence, metadata, ok); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateApexMetadataInput(path string, expected typesys.ApexMetadataInput) error {
	data, err := os.ReadFile(path) // #nosec G304 -- fixed metadata companion for an indexed Apex source.
	if err != nil {
		if os.IsNotExist(err) && !expected.Present {
			return nil
		}
		return sourceSnapshotMismatch(path, expected.Digest, nil, err)
	}
	actual := sha256.Sum256(data)
	if !expected.Present || actual != expected.Digest {
		return sourceSnapshotMismatch(path, expected.Digest, &actual, nil)
	}
	return nil
}

func sourceSnapshotMismatch(file string, expected [sha256.Size]byte, actual *[sha256.Size]byte, cause error) error {
	mismatch := &SourceSnapshotMismatchError{File: file, ExpectedSHA256: hex.EncodeToString(expected[:]), Cause: cause}
	if actual != nil {
		mismatch.ActualSHA256 = hex.EncodeToString(actual[:])
	}
	return mismatch
}

func sourceSnapshotValidationKey(file, root, namespace, version string, dependency bool, remaps []namespaceremap.Rule) string {
	return strings.Join([]string{file, root, namespace, version, strconv.FormatBool(dependency), namespaceremap.Fingerprint(remaps)}, "\x00")
}

func validateArtifactDigest(file string, actual [sha256.Size]byte, digests *typesys.SourceDigestSet) error {
	expected, ok := digests.Digest(file)
	if !ok {
		return incompleteSourceSnapshotError("missing digest for " + file)
	}
	if expected == actual {
		return nil
	}
	return &SourceSnapshotMismatchError{
		File:           file,
		ExpectedSHA256: hex.EncodeToString(expected[:]),
		ActualSHA256:   hex.EncodeToString(actual[:]),
		Cause:          errors.New("source snapshot digest does not match its captured source"),
	}
}

func incompleteSourceSnapshotError(reason string) error {
	return &SourceSnapshotMismatchError{File: "build artifacts", Cause: errors.New("source snapshot is incomplete: " + reason)}
}

// semanticAnalysisIndex returns a copy of index whose Objects/Fields slices
// do not alias the caller's backing arrays. typesys.Index documents that
// nested payloads may be structurally shared between snapshots and must not
// be mutated after publication, but sema's schema-inference passes upsert
// inferred fields onto index.Objects[i].Fields in place. RunCasesContext
// reuses the same index for runtime/org construction after this check, so
// without this copy sema's inferred-field synthesis (e.g. a Name field
// missing the Required flag a full schema load would set) would leak into
// the org the test actually runs against.
func semanticAnalysisIndex(index typesys.Index) typesys.Index {
	if len(index.Objects) == 0 {
		return index
	}
	objects := make([]schema.Object, len(index.Objects))
	for i, object := range index.Objects {
		if len(object.Fields) > 0 {
			object.Fields = append([]schema.Field(nil), object.Fields...)
		}
		objects[i] = object
	}
	index.Objects = objects
	return index
}

func useDiskRuntimeCache(opts Options) bool {
	return ResolveDiskRuntimeCachePolicy(opts).Enabled
}

type testCasePlan struct {
	TestCase      TestCase
	TestMethodErr error
	InvokeProgram ir.Program
	InvokeProgErr error
	SetupErr      error
	SetupOrg      storage.OrgState
	SetupRandom   uint64
	SetupShared   bool
}

type testSetupResult struct {
	Org         storage.OrgState
	Err         error
	Random      uint64
	OrgIsShared bool
}

type runtimeCacheKey string

type runtimeCacheEntry struct {
	Methods                      map[string]vm.Method
	Classes                      []vm.Class
	Triggers                     []vm.Trigger
	TriggerErrors                []error
	PageNames                    []string
	BaseErr                      error
	restored                     vm.RestoredRuntimeTemplate
	patchAuthority               *runtimePatchAuthority
	executionProjectionValidated bool
}

// runtimeExecutionView is the immutable subset of a compiled runtime needed by
// RunCasesContext after project compilation. The full compiled payload remains
// cache-owned so runner startup does not deep-clone methods, classes, triggers,
// page names, or patch authority that it never reads.
type runtimeExecutionView struct {
	TriggerErrors []error
	BaseErr       error
	restored      vm.RestoredRuntimeTemplate
}

type CompiledProjectRuntime struct {
	Methods   map[string]vm.Method `json:"methods,omitempty"`
	Classes   []vm.Class           `json:"classes,omitempty"`
	Triggers  []vm.Trigger         `json:"triggers,omitempty"`
	PageNames []string             `json:"pageNames,omitempty"`
}

type setupCompileCacheEntry struct {
	Methods     map[string][]vm.Method
	Errors      map[string]error
	Programs    map[string][]ir.Program
	ProgramErrs map[string]error
}

type testCompileCacheEntry struct {
	Methods     map[string]vm.Method
	MethodErrs  map[string]error
	Programs    map[string]ir.Program
	ProgramErrs map[string]error
}

var (
	runtimeCacheMu     sync.RWMutex
	runtimeCache       = make(map[runtimeCacheKey]runtimeCacheEntry)
	runtimeCacheRootMu sync.Mutex
	runtimeCacheRoots  = make(map[runtimeCacheKey]string)
	setupCacheMu       sync.RWMutex
	setupCache         = make(map[string]setupCompileCacheEntry)
	testCacheMu        sync.RWMutex
	testCache          = make(map[string]testCompileCacheEntry)
)

func runtimeKey(index typesys.Index) runtimeCacheKey {
	return runtimeContentKey(index, nil)
}

// RuntimeContentKey returns the exact key used by compiled-runtime caches.
func RuntimeContentKey(index typesys.Index, digests *typesys.SourceDigestSet) string {
	return string(runtimeContentKey(index, digests))
}

func runtimeContentKey(index typesys.Index, digests *typesys.SourceDigestSet) runtimeCacheKey {
	sources := newSourceCache()
	resolvedDigests, err := authoritativeRuntimeSourceDigests(index, digests)
	if err == nil {
		if generation, generationErr := prepareRuntimeGeneration(index, resolvedDigests, sources); generationErr == nil {
			return generation.key
		}
	}
	// RuntimeContentKey predates generation validation and cannot return an
	// error. Preserve its deterministic fallback for incomplete or concurrently
	// changing inputs; runtime construction still rejects those inputs before
	// cache lookup or publication.
	return runtimeKeyWithSourceDigests(index, digests, os.ReadFile)
}

func runtimeKeyWithSourceDigests(index typesys.Index, digests *typesys.SourceDigestSet, readFile func(string) ([]byte, error)) runtimeCacheKey {
	var lookup func(string) ([sha256.Size]byte, bool)
	if digests != nil {
		lookup = digests.Digest
	}
	return runtimeKeyWithDigestLookup(index, lookup, readFile)
}

func runtimeKeyWithDigestLookup(index typesys.Index, lookup func(string) ([sha256.Size]byte, bool), readFile func(string) ([]byte, error)) runtimeCacheKey {
	if readFile == nil {
		readFile = os.ReadFile
	}
	h := fnv.New128a()
	write := func(s string) {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	seenFiles := make(map[string]bool)
	for _, typ := range index.Types {
		if typ.HasSourceSnapshot() && typ.File != "" {
			seenFiles[typ.File] = true
		}
	}
	for _, trigger := range index.Triggers {
		if trigger.HasSourceSnapshot() && trigger.File != "" {
			seenFiles[trigger.File] = true
		}
	}
	useSnapshot := lookup != nil
	if useSnapshot {
		for file := range seenFiles {
			if _, ok := lookup(file); !ok {
				useSnapshot = false
				break
			}
		}
	}
	clear(seenFiles)
	writeFileBody := func(file string) {
		if file == "" || seenFiles[file] {
			return
		}
		seenFiles[file] = true
		write("file:" + file)
		var digest [sha256.Size]byte
		if useSnapshot {
			digest, _ = lookup(file)
		} else {
			data, err := readFile(file)
			if err != nil {
				write("read-error:" + err.Error())
				return
			}
			digest = sha256.Sum256(data)
		}
		_, _ = h.Write(digest[:])
		_, _ = h.Write([]byte{0})
	}
	write(index.Project.Root)
	write(index.Project.Namespace)
	write(index.Project.SourceAPIVersion)
	write(fmt.Sprintf("types:%d triggers:%d objects:%d cmdt:%d", len(index.Types), len(index.Triggers), len(index.Objects), len(index.CustomMetadataRecords)))
	semanticMetadata, err := json.Marshal(struct {
		Objects               []schema.Object               `json:"objects"`
		CustomMetadataRecords []schema.CustomMetadataRecord `json:"customMetadataRecords,omitempty"`
		Dependencies          []typesys.DependencyInfo      `json:"dependencies,omitempty"`
		Diagnostics           []diagnostic.Diagnostic       `json:"diagnostics,omitempty"`
	}{
		Objects:               index.Objects,
		CustomMetadataRecords: index.CustomMetadataRecords,
		Dependencies:          index.Dependencies,
		Diagnostics:           index.Diagnostics,
	})
	if err != nil {
		write("semantic-metadata-error:" + err.Error())
	} else {
		_, _ = h.Write(semanticMetadata)
		_, _ = h.Write([]byte{0})
	}
	for _, typ := range index.Types {
		write(typ.File)
		write(typ.Name)
		write(typ.Namespace)
		write(typ.SourceRoot)
		write(typ.Version)
		write(typ.EffectiveAPIVersion)
		write(fmt.Sprintf("dependency:%t artifact:%t sourceBacked:%t", typ.Dependency, typ.Artifact, typ.HasSourceSnapshot()))
		write(namespaceremap.Fingerprint(typ.SourceNamespaceRemaps))
		if typ.HasSourceSnapshot() {
			writeFileBody(typ.File)
		}
	}
	for _, trig := range index.Triggers {
		write(trig.File)
		write(trig.Name)
		write(trig.ObjectName)
		write(trig.SourceRoot)
		write(trig.Version)
		write(trig.EffectiveAPIVersion)
		write(fmt.Sprintf("dependency:%t sourceBacked:%t", trig.Dependency, trig.HasSourceSnapshot()))
		write(namespaceremap.Fingerprint(trig.SourceNamespaceRemaps))
		if trig.HasSourceSnapshot() {
			writeFileBody(trig.File)
		}
	}
	return runtimeCacheKey(hex.EncodeToString(h.Sum(nil)))
}

type runtimeMetadataAuthority string

const (
	runtimeMetadataAuthorityLegacy   runtimeMetadataAuthority = "legacy-live"
	runtimeMetadataAuthorityArtifact runtimeMetadataAuthority = "build-artifacts"
)

type runtimeMetadataGeneration struct {
	authority   runtimeMetadataAuthority
	inputs      map[string]typesys.ApexMetadataInput
	apiVersions map[string]string
}

type runtimeSourceInput struct {
	digest [sha256.Size]byte
	raw    string
}

type runtimeSourceGeneration struct {
	inputs map[string]runtimeSourceInput
}

func (generation runtimeSourceGeneration) capturedSource(file string) (string, bool) {
	input, ok := generation.inputs[file]
	return input.raw, ok
}

type runtimeGeneration struct {
	source   runtimeSourceGeneration
	metadata runtimeMetadataGeneration
	key      runtimeCacheKey
}

func runtimeKeyWithSourceGeneration(index typesys.Index, generation runtimeSourceGeneration) runtimeCacheKey {
	lookup := func(file string) ([sha256.Size]byte, bool) {
		input, ok := generation.inputs[file]
		return input.digest, ok
	}
	readCaptured := func(file string) ([]byte, error) {
		input, ok := generation.inputs[file]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(input.raw), nil
	}
	return runtimeKeyWithDigestLookup(index, lookup, readCaptured)
}

func runtimeKeyWithMetadataGeneration(base runtimeCacheKey, generation runtimeMetadataGeneration) runtimeCacheKey {
	h := fnv.New128a()
	write := func(value string) {
		_, _ = h.Write([]byte(value))
		_, _ = h.Write([]byte{0})
	}
	write("apextest-runtime-metadata-v1")
	write(string(base))
	write(string(generation.authority))
	paths := make([]string, 0, len(generation.inputs))
	for path := range generation.inputs {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		input := generation.inputs[path]
		write(path)
		write(strconv.FormatBool(input.Present))
		_, _ = h.Write(input.Digest[:])
		_, _ = h.Write([]byte{0})
		write(generation.apiVersions[strings.TrimSuffix(path, "-meta.xml")])
	}
	return runtimeCacheKey(hex.EncodeToString(h.Sum(nil)))
}

func cloneRuntimeCacheEntry(in runtimeCacheEntry) runtimeCacheEntry {
	out, _ := cloneRuntimeCacheEntryChecked(in)
	return out
}

func cloneRuntimeCacheEntryChecked(in runtimeCacheEntry) (runtimeCacheEntry, bool) {
	methods, ok := runtimePatchCloneMethods(in.Methods)
	if !ok {
		return runtimeCacheEntry{}, false
	}
	var classes []vm.Class
	if in.Classes != nil {
		classes = make([]vm.Class, len(in.Classes))
	}
	for i, class := range in.Classes {
		classes[i], ok = runtimePatchCloneClass(class)
		if !ok {
			return runtimeCacheEntry{}, false
		}
	}
	var triggers []vm.Trigger
	if in.Triggers != nil {
		triggers = make([]vm.Trigger, len(in.Triggers))
	}
	for i, trigger := range in.Triggers {
		program, programOK := runtimePatchCloneProgram(trigger.Program, make(map[*ir.Expr]bool), 0)
		if !programOK {
			return runtimeCacheEntry{}, false
		}
		triggers[i] = trigger
		triggers[i].Program = program
	}
	var authority *runtimePatchAuthority
	if in.patchAuthority != nil {
		copied := *in.patchAuthority
		if in.patchAuthority.sourceReferences != nil {
			copied.sourceReferences = make(map[string]string, len(in.patchAuthority.sourceReferences))
			for path, reference := range in.patchAuthority.sourceReferences {
				copied.sourceReferences[path] = reference
			}
		}
		copied.affected = runtimePatchCloneSlice(in.patchAuthority.affected)
		authority = &copied
	}
	return runtimeCacheEntry{
		Methods:        methods,
		Classes:        classes,
		Triggers:       triggers,
		TriggerErrors:  runtimePatchCloneSlice(in.TriggerErrors),
		PageNames:      runtimePatchCloneSlice(in.PageNames),
		BaseErr:        in.BaseErr,
		restored:       in.restored,
		patchAuthority: authority,
		// A successful full structural clone is itself a validation boundary.
		executionProjectionValidated: true,
	}, true
}

func runtimeExecutionViewFromEntry(in runtimeCacheEntry) (runtimeExecutionView, bool) {
	if !in.restored.Valid() {
		return runtimeExecutionView{}, false
	}
	return runtimeExecutionView{
		TriggerErrors: runtimePatchCloneSlice(in.TriggerErrors),
		BaseErr:       in.BaseErr,
		restored:      in.restored,
	}, true
}

func runtimeExecutionProjection(in runtimeCacheEntry) (runtimeExecutionView, bool) {
	// The marker is internal and never serialized. Only compiler-owned entries,
	// structurally validated disk entries, and successful full clones receive
	// it. Hand-injected or malformed entries therefore fail closed.
	if !in.executionProjectionValidated {
		return runtimeExecutionView{}, false
	}
	return runtimeExecutionViewFromEntry(in)
}

func runtimeCacheEntryUsable(entry runtimeCacheEntry) bool {
	return entry.restored.Valid() && (entry.patchAuthority == nil || runtimePatchAuthorityMatchesPayload(entry))
}

func validMemoryRuntimeEntry(key runtimeCacheKey) (runtimeCacheEntry, bool) {
	return validMemoryRuntimeProjection(key, cloneRuntimeCacheEntryChecked)
}

func validMemoryRuntimeExecution(key runtimeCacheKey) (runtimeExecutionView, bool) {
	return validMemoryRuntimeProjection(key, runtimeExecutionProjection)
}

func validMemoryRuntimeProjection[T any](key runtimeCacheKey, project func(runtimeCacheEntry) (T, bool)) (T, bool) {
	runtimeCacheMu.RLock()
	cached, ok := runtimeCache[key]
	if ok && runtimeCacheEntryUsable(cached) {
		projected, projectedOK := project(cached)
		runtimeCacheMu.RUnlock()
		if projectedOK {
			return projected, true
		}
		return recheckMemoryRuntimeProjectionAfterInvalidObservation(key, project)
	}
	runtimeCacheMu.RUnlock()
	if !ok {
		var zero T
		return zero, false
	}
	return recheckMemoryRuntimeProjectionAfterInvalidObservation(key, project)
}

func recheckMemoryRuntimeEntryAfterInvalidObservation(key runtimeCacheKey) (runtimeCacheEntry, bool) {
	return recheckMemoryRuntimeProjectionAfterInvalidObservation(key, cloneRuntimeCacheEntryChecked)
}

func recheckMemoryRuntimeProjectionAfterInvalidObservation[T any](key runtimeCacheKey, project func(runtimeCacheEntry) (T, bool)) (T, bool) {
	// Recheck under the write lock so an invalid observation cannot evict a
	// concurrently published valid replacement.
	runtimeCacheMu.Lock()
	cached, ok := runtimeCache[key]
	var projected T
	if ok && runtimeCacheEntryUsable(cached) {
		var projectedOK bool
		projected, projectedOK = project(cached)
		if !projectedOK {
			delete(runtimeCache, key)
			ok = false
		}
	} else if ok {
		delete(runtimeCache, key)
		ok = false
	}
	runtimeCacheMu.Unlock()
	if !ok {
		var zero T
		return zero, false
	}
	return projected, true
}

func authoritativeRuntimeSourceDigests(index typesys.Index, digests *typesys.SourceDigestSet) (*typesys.SourceDigestSet, error) {
	if digests == nil {
		return nil, nil
	}
	seen := make(map[string]bool)
	complete := true
	var mismatch error
	check := func(file string) {
		if file == "" || seen[file] {
			return
		}
		seen[file] = true
		supplied, ok := digests.Digest(file)
		if !ok {
			complete = false
			return
		}
		expected, indexOK := index.SourceDigest(file)
		if !indexOK {
			complete = false
			return
		}
		if supplied != expected && mismatch == nil {
			mismatch = sourceSnapshotMismatch(file, expected, &supplied, errors.New("source digest does not match the index generation"))
		}
	}
	for _, typ := range index.Types {
		if typ.HasSourceSnapshot() {
			check(typ.File)
		}
	}
	for _, trigger := range index.Triggers {
		if trigger.HasSourceSnapshot() {
			check(trigger.File)
		}
	}
	if !complete {
		return nil, incompleteSourceSnapshotError("missing digest for a source-backed index occurrence")
	}
	if mismatch != nil {
		return nil, mismatch
	}
	return digests, nil
}

func preloadRuntimeSources(index typesys.Index, sources *sourceCache) error {
	seen := make(map[string]bool)
	preload := func(file string) error {
		if file == "" || seen[file] {
			return nil
		}
		seen[file] = true
		_, err := sources.read(file)
		return err
	}
	for _, typ := range index.Types {
		if !typ.HasSourceSnapshot() {
			continue
		}
		if err := preload(typ.File); err != nil {
			return err
		}
	}
	for _, trigger := range index.Triggers {
		if !trigger.HasSourceSnapshot() {
			continue
		}
		if err := preload(trigger.File); err != nil {
			return err
		}
	}
	return sources.sourceSnapshotError()
}

func validateRuntimeSourceDigests(index typesys.Index, digests *typesys.SourceDigestSet) error {
	if digests == nil {
		return nil
	}
	seen := make(map[string]bool)
	validate := func(file string) error {
		if file == "" || seen[file] {
			return nil
		}
		seen[file] = true
		expected, ok := digests.Digest(file)
		if !ok {
			return incompleteSourceSnapshotError("missing digest for " + file)
		}
		data, err := os.ReadFile(file) // #nosec G304 -- indexed source path.
		if err != nil {
			return sourceSnapshotMismatch(file, expected, nil, err)
		}
		actual := sha256.Sum256(data)
		if actual != expected {
			return sourceSnapshotMismatch(file, expected, &actual, nil)
		}
		return nil
	}
	for _, typ := range index.Types {
		if typ.HasSourceSnapshot() {
			if err := validate(typ.File); err != nil {
				return err
			}
		}
	}
	for _, trigger := range index.Triggers {
		if trigger.HasSourceSnapshot() {
			if err := validate(trigger.File); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateDigestGeneration(index typesys.Index, digests *typesys.SourceDigestSet) error {
	if err := validateRuntimeSourceDigests(index, digests); err != nil {
		return err
	}
	if digests == nil {
		return nil
	}
	for _, typ := range index.Types {
		if !typ.HasSourceSnapshot() {
			continue
		}
		input, ok := index.ApexMetadataForType(typ)
		if !ok {
			return incompleteSourceSnapshotError("missing Apex metadata input " + typ.File + "-meta.xml")
		}
		if err := validateApexMetadataInput(typ.File+"-meta.xml", input); err != nil {
			return err
		}
	}
	for _, trigger := range index.Triggers {
		if !trigger.HasSourceSnapshot() {
			continue
		}
		input, ok := index.ApexMetadataForTrigger(trigger)
		if !ok {
			return incompleteSourceSnapshotError("missing Apex metadata input " + trigger.File + "-meta.xml")
		}
		if err := validateApexMetadataInput(trigger.File+"-meta.xml", input); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeGeneration(sources *sourceCache) error {
	// Source and companion metadata bytes both come from the immutable
	// per-invocation generation captured in sources. Revalidate that one
	// authority at every cache return and publication boundary.
	return sources.validateCapturedSourceGenerationRaw()
}

func prepareRuntimeGeneration(index typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache) (runtimeGeneration, error) {
	if sources == nil {
		return runtimeGeneration{}, fmt.Errorf("source cache is nil")
	}
	sources.configureNamespaceRemaps(index.Types, index.Triggers)
	sources.bindSourceDigests(digests)
	source, err := sources.prepareSourceGeneration(index, digests)
	if err != nil {
		return runtimeGeneration{}, err
	}
	metadata, err := sources.prepareMetadataGeneration(index)
	if err != nil {
		return runtimeGeneration{}, err
	}
	sources.generationValidator = func() error { return validateRuntimeGeneration(sources) }
	if err := sources.validateCapturedSourceGeneration(); err != nil {
		return runtimeGeneration{}, err
	}
	base := runtimeKeyWithSourceGeneration(index, source)
	return runtimeGeneration{
		source:   source,
		metadata: metadata,
		key:      runtimeKeyWithMetadataGeneration(base, metadata),
	}, nil
}

func runtimeFromIndex(index typesys.Index, sources *sourceCache, useDiskCache ...bool) (runtimeCacheKey, runtimeCacheEntry) {
	key, entry, _ := runtimeFromIndexWithSourceDigests(index, nil, sources, useDiskCache...)
	return key, entry
}

func runtimeFromIndexWithSourceDigests(index typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, useDiskCache ...bool) (runtimeCacheKey, runtimeCacheEntry, error) {
	return runtimeFromIndexWithSourceDigestsProjected(index, digests, sources, cloneRuntimeCacheEntryChecked, useDiskCache...)
}

func runtimeFromIndexForExecutionWithSourceDigests(index typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, useDiskCache ...bool) (runtimeCacheKey, runtimeExecutionView, error) {
	return runtimeFromIndexWithSourceDigestsProjected(index, digests, sources, runtimeExecutionProjection, useDiskCache...)
}

func runtimeFromIndexWithSourceDigestsProjected[T any](index typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, project func(runtimeCacheEntry) (T, bool), useDiskCache ...bool) (runtimeCacheKey, T, error) {
	return runtimeFromIndexWithPreparedGenerationProjected(index, digests, sources, nil, project, useDiskCache...)
}

func runtimeFromIndexWithPreparedGenerationProjected[T any](index typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, prepared *runtimeGeneration, project func(runtimeCacheEntry) (T, bool), useDiskCache ...bool) (runtimeCacheKey, T, error) {
	var zero T
	var err error
	digests, err = authoritativeRuntimeSourceDigests(index, digests)
	if err != nil {
		key := runtimeKeyWithSourceDigests(index, nil, os.ReadFile)
		rememberRuntimeCacheRoot(index, key)
		evictSnapshotCaches(key)
		return key, zero, err
	}
	if sources == nil {
		sources = newSourceCache()
	}
	var generation runtimeGeneration
	if prepared == nil {
		generation, err = prepareRuntimeGeneration(index, digests, sources)
		if err != nil {
			key := runtimeKeyWithSourceDigests(index, digests, os.ReadFile)
			rememberRuntimeCacheRoot(index, key)
			evictSnapshotCaches(key)
			return key, zero, err
		}
	} else {
		generation = *prepared
	}
	if err := sources.validateCapturedSourceGeneration(); err != nil {
		key := generation.key
		rememberRuntimeCacheRoot(index, key)
		evictSnapshotCaches(key)
		return key, zero, err
	}
	invokeSnapshotValidationHook("runtime_after_initial_validation")
	diskCacheAllowed := diskCacheEnabled()
	if len(useDiskCache) > 0 {
		diskCacheAllowed = useDiskCache[0]
	}
	key := generation.key
	rememberRuntimeCacheRoot(index, key)
	if cached, ok := validMemoryRuntimeProjection(key, project); ok {
		invokeSnapshotValidationHook("runtime_before_memory_cache_return")
		if err := sources.validateCapturedSourceGeneration(); err != nil {
			evictSnapshotCaches(key)
			return key, zero, err
		}
		return key, cached, nil
	}

	var lookupInput *startupcache.ValidatedInput
	if diskCacheAllowed {
		lookupInput, _ = validatedDiskRuntimeInput(index, digests)
		if diskEntry, ok := tryLoadDiskRuntimeWithValidatedInput(index, key, lookupInput); ok {
			if projected, projectOK := project(diskEntry); projectOK {
				invokeSnapshotValidationHook("runtime_before_disk_cache_return")
				if err := sources.validateCapturedSourceGeneration(); err != nil {
					evictSnapshotCaches(key)
					return key, zero, err
				}
				invokeSnapshotValidationHook("runtime_before_memory_cache_publication")
				if err := sources.validateCapturedSourceGeneration(); err != nil {
					evictSnapshotCaches(key)
					return key, zero, err
				}
				runtimeCacheMu.Lock()
				runtimeCache[key] = diskEntry
				runtimeCacheMu.Unlock()
				return key, projected, nil
			}
		}
	}
	if digests != nil {
		if err := preloadRuntimeSources(index, sources); err != nil {
			return key, zero, err
		}
	}

	methods := compileProjectMethods(index, sources)
	classes := compileProjectClasses(index, methods, sources)
	triggers, triggerErrors := compileProjectTriggers(index, sources)
	org := orgFromIndex(index, sources)
	pageNames := visualforcePageNames(index)
	baseMachine := vm.New(nil)
	baseMachine.SetTraceEnabled(false)
	registerVisualforcePages(baseMachine, pageNames)
	baseErr := registerBaseRuntime(baseMachine, methods, classes, triggers)
	if err := sources.sourceSnapshotError(); err != nil {
		return key, zero, err
	}
	entry := runtimeCacheEntry{
		Methods:                      methods,
		Classes:                      classes,
		Triggers:                     triggers,
		TriggerErrors:                triggerErrors,
		PageNames:                    pageNames,
		BaseErr:                      baseErr,
		restored:                     vm.NewRestoredRuntimeTemplate(org, baseMachine),
		executionProjectionValidated: true,
	}
	entry.patchAuthority = newRuntimePatchAuthority(index, key, digests, sources, entry, org)
	projected, projectOK := project(entry)
	if !projectOK {
		return key, zero, fmt.Errorf("compiled runtime payload cannot be cloned safely")
	}
	invokeSnapshotValidationHook("runtime_before_memory_cache_publication")
	if err := sources.validateCapturedSourceGeneration(); err != nil {
		evictSnapshotCaches(key)
		return key, zero, err
	}
	runtimeCacheMu.Lock()
	runtimeCache[key] = entry
	runtimeCacheMu.Unlock()
	if diskCacheAllowed {
		invokeSnapshotValidationHook("runtime_before_disk_cache_publication")
		if err := sources.validateCapturedSourceGeneration(); err != nil {
			evictSnapshotCaches(key)
			return key, zero, err
		}
		if publicationInput, ok := validatedDiskRuntimeInputForPublication(index, digests, lookupInput); ok {
			persistDiskRuntimeWithValidatedInput(index, key, org, entry, publicationInput)
		}
	}
	return key, projected, nil
}

func runtimeFromIndexWithPerf(index typesys.Index, sources *sourceCache, diskCacheAllowed bool, counters *runPerfCounters) (runtimeCacheKey, runtimeCacheEntry) {
	key, entry, _ := runtimeFromIndexWithSourceDigestsAndPerf(index, nil, sources, diskCacheAllowed, counters)
	return key, entry
}

func runtimeFromIndexWithSourceDigestsAndPerf(index typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, diskCacheAllowed bool, counters *runPerfCounters) (runtimeCacheKey, runtimeCacheEntry, error) {
	return runtimeFromIndexWithSourceDigestsAndPerfProjected(index, digests, sources, diskCacheAllowed, counters, cloneRuntimeCacheEntryChecked)
}

func runtimeFromIndexForExecutionWithSourceDigestsAndPerf(index typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, diskCacheAllowed bool, counters *runPerfCounters) (runtimeCacheKey, runtimeExecutionView, error) {
	return runtimeFromIndexWithSourceDigestsAndPerfProjected(index, digests, sources, diskCacheAllowed, counters, runtimeExecutionProjection)
}

func runtimeFromIndexWithSourceDigestsAndPerfProjected[T any](index typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, diskCacheAllowed bool, counters *runPerfCounters, project func(runtimeCacheEntry) (T, bool)) (runtimeCacheKey, T, error) {
	return runtimeFromIndexWithPreparedGenerationAndPerfProjected(index, digests, sources, nil, diskCacheAllowed, counters, project)
}

func runtimeFromIndexWithPreparedGenerationAndPerfProjected[T any](index typesys.Index, digests *typesys.SourceDigestSet, sources *sourceCache, prepared *runtimeGeneration, diskCacheAllowed bool, counters *runPerfCounters, project func(runtimeCacheEntry) (T, bool)) (runtimeCacheKey, T, error) {
	var zero T
	var err error
	digests, err = authoritativeRuntimeSourceDigests(index, digests)
	if err != nil {
		key := runtimeKeyWithSourceDigests(index, nil, os.ReadFile)
		rememberRuntimeCacheRoot(index, key)
		evictSnapshotCaches(key)
		return key, zero, err
	}
	if sources == nil {
		sources = newSourceCache()
	}
	var generation runtimeGeneration
	if prepared == nil {
		generation, err = prepareRuntimeGeneration(index, digests, sources)
		if err != nil {
			key := runtimeKeyWithSourceDigests(index, digests, os.ReadFile)
			rememberRuntimeCacheRoot(index, key)
			evictSnapshotCaches(key)
			return key, zero, err
		}
	} else {
		generation = *prepared
	}
	if err := sources.validateCapturedSourceGeneration(); err != nil {
		key := generation.key
		rememberRuntimeCacheRoot(index, key)
		evictSnapshotCaches(key)
		return key, zero, err
	}
	invokeSnapshotValidationHook("runtime_after_initial_validation")
	keyStarted := time.Now()
	key := generation.key
	rememberRuntimeCacheRoot(index, key)
	counters.phases.runtimeKeyNS.Add(time.Since(keyStarted).Nanoseconds())
	if cached, ok := validMemoryRuntimeProjection(key, project); ok {
		invokeSnapshotValidationHook("runtime_before_memory_cache_return")
		if err := sources.validateCapturedSourceGeneration(); err != nil {
			evictSnapshotCaches(key)
			return key, zero, err
		}
		counters.phases.memoryCacheHits.Add(1)
		return key, cached, nil
	}

	var lookupInput *startupcache.ValidatedInput
	if diskCacheAllowed {
		lookupStarted := time.Now()
		lookupInput, _ = validatedDiskRuntimeInput(index, digests)
		counters.phases.cacheValidateNS.Add(time.Since(lookupStarted).Nanoseconds())
		if diskEntry, ok := tryLoadDiskRuntimeWithPerfValidatedInput(index, key, lookupInput, counters); ok {
			if projected, projectOK := project(diskEntry); projectOK {
				invokeSnapshotValidationHook("runtime_before_disk_cache_return")
				if err := sources.validateCapturedSourceGeneration(); err != nil {
					evictSnapshotCaches(key)
					return key, zero, err
				}
				invokeSnapshotValidationHook("runtime_before_memory_cache_publication")
				if err := sources.validateCapturedSourceGeneration(); err != nil {
					evictSnapshotCaches(key)
					return key, zero, err
				}
				runtimeCacheMu.Lock()
				runtimeCache[key] = diskEntry
				runtimeCacheMu.Unlock()
				counters.phases.diskCacheHits.Add(1)
				return key, projected, nil
			}
		}
	}
	counters.phases.cacheMisses.Add(1)
	if digests != nil {
		bindingStarted := time.Now()
		err := preloadRuntimeSources(index, sources)
		counters.phases.runtimeKeyNS.Add(time.Since(bindingStarted).Nanoseconds())
		if err != nil {
			return key, zero, err
		}
	}

	compileStarted := time.Now()
	methods := compileProjectMethods(index, sources)
	classes := compileProjectClasses(index, methods, sources)
	triggers, triggerErrors := compileProjectTriggers(index, sources)
	counters.phases.projectCompileNS.Add(time.Since(compileStarted).Nanoseconds())

	orgStarted := time.Now()
	org := orgFromIndex(index, sources)
	counters.phases.orgBuildNS.Add(time.Since(orgStarted).Nanoseconds())

	compileStarted = time.Now()
	pageNames := visualforcePageNames(index)
	baseMachine := vm.New(nil)
	baseMachine.SetTraceEnabled(false)
	registerVisualforcePages(baseMachine, pageNames)
	baseErr := registerBaseRuntime(baseMachine, methods, classes, triggers)
	counters.phases.projectCompileNS.Add(time.Since(compileStarted).Nanoseconds())
	restoredStarted := time.Now()
	restored := vm.NewRestoredRuntimeTemplate(org, baseMachine)
	counters.phases.orgBuildNS.Add(time.Since(restoredStarted).Nanoseconds())
	if err := sources.sourceSnapshotError(); err != nil {
		return key, zero, err
	}
	authorityStarted := time.Now()
	entry := runtimeCacheEntry{
		Methods:                      methods,
		Classes:                      classes,
		Triggers:                     triggers,
		TriggerErrors:                triggerErrors,
		PageNames:                    pageNames,
		BaseErr:                      baseErr,
		restored:                     restored,
		executionProjectionValidated: true,
	}
	entry.patchAuthority = newRuntimePatchAuthorityWithPerf(index, key, digests, sources, entry, org, counters)
	counters.phases.projectCompileNS.Add(time.Since(authorityStarted).Nanoseconds())
	projected, projectOK := project(entry)
	if !projectOK {
		return key, zero, fmt.Errorf("compiled runtime payload cannot be cloned safely")
	}
	invokeSnapshotValidationHook("runtime_before_memory_cache_publication")
	if err := sources.validateCapturedSourceGeneration(); err != nil {
		evictSnapshotCaches(key)
		return key, zero, err
	}
	runtimeCacheMu.Lock()
	runtimeCache[key] = entry
	runtimeCacheMu.Unlock()
	if diskCacheAllowed {
		invokeSnapshotValidationHook("runtime_before_disk_cache_publication")
		if err := sources.validateCapturedSourceGeneration(); err != nil {
			evictSnapshotCaches(key)
			return key, zero, err
		}
		publicationStarted := time.Now()
		publicationInput, ok := validatedDiskRuntimeInputForPublication(index, digests, lookupInput)
		counters.phases.cacheValidateNS.Add(time.Since(publicationStarted).Nanoseconds())
		if ok {
			persistDiskRuntimeWithPerfValidatedInput(index, key, org, entry, publicationInput, counters)
		}
	}
	return key, projected, nil
}

func classSetKey(classes map[string]bool) string {
	if len(classes) == 0 {
		return ""
	}
	// Order-independent signature to avoid map->slice->sort overhead.
	var acc uint64
	for name := range classes {
		h := fnv.New64a()
		_, _ = h.Write([]byte(name))
		acc ^= h.Sum64()
	}
	return fmt.Sprintf("%016x:%d", acc, len(classes))
}

func caseSetKey(cases []TestCase) string {
	if len(cases) == 0 {
		return ""
	}
	h := fnv.New128a()
	for _, tc := range cases {
		_, _ = h.Write([]byte(tc.ClassName))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(tc.MethodName))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func noSetupFastPath(setups map[string][]vm.Method, setupErrors map[string]error, setupInvokeErrors map[string]error) bool {
	for _, methods := range setups {
		if len(methods) > 0 {
			return false
		}
	}
	for _, err := range setupErrors {
		if err != nil {
			return false
		}
	}
	for _, err := range setupInvokeErrors {
		if err != nil {
			return false
		}
	}
	return true
}

func allClassesHaveSingleMethod(classIndexes map[string][]int) bool {
	for _, indexes := range classIndexes {
		if len(indexes) > 1 {
			return false
		}
	}
	return true
}

func compileTestSetupsCached(index typesys.Index, digests *typesys.SourceDigestSet, baseKey runtimeCacheKey, selectedClasses map[string]bool, sources *sourceCache) (map[string][]vm.Method, map[string]error, map[string][]ir.Program, map[string]error, error) {
	key := string(baseKey) + "|setup|" + classSetKey(selectedClasses)
	setupCacheMu.RLock()
	if cached, ok := setupCache[key]; ok {
		setupCacheMu.RUnlock()
		invokeSnapshotValidationHook("setup_before_cache_return")
		if err := validateRuntimeSourceDigests(index, digests); err != nil {
			evictSnapshotCaches(baseKey)
			return nil, nil, nil, nil, err
		}
		if err := sources.validateCapturedSourceGeneration(); err != nil {
			evictSnapshotCaches(baseKey)
			return nil, nil, nil, nil, err
		}
		return cached.Methods, cached.Errors, cached.Programs, cached.ProgramErrs, nil
	}
	setupCacheMu.RUnlock()
	methods, errs, programs, programErrs := compileTestSetupMethodsForClasses(index, selectedClasses, sources)
	if err := sources.sourceSnapshotError(); err != nil {
		return methods, errs, programs, programErrs, err
	}
	invokeSnapshotValidationHook("setup_before_cache_publication")
	if err := validateRuntimeSourceDigests(index, digests); err != nil {
		evictSnapshotCaches(baseKey)
		return methods, errs, programs, programErrs, err
	}
	if err := sources.validateCapturedSourceGeneration(); err != nil {
		evictSnapshotCaches(baseKey)
		return methods, errs, programs, programErrs, err
	}
	setupCacheMu.Lock()
	setupCache[key] = setupCompileCacheEntry{Methods: methods, Errors: errs, Programs: programs, ProgramErrs: programErrs}
	setupCacheMu.Unlock()
	return methods, errs, programs, programErrs, nil
}

func compileTestsCached(index typesys.Index, digests *typesys.SourceDigestSet, baseKey runtimeCacheKey, cases []TestCase, sources *sourceCache) (map[string]vm.Method, map[string]error, map[string]ir.Program, map[string]error, error) {
	key := string(baseKey) + "|tests|" + caseSetKey(cases)
	testCacheMu.RLock()
	if cached, ok := testCache[key]; ok {
		testCacheMu.RUnlock()
		invokeSnapshotValidationHook("test_before_cache_return")
		if err := validateRuntimeSourceDigests(index, digests); err != nil {
			evictSnapshotCaches(baseKey)
			return nil, nil, nil, nil, err
		}
		if err := sources.validateCapturedSourceGeneration(); err != nil {
			evictSnapshotCaches(baseKey)
			return nil, nil, nil, nil, err
		}
		return cached.Methods, cached.MethodErrs, cached.Programs, cached.ProgramErrs, nil
	}
	testCacheMu.RUnlock()
	methods, methodErrs := compileTestMethods(cases, sources)
	programs, programErrs := compileTestInvokePrograms(cases)
	if err := sources.sourceSnapshotError(); err != nil {
		return methods, methodErrs, programs, programErrs, err
	}
	invokeSnapshotValidationHook("test_before_cache_publication")
	if err := validateRuntimeSourceDigests(index, digests); err != nil {
		evictSnapshotCaches(baseKey)
		return methods, methodErrs, programs, programErrs, err
	}
	if err := sources.validateCapturedSourceGeneration(); err != nil {
		evictSnapshotCaches(baseKey)
		return methods, methodErrs, programs, programErrs, err
	}
	testCacheMu.Lock()
	testCache[key] = testCompileCacheEntry{Methods: methods, MethodErrs: methodErrs, Programs: programs, ProgramErrs: programErrs}
	testCacheMu.Unlock()
	return methods, methodErrs, programs, programErrs, nil
}

func prepareTestSetups(ctx context.Context, classNames []string, baseMachine *vm.VM, baseRuntimeErr error, setups map[string][]vm.Method, setupErrors map[string]error, setupInvokePrograms map[string][]ir.Program, setupInvokeErrors map[string]error, triggerErrors []error, org storage.OrgState, opts Options, counters *runPerfCounters) map[string]testSetupResult {
	results := make(map[string]testSetupResult, len(classNames))
	emitProgress := opts.Progress != nil
	parallelism := opts.Parallelism
	if parallelism <= 1 || len(classNames) <= 1 {
		for _, className := range classNames {
			if emitProgress {
				reportProgress(opts, TestProgress{Event: "setup_start", ClassName: className})
			}
			var started time.Time
			if emitProgress {
				started = time.Now()
			}
			setupCtx, setupCancel := testContext(ctx, opts.TimeoutMS)
			setupOrg, setupRandom, setupErr, shared := prepareTestSetupOrg(setupCtx, className, baseMachine, baseRuntimeErr, setups[className], setupErrors[className], setupInvokePrograms[className], setupInvokeErrors[className], triggerErrors, org, opts, counters)
			if setupCancel != nil {
				setupCancel()
			}
			if emitProgress {
				reportProgress(opts, TestProgress{Event: "setup_done", ClassName: className, DurationMS: time.Since(started).Milliseconds(), Status: progressStatus(setupErr)})
			}
			results[className] = testSetupResult{Org: setupOrg, Err: setupErr, Random: setupRandom, OrgIsShared: shared}
		}
		return results
	}
	if parallelism > len(classNames) {
		parallelism = len(classNames)
	}
	type setupJobResult struct {
		ClassName string
		Result    testSetupResult
	}
	jobs := make(chan string)
	out := make(chan setupJobResult, len(classNames))
	var wg sync.WaitGroup
	for worker := 0; worker < parallelism; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for className := range jobs {
				if emitProgress {
					reportProgress(opts, TestProgress{Event: "setup_start", ClassName: className})
				}
				var started time.Time
				if emitProgress {
					started = time.Now()
				}
				setupCtx, setupCancel := testContext(ctx, opts.TimeoutMS)
				setupOrg, setupRandom, setupErr, shared := prepareTestSetupOrg(setupCtx, className, baseMachine, baseRuntimeErr, setups[className], setupErrors[className], setupInvokePrograms[className], setupInvokeErrors[className], triggerErrors, org, opts, counters)
				if setupCancel != nil {
					setupCancel()
				}
				if emitProgress {
					reportProgress(opts, TestProgress{Event: "setup_done", ClassName: className, DurationMS: time.Since(started).Milliseconds(), Status: progressStatus(setupErr)})
				}
				out <- setupJobResult{ClassName: className, Result: testSetupResult{Org: setupOrg, Err: setupErr, Random: setupRandom, OrgIsShared: shared}}
			}
		}()
	}
	for _, className := range classNames {
		jobs <- className
	}
	close(jobs)
	wg.Wait()
	close(out)
	for item := range out {
		results[item.ClassName] = item.Result
	}
	return results
}

func runTestPlans(ctx context.Context, planned []testCasePlan, results []testreport.Case, baseMachine *vm.VM, baseRuntimeErr error, triggerErrors []error, opts Options, counters *runPerfCounters) {
	parallelism := opts.Parallelism
	emitProgress := opts.Progress != nil
	classOrder := make([]string, 0)
	classIndexes := make(map[string][]int)
	for i, plan := range planned {
		className := plan.TestCase.ClassName
		if _, ok := classIndexes[className]; !ok {
			classOrder = append(classOrder, className)
		}
		classIndexes[className] = append(classIndexes[className], i)
	}
	if parallelism <= 1 || len(planned) <= 1 {
		var methodWindowStarted time.Time
		for i, plan := range planned {
			if results[i].Status != "" {
				continue
			}
			if emitProgress {
				reportProgress(opts, TestProgress{Event: "test_start", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName})
			}
			caseCtx, caseCancel := testContext(ctx, opts.TimeoutMS)
			cloneOrg := len(planned) > 1 || plan.SetupShared
			if methodWindowStarted.IsZero() {
				methodWindowStarted = startMethodWindow(counters)
			}
			results[i] = runCase(caseCtx, plan.TestCase, plan.TestMethodErr, plan.InvokeProgram, plan.InvokeProgErr, baseMachine, baseRuntimeErr, plan.SetupErr, triggerErrors, plan.SetupOrg, plan.SetupRandom, opts, cloneOrg, nil, counters)
			if caseCancel != nil {
				caseCancel()
			}
			if emitProgress {
				reportProgress(opts, TestProgress{Event: "test_done", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName, DurationMS: results[i].DurationMS, Status: string(results[i].Status)})
			}
		}
		recordMethodWindow(methodWindowStarted, counters)
		return
	}
	if opts.ParallelMethods && len(classOrder) == 1 && len(planned) > 1 {
		runSingleClassTestPlans(ctx, planned, results, baseMachine, baseRuntimeErr, triggerErrors, opts, counters, parallelism)
		return
	}
	if parallelism > len(classOrder) {
		parallelism = len(classOrder)
	}
	sortClassRunOrder(classOrder, classIndexes, opts.ClassDurationMS, counters)
	jobs := make(chan string)
	methodWindow := methodWindowTimer{counters: counters}
	var wg sync.WaitGroup
	for worker := 0; worker < parallelism; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for className := range jobs {
				methodIndexes := append([]int(nil), classIndexes[className]...)
				sortMethodIndexes(methodIndexes, planned, opts.MethodDurationMS, counters)
				for _, i := range methodIndexes {
					if results[i].Status != "" {
						continue
					}
					plan := planned[i]
					methodWindow.dispatch()
					if emitProgress {
						reportProgress(opts, TestProgress{Event: "test_start", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName})
					}
					caseCtx, caseCancel := testContext(ctx, opts.TimeoutMS)
					cloneOrg := len(planned) > 1 || plan.SetupShared
					results[i] = runCase(caseCtx, plan.TestCase, plan.TestMethodErr, plan.InvokeProgram, plan.InvokeProgErr, baseMachine, baseRuntimeErr, plan.SetupErr, triggerErrors, plan.SetupOrg, plan.SetupRandom, opts, cloneOrg, nil, counters)
					if caseCancel != nil {
						caseCancel()
					}
					if emitProgress {
						reportProgress(opts, TestProgress{Event: "test_done", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName, DurationMS: results[i].DurationMS, Status: string(results[i].Status)})
					}
				}
			}
		}()
	}
	for _, className := range classOrder {
		jobs <- className
	}
	close(jobs)
	wg.Wait()
	methodWindow.join()
}

func runNoSetupJournalPlans(ctx context.Context, planned []testCasePlan, results []testreport.Case, baseMachine *vm.VM, baseRuntimeErr error, triggerErrors []error, org storage.OrgState, opts Options, counters *runPerfCounters) {
	parallelism := opts.Parallelism
	emitProgress := opts.Progress != nil
	classOrder := make([]string, 0)
	classIndexes := make(map[string][]int)
	for i, plan := range planned {
		if results[i].Status != "" {
			continue
		}
		className := plan.TestCase.ClassName
		if _, ok := classIndexes[className]; !ok {
			classOrder = append(classOrder, className)
		}
		classIndexes[className] = append(classIndexes[className], i)
	}
	if len(classOrder) == 0 {
		return
	}
	if parallelism <= 1 || len(classOrder) <= 1 {
		journal := storage.NewIsolationJournal(&org)
		methodWindow := methodWindowTimer{counters: counters}
		for _, className := range classOrder {
			methodIndexes := append([]int(nil), classIndexes[className]...)
			sortMethodIndexes(methodIndexes, planned, opts.MethodDurationMS, counters)
			for _, i := range methodIndexes {
				methodWindow.dispatch()
				runJournaledNoSetupPlan(ctx, i, planned, results, journal, 0, baseMachine, baseRuntimeErr, triggerErrors, opts, counters, emitProgress)
			}
		}
		methodWindow.join()
		return
	}
	if parallelism > len(classOrder) {
		parallelism = len(classOrder)
	}
	sortClassRunOrder(classOrder, classIndexes, opts.ClassDurationMS, counters)
	jobs := make(chan string)
	methodWindow := methodWindowTimer{counters: counters}
	var wg sync.WaitGroup
	for worker := 0; worker < parallelism; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			workerOrg := cloneRuntimeOrgForClass(org, "", "test-worker", counters)
			journal := storage.NewIsolationJournal(&workerOrg)
			for className := range jobs {
				methodIndexes := append([]int(nil), classIndexes[className]...)
				sortMethodIndexes(methodIndexes, planned, opts.MethodDurationMS, counters)
				for _, i := range methodIndexes {
					methodWindow.dispatch()
					runJournaledNoSetupPlan(ctx, i, planned, results, journal, 0, baseMachine, baseRuntimeErr, triggerErrors, opts, counters, emitProgress)
				}
			}
		}()
	}
	for _, className := range classOrder {
		jobs <- className
	}
	close(jobs)
	wg.Wait()
	methodWindow.join()
}

type methodWindowTimer struct {
	counters *runPerfCounters
	once     sync.Once
	started  time.Time
}

func (t *methodWindowTimer) dispatch() {
	if t == nil || t.counters == nil || !t.counters.enabled {
		return
	}
	t.once.Do(func() { t.started = time.Now() })
}

func (t *methodWindowTimer) join() {
	if t == nil || t.started.IsZero() {
		return
	}
	recordMethodWindow(t.started, t.counters)
}

func runJournaledNoSetupPlan(ctx context.Context, i int, planned []testCasePlan, results []testreport.Case, journal *storage.IsolationJournal, setupRandom uint64, baseMachine *vm.VM, baseRuntimeErr error, triggerErrors []error, opts Options, counters *runPerfCounters, emitProgress bool) {
	if i < 0 || i >= len(planned) || results[i].Status != "" {
		return
	}
	plan := planned[i]
	if emitProgress {
		reportProgress(opts, TestProgress{Event: "test_start", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName})
	}
	caseCtx, caseCancel := testContext(ctx, opts.TimeoutMS)
	mark := journal.Mark()
	results[i] = runCase(caseCtx, plan.TestCase, plan.TestMethodErr, plan.InvokeProgram, plan.InvokeProgErr, baseMachine, baseRuntimeErr, plan.SetupErr, triggerErrors, *journal.Org(), setupRandom, opts, false, journal, counters)
	if rollbackErr := rollbackJournal(journal, mark, counters); rollbackErr != nil && results[i].Problem == nil {
		results[i].Status = testreport.StatusFail
		results[i].Problem = problem("InternalError", rollbackErr.Error(), plan.TestCase)
	}
	recordJournalRollback(counters)
	if caseCancel != nil {
		caseCancel()
	}
	if emitProgress {
		reportProgress(opts, TestProgress{Event: "test_done", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName, DurationMS: results[i].DurationMS, Status: string(results[i].Status)})
	}
}

func rollbackJournal(journal *storage.IsolationJournal, mark storage.IsolationMark, counters *runPerfCounters) error {
	if counters == nil || !counters.enabled {
		return journal.Rollback(mark)
	}
	started := time.Now()
	err := journal.Rollback(mark)
	counters.phases.rollbackNS.Add(time.Since(started).Nanoseconds())
	return err
}

func runTestPlansWithSetups(ctx context.Context, classOrder []string, classIndexes map[string][]int, planned []testCasePlan, results []testreport.Case, baseMachine *vm.VM, baseRuntimeErr error, setups map[string][]vm.Method, setupErrors map[string]error, setupInvokePrograms map[string][]ir.Program, setupInvokeErrors map[string]error, triggerErrors []error, org storage.OrgState, opts Options, counters *runPerfCounters) {
	parallelism := opts.Parallelism
	if parallelism <= 1 {
		parallelism = 1
	}
	emitProgress := opts.Progress != nil
	if parallelism > len(classOrder) {
		parallelism = len(classOrder)
	}
	if parallelism > 1 {
		sortClassRunOrder(classOrder, classIndexes, opts.ClassDurationMS, counters)
	}
	adaptiveBudgets := map[string]int{}
	if opts.ParallelMethods {
		var adaptiveHistoryStarted time.Time
		if counters != nil && counters.enabled && len(opts.ClassDurationMS) > 0 {
			adaptiveHistoryStarted = time.Now()
		}
		adaptiveBudgets = adaptiveClassMethodBudget(opts.Parallelism, classScheduleInputs(classOrder, classIndexes, opts.ClassDurationMS))
		if counters != nil && counters.enabled && len(opts.ClassDurationMS) > 0 {
			counters.phases.historyApplyNS.Add(time.Since(adaptiveHistoryStarted).Nanoseconds())
		}
		reservedMethodWorkers := 0
		for _, budget := range adaptiveBudgets {
			if budget > 1 && budget-1 > reservedMethodWorkers {
				reservedMethodWorkers = budget - 1
			}
		}
		if reservedMethodWorkers > 0 && parallelism > 1 {
			parallelism -= reservedMethodWorkers
			if parallelism < 1 {
				parallelism = 1
			}
		}
	}
	costHints := aggregateClassCostHints(classOrder, planned)
	var historyStarted time.Time
	if counters != nil && counters.enabled && len(opts.ClassDurationMS) > 0 {
		historyStarted = time.Now()
	}
	dispatcher := newClassDispatcher(classOrder, costHints, opts.ClassDurationMS)
	if counters != nil && counters.enabled && len(opts.ClassDurationMS) > 0 {
		counters.phases.historyApplyNS.Add(time.Since(historyStarted).Nanoseconds())
	}
	defer func() {
		if counters == nil || !counters.enabled {
			dispatcher.close()
			return
		}
		teardownStarted := time.Now()
		dispatcher.close()
		counters.phases.teardownNS.Add(time.Since(teardownStarted).Nanoseconds())
	}()
	methodWindow := methodWindowTimer{counters: counters}
	var wg sync.WaitGroup
	for worker := 0; worker < parallelism; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				className, ok := dispatcher.next()
				if !ok {
					return
				}
				classStart := time.Now()
				if emitProgress {
					reportProgress(opts, TestProgress{Event: "setup_start", ClassName: className})
				}
				var started time.Time
				if emitProgress {
					started = time.Now()
				}
				setupCtx, setupCancel := testContext(ctx, opts.TimeoutMS)
				setupOrg, setupRandom, setupErr, setupShared := prepareTestSetupOrg(setupCtx, className, baseMachine, baseRuntimeErr, setups[className], setupErrors[className], setupInvokePrograms[className], setupInvokeErrors[className], triggerErrors, org, opts, counters)
				if setupCancel != nil {
					setupCancel()
				}
				if emitProgress {
					reportProgress(opts, TestProgress{Event: "setup_done", ClassName: className, DurationMS: time.Since(started).Milliseconds(), Status: progressStatus(setupErr)})
				}
				methodIndexes := append([]int(nil), classIndexes[className]...)
				sortMethodIndexes(methodIndexes, planned, opts.MethodDurationMS, counters)
				if opts.ParallelMethods && len(methodIndexes) > 1 {
					methodParallelism := methodParallelismForClassRun(opts.Parallelism, parallelism, len(methodIndexes), dispatcher.unfinishedClassCount())
					if extra := adaptiveBudgets[className]; extra > methodParallelism {
						methodParallelism = extra
					}
					runClassMethodIndexes(ctx, methodIndexes, planned, results, setupOrg, setupErr, setupRandom, setupShared, baseMachine, baseRuntimeErr, triggerErrors, opts, counters, methodParallelism, &methodWindow)
					dispatcher.recordObserved(className, time.Since(classStart).Milliseconds())
					continue
				}
				var journal *storage.IsolationJournal
				if len(methodIndexes) > 1 && classSupportsJournalIsolation(methodIndexes, planned) {
					if setupShared {
						setupOrg = cloneRuntimeOrgForClass(setupOrg, className, "setup", counters)
						setupShared = false
					}
					journal = storage.NewIsolationJournal(&setupOrg)
				}
				for _, i := range methodIndexes {
					if results[i].Status != "" {
						continue
					}
					plan := planned[i]
					plan.SetupErr = setupErr
					plan.SetupOrg = setupOrg
					plan.SetupRandom = setupRandom
					plan.SetupShared = setupShared
					if emitProgress {
						reportProgress(opts, TestProgress{Event: "test_start", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName})
					}
					methodWindow.dispatch()
					caseCtx, caseCancel := testContext(ctx, opts.TimeoutMS)
					cloneOrg := len(methodIndexes) > 1 || plan.SetupShared
					var mark storage.IsolationMark
					caseJournal := journal
					if caseJournal != nil {
						mark = caseJournal.Mark()
						cloneOrg = false
					}
					if cloneOrg {
						recordCloneFallback(counters)
					}
					results[i] = runCase(caseCtx, plan.TestCase, plan.TestMethodErr, plan.InvokeProgram, plan.InvokeProgErr, baseMachine, baseRuntimeErr, plan.SetupErr, triggerErrors, plan.SetupOrg, plan.SetupRandom, opts, cloneOrg, caseJournal, counters)
					if caseJournal != nil {
						if rollbackErr := rollbackJournal(caseJournal, mark, counters); rollbackErr != nil && results[i].Problem == nil {
							results[i].Status = testreport.StatusFail
							results[i].Problem = problem("InternalError", rollbackErr.Error(), plan.TestCase)
						}
						recordJournalRollback(counters)
					}
					if caseCancel != nil {
						caseCancel()
					}
					if emitProgress {
						reportProgress(opts, TestProgress{Event: "test_done", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName, DurationMS: results[i].DurationMS, Status: string(results[i].Status)})
					}
				}
				dispatcher.recordObserved(className, time.Since(classStart).Milliseconds())
			}
		}()
	}
	wg.Wait()
	methodWindow.join()
}

var apexMergeDMLPattern = regexp.MustCompile(`(?i)\bmerge\s+(?:new\s+)?[A-Za-z_][A-Za-z0-9_]*`)
var apexMetadataMutationPattern = regexp.MustCompile(`(?i)\bMetadata\s*\.`)
var apexTestSetMockPattern = regexp.MustCompile(`(?i)\b(?:System\s*\.\s*)?Test\s*\.\s*setMock\s*\(`)

type classIsolationProbeCache struct {
	mu       sync.Mutex
	readFile func(string) ([]byte, error)
	files    map[string]bool
}

func newClassIsolationProbeCache(readFile func(string) ([]byte, error)) *classIsolationProbeCache {
	if readFile == nil {
		readFile = os.ReadFile
	}
	return &classIsolationProbeCache{
		readFile: readFile,
		files:    make(map[string]bool),
	}
}

func classSupportsJournalIsolation(indexes []int, planned []testCasePlan) bool {
	return newClassIsolationProbeCache(os.ReadFile).supportsJournalIsolation(indexes, planned)
}

func allPlansSupportJournalIsolation(planned []testCasePlan) bool {
	if len(planned) == 0 {
		return true
	}
	indexes := make([]int, len(planned))
	for i := range planned {
		indexes[i] = i
	}
	return newClassIsolationProbeCache(os.ReadFile).supportsJournalIsolation(indexes, planned)
}

func (c *classIsolationProbeCache) supportsJournalIsolation(indexes []int, planned []testCasePlan) bool {
	for _, i := range indexes {
		if i < 0 || i >= len(planned) {
			continue
		}
		file := strings.TrimSpace(planned[i].TestCase.File)
		if file == "" {
			continue
		}
		if !c.fileSupportsJournalIsolation(file) {
			return false
		}
	}
	return true
}

func (c *classIsolationProbeCache) fileSupportsJournalIsolation(file string) bool {
	if c == nil {
		c = newClassIsolationProbeCache(os.ReadFile)
	}
	c.mu.Lock()
	if supported, ok := c.files[file]; ok {
		c.mu.Unlock()
		return supported
	}
	c.mu.Unlock()

	source, err := c.readFile(file)
	supported := err != nil || !apexMergeDMLPattern.Match(source) &&
		!apexMetadataMutationPattern.Match(source) &&
		!apexTestSetMockPattern.Match(source)

	c.mu.Lock()
	c.files[file] = supported
	c.mu.Unlock()
	return supported
}

// aggregateClassCostHints sums the per-test CostHint values for every class
// in classOrder. The signal is intentionally coarse — see testCaseCostHint.
func aggregateClassCostHints(classOrder []string, planned []testCasePlan) map[string]int64 {
	if len(classOrder) == 0 || len(planned) == 0 {
		return nil
	}
	out := make(map[string]int64, len(classOrder))
	for _, plan := range planned {
		if plan.TestCase.ClassName == "" {
			continue
		}
		out[plan.TestCase.ClassName] += plan.TestCase.CostHint
	}
	return out
}

func methodParallelismForClassRun(totalParallelism, classParallelism, methods, unfinishedClasses int) int {
	if methods <= 1 {
		return 1
	}
	if totalParallelism <= 1 {
		return 1
	}
	if classParallelism <= 1 {
		classParallelism = 1
	}
	parallelism := totalParallelism / classParallelism
	if parallelism < 1 {
		parallelism = 1
	}
	// When most class workers are idle at the tail, let the remaining class(es)
	// use the freed CPU budget for method-level parallelism.
	if unfinishedClasses > 0 && unfinishedClasses < classParallelism {
		if boosted := totalParallelism / unfinishedClasses; boosted > parallelism {
			parallelism = boosted
		}
	}
	if parallelism > methods {
		parallelism = methods
	}
	return parallelism
}

func sortClassRunOrder(classOrder []string, classIndexes map[string][]int, classDurationMS map[string]int64, counters ...*runPerfCounters) {
	c := perfCounterFor(counters)
	var started time.Time
	if c.enabled && len(classDurationMS) > 0 {
		started = time.Now()
		defer func() { c.phases.historyApplyNS.Add(time.Since(started).Nanoseconds()) }()
	}
	sort.SliceStable(classOrder, func(i, j int) bool {
		left := classOrder[i]
		right := classOrder[j]
		leftDuration := classDurationMS[left]
		rightDuration := classDurationMS[right]
		if leftDuration > 0 || rightDuration > 0 {
			if leftDuration == rightDuration {
				return len(classIndexes[left]) > len(classIndexes[right])
			}
			return leftDuration > rightDuration
		}
		return len(classIndexes[left]) > len(classIndexes[right])
	})
}

func sortMethodIndexes(indexes []int, planned []testCasePlan, methodDurationMS map[string]int64, counters ...*runPerfCounters) {
	if len(indexes) <= 1 || len(methodDurationMS) == 0 {
		return
	}
	c := perfCounterFor(counters)
	var started time.Time
	if c.enabled {
		started = time.Now()
		defer func() { c.phases.historyApplyNS.Add(time.Since(started).Nanoseconds()) }()
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		left := planned[indexes[i]].TestCase
		right := planned[indexes[j]].TestCase
		leftMS := methodDurationMS[left.ClassName+"."+left.MethodName]
		rightMS := methodDurationMS[right.ClassName+"."+right.MethodName]
		if leftMS == rightMS {
			if left.ClassName == right.ClassName {
				return left.MethodName < right.MethodName
			}
			return left.ClassName < right.ClassName
		}
		return leftMS > rightMS
	})
}

func runClassMethodIndexes(ctx context.Context, indexes []int, planned []testCasePlan, results []testreport.Case, setupOrg storage.OrgState, setupErr error, setupRandom uint64, setupShared bool, baseMachine *vm.VM, baseRuntimeErr error, triggerErrors []error, opts Options, counters *runPerfCounters, parallelism int, methodWindow *methodWindowTimer) {
	emitProgress := opts.Progress != nil
	if parallelism <= 1 {
		parallelism = 1
	}
	if parallelism > len(indexes) {
		parallelism = len(indexes)
	}
	useJournal := len(indexes) > 1 && classSupportsJournalIsolation(indexes, planned)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < parallelism; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			workerOrg := setupOrg
			workerSetupShared := setupShared
			var journal *storage.IsolationJournal
			if useJournal {
				className := ""
				if len(indexes) > 0 {
					className = planned[indexes[0]].TestCase.ClassName
				}
				workerOrg = cloneRuntimeOrgForClass(setupOrg, className, "test-worker", counters)
				workerSetupShared = false
				journal = storage.NewIsolationJournal(&workerOrg)
			}
			for i := range jobs {
				if results[i].Status != "" {
					continue
				}
				plan := planned[i]
				methodWindow.dispatch()
				plan.SetupErr = setupErr
				plan.SetupOrg = workerOrg
				plan.SetupRandom = setupRandom
				plan.SetupShared = workerSetupShared
				if emitProgress {
					reportProgress(opts, TestProgress{Event: "test_start", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName})
				}
				caseCtx, caseCancel := testContext(ctx, opts.TimeoutMS)
				cloneOrg := true
				caseJournal := journal
				var mark storage.IsolationMark
				if caseJournal != nil {
					mark = caseJournal.Mark()
					cloneOrg = false
				}
				results[i] = runCase(caseCtx, plan.TestCase, plan.TestMethodErr, plan.InvokeProgram, plan.InvokeProgErr, baseMachine, baseRuntimeErr, plan.SetupErr, triggerErrors, plan.SetupOrg, plan.SetupRandom, opts, cloneOrg, caseJournal, counters)
				if caseJournal != nil {
					if rollbackErr := rollbackJournal(caseJournal, mark, counters); rollbackErr != nil && results[i].Problem == nil {
						results[i].Status = testreport.StatusFail
						results[i].Problem = problem("InternalError", rollbackErr.Error(), plan.TestCase)
					}
					recordJournalRollback(counters)
				}
				if caseCancel != nil {
					caseCancel()
				}
				if emitProgress {
					reportProgress(opts, TestProgress{Event: "test_done", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName, DurationMS: results[i].DurationMS, Status: string(results[i].Status)})
				}
			}
		}()
	}
	for _, i := range indexes {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

func runSingleClassTestPlans(ctx context.Context, planned []testCasePlan, results []testreport.Case, baseMachine *vm.VM, baseRuntimeErr error, triggerErrors []error, opts Options, counters *runPerfCounters, parallelism int) {
	emitProgress := opts.Progress != nil
	if parallelism <= 1 {
		parallelism = 1
	}
	if parallelism > len(planned) {
		parallelism = len(planned)
	}
	jobs := make(chan int)
	methodWindow := methodWindowTimer{counters: counters}
	var wg sync.WaitGroup
	for worker := 0; worker < parallelism; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if results[i].Status != "" {
					continue
				}
				plan := planned[i]
				methodWindow.dispatch()
				if emitProgress {
					reportProgress(opts, TestProgress{Event: "test_start", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName})
				}
				caseCtx, caseCancel := testContext(ctx, opts.TimeoutMS)
				results[i] = runCase(caseCtx, plan.TestCase, plan.TestMethodErr, plan.InvokeProgram, plan.InvokeProgErr, baseMachine, baseRuntimeErr, plan.SetupErr, triggerErrors, plan.SetupOrg, plan.SetupRandom, opts, len(planned) > 1, nil, counters)
				if caseCancel != nil {
					caseCancel()
				}
				if emitProgress {
					reportProgress(opts, TestProgress{Event: "test_done", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName, DurationMS: results[i].DurationMS, Status: string(results[i].Status)})
				}
			}
		}()
	}
	indexes := make([]int, 0, len(planned))
	for i := range planned {
		indexes = append(indexes, i)
	}
	sortMethodIndexes(indexes, planned, opts.MethodDurationMS, counters)
	for _, i := range indexes {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	methodWindow.join()
}

func reportProgress(opts Options, progress TestProgress) {
	if opts.Progress != nil {
		opts.Progress(progress)
	}
}

func progressStatus(err error) string {
	if err != nil {
		return "error"
	}
	return "pass"
}

func testContext(parent context.Context, timeoutMS int64) (context.Context, context.CancelFunc) {
	if timeoutMS <= 0 {
		return parent, nil
	}
	return context.WithTimeout(parent, time.Duration(timeoutMS)*time.Millisecond)
}

func initializeTestOrg(org *storage.OrgState) {
	machine := vm.New(nil)
	machine.SetTraceEnabled(false)
	machine.SetOrg(org)
	machine.EnableTestContext()
}

func prepareTestSetupOrg(ctx context.Context, className string, baseMachine *vm.VM, baseRuntimeErr error, setups []vm.Method, setupErr error, setupPrograms []ir.Program, setupProgramErr error, triggerErrors []error, org storage.OrgState, opts Options, counters *runPerfCounters) (storage.OrgState, uint64, error, bool) {
	started := time.Now()
	defer func() { recordSetupDuration(time.Since(started), counters) }()
	if err := ctx.Err(); err != nil {
		return org, 0, err, true
	}
	if baseRuntimeErr != nil {
		return org, 0, baseRuntimeErr, true
	}
	if setupErr != nil {
		return org, 0, setupErr, true
	}
	if setupProgramErr != nil {
		return org, 0, setupProgramErr, true
	}
	if len(triggerErrors) > 0 {
		return org, 0, triggerErrors[0], true
	}
	if len(setups) == 0 {
		return org, 0, nil, true
	}
	setupOrg := cloneRuntimeOrgForClass(org, className, "setup", counters)
	machine := cloneRuntimeMachineFor(baseMachine, className, "setup", counters)
	machine.SetTraceEnabled(false)
	if opts.LimitMode != "" {
		machine.SetLimitMode(opts.LimitMode)
	}
	if opts.LimitCapsSet {
		machine.SetLimitCaps(opts.LimitCaps)
	}
	machine.SetOrg(&setupOrg)
	machine.SetContext(ctx)
	machine.EnableTestContext()
	machine.SetCurrentPageURLNull()
	for i, setup := range setups {
		if err := ctx.Err(); err != nil {
			return setupOrg, machine.DeterministicRandomState(), err, false
		}
		if i >= len(setupPrograms) {
			return setupOrg, machine.DeterministicRandomState(), fmt.Errorf("missing compiled @TestSetup invocation for %s", setup.Name), false
		}
		if _, err := machine.ExecuteInClass(setupPrograms[i], setup.ClassName); err != nil {
			return setupOrg, machine.DeterministicRandomState(), err, false
		}
	}
	return setupOrg, machine.DeterministicRandomState(), nil, false
}

func runCase(ctx context.Context, testCase TestCase, testMethodErr error, invokeProgram ir.Program, invokeErr error, baseMachine *vm.VM, baseRuntimeErr error, setupErr error, triggerErrors []error, org storage.OrgState, setupRandom uint64, opts Options, cloneOrg bool, journal *storage.IsolationJournal, counters *runPerfCounters) testreport.Case {
	if err := ctx.Err(); err != nil {
		return canceledCase(testCase, err)
	}
	out := testreport.Case{
		ClassName:  testCase.ClassName,
		MethodName: testCase.MethodName,
		Status:     testreport.StatusPass,
	}
	started := time.Now()
	defer func() {
		elapsed := time.Since(started)
		recordRunDuration(elapsed, counters)
		out.DurationMS = elapsed.Milliseconds()
	}()
	if setupErr != nil {
		if errors.Is(setupErr, context.Canceled) || errors.Is(setupErr, context.DeadlineExceeded) {
			out.Status = testreport.StatusUnsupported
			out.Problem = problem("Canceled", setupErr.Error(), testCase)
			return out
		}
		out.Status = testreport.StatusUnsupported
		out.Problem = problem("UnsupportedFeature", setupErr.Error(), testCase)
		return out
	}
	if baseRuntimeErr != nil {
		out.Status = testreport.StatusUnsupported
		out.Problem = problem("UnsupportedFeature", baseRuntimeErr.Error(), testCase)
		return out
	}
	if len(triggerErrors) > 0 {
		out.Status = testreport.StatusUnsupported
		out.Problem = problem("UnsupportedFeature", triggerErrors[0].Error(), testCase)
		return out
	}
	if testMethodErr != nil {
		out.Status = testreport.StatusUnsupported
		out.Problem = problem("UnsupportedFeature", testMethodErr.Error(), testCase)
		return out
	}
	if invokeErr != nil {
		out.Status = testreport.StatusUnsupported
		out.Problem = problem("UnsupportedFeature", invokeErr.Error(), testCase)
		return out
	}
	machine := cloneRuntimeMachineFor(baseMachine, testCase.ClassName, "test", counters)
	machine.SetDeterministicRandomState(setupRandom)
	machine.SetTraceEnabled(opts.TraceAll || opts.TraceBlocked || opts.SlowTestThresholdMS > 0)
	if opts.LimitMode != "" {
		machine.SetLimitMode(opts.LimitMode)
	}
	if opts.LimitCapsSet {
		machine.SetLimitCaps(opts.LimitCaps)
	}
	machine.SetContext(ctx)
	if cloneOrg {
		org = cloneRuntimeOrgForClass(org, testCase.ClassName, "test", counters)
	}
	if journal != nil && !cloneOrg {
		machine.SetOrg(journal.Org())
	} else {
		machine.SetOrg(&org)
	}
	machine.SetIsolationJournal(journal)
	machine.EnableTestContext()
	machine.SetCurrentPageURLNull()
	machine.SetTestSeeAllData(testCase.SeeAllData)
	machine.ResetLimits()
	result, err := machine.ExecuteInClass(invokeProgram, testCase.ClassName)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			out.Status = testreport.StatusUnsupported
		} else {
			var runtimeErr *vm.RuntimeError
			if errors.As(err, &runtimeErr) && runtimeErr.Type == "UnsupportedFeature" {
				out.Status = testreport.StatusUnsupported
			} else {
				out.Status = testreport.StatusFail
			}
		}
		out.Problem = problemFromError(err, testCase)
	}
	out.DurationMS = time.Since(started).Milliseconds()
	attachTraceProfile(&out, result, opts)
	return out
}

func attachTraceProfile(out *testreport.Case, result vm.Result, opts Options) {
	if len(result.Trace) == 0 {
		return
	}
	blocked := out.Status != testreport.StatusPass
	slow := opts.SlowTestThresholdMS > 0 && out.DurationMS >= opts.SlowTestThresholdMS
	if opts.TraceAll {
		out.Trace = append([]trace.Event(nil), result.Trace...)
		report := profile.Analyze(trace.NewDocument(out.Trace))
		out.Profile = &report
		return
	}
	if !blocked && !slow {
		return
	}
	if !opts.TraceBlocked && !slow {
		return
	}
	out.Trace = append([]trace.Event(nil), result.Trace...)
	report := profile.Analyze(trace.NewDocument(out.Trace))
	out.Profile = &report
}

func canceledCase(testCase TestCase, err error) testreport.Case {
	return testreport.Case{
		ClassName:  testCase.ClassName,
		MethodName: testCase.MethodName,
		Status:     testreport.StatusUnsupported,
		Problem:    problem("Canceled", err.Error(), testCase),
	}
}

func registerBaseRuntime(machine *vm.VM, methods map[string]vm.Method, classes []vm.Class, triggers []vm.Trigger) error {
	for _, class := range classes {
		if err := machine.RegisterClass(class); err != nil {
			return err
		}
	}
	for _, trigger := range triggers {
		if err := machine.RegisterTrigger(trigger); err != nil {
			return err
		}
	}
	for _, method := range methods {
		if err := machine.RegisterMethod(method); err != nil {
			return err
		}
	}
	return nil
}

func registerTestRuntime(machine *vm.VM, methods []vm.Method) error {
	for _, method := range methods {
		if err := machine.RegisterMethod(method); err != nil {
			return err
		}
	}
	return nil
}

func registerRuntime(machine *vm.VM, methods map[string]vm.Method, classes []vm.Class, setups []vm.Method, triggers []vm.Trigger) error {
	if err := registerBaseRuntime(machine, methods, classes, triggers); err != nil {
		return err
	}
	return registerTestRuntime(machine, setups)
}

func cloneRuntimeOrg(org storage.OrgState, counters ...*runPerfCounters) storage.OrgState {
	recordCloneRuntimeOrg("", "", counters...)
	recordStorageCloneRollbackSnapshot(counters...)
	recordCloneReason("", "rollback-snapshot", "org-rollback-snapshot", counters...)
	started := time.Now()
	clone := org.CloneRollbackSnapshot()
	recordCloneRuntimeOrgDuration(time.Since(started), counters...)
	return clone
}

func cloneRuntimeOrgForClass(org storage.OrgState, className, phase string, counters ...*runPerfCounters) storage.OrgState {
	recordCloneRuntimeOrg(className, phase, counters...)
	recordStorageCloneRuntime(counters...)
	capability := "org-frozen-shared"
	if phase == "test-worker" {
		capability = "journal-worker"
	} else if phase == "test" {
		capability = "full-method-isolation"
	}
	recordCloneReason(className, phase, capability, counters...)
	started := time.Now()
	clone := org.CloneRuntimeFrozenShared()
	recordCloneRuntimeOrgDuration(time.Since(started), counters...)
	return clone
}

func cloneRuntimeMachine(machine *vm.VM, counters ...*runPerfCounters) *vm.VM {
	return cloneRuntimeMachineFor(machine, "", "runtime", perfCounterFor(counters))
}

func cloneRuntimeMachineFor(machine *vm.VM, className, phase string, counters *runPerfCounters) *vm.VM {
	recordCloneReason(className, phase, "vm-runtime", counters)
	started := time.Now()
	clone := machine.CloneRuntimeFrozenShared(nil)
	recordCloneRuntimeMachineDuration(time.Since(started), counters)
	return clone
}

func flattenSetupMethods(setups map[string][]vm.Method) []vm.Method {
	total := 0
	for _, methods := range setups {
		total += len(methods)
	}
	out := make([]vm.Method, 0, total)
	for _, methods := range setups {
		out = append(out, methods...)
	}
	return out
}

func testCaseClassSet(cases []TestCase) map[string]bool {
	out := make(map[string]bool, len(cases))
	for _, testCase := range cases {
		out[testCase.ClassName] = true
	}
	return out
}

func methodMapValues(methods map[string]vm.Method) []vm.Method {
	out := make([]vm.Method, 0, len(methods))
	for _, method := range methods {
		out = append(out, method)
	}
	return out
}

// RegisterProjectRuntime compiles project classes, methods, and triggers from an
// index and installs them into the VM. It is used by non-test runtimes that need
// the same supported Apex subset as the local test runner.
func RegisterProjectRuntime(machine *vm.VM, index typesys.Index) error {
	return RegisterProjectRuntimeWithSourceDigests(machine, index, nil)
}

// RegisterProjectRuntimeWithSourceDigests installs a project runtime only
// after the supplied index generation remains valid. A nil digest set keeps
// the documented legacy live-read behavior.
func RegisterProjectRuntimeWithSourceDigests(machine *vm.VM, index typesys.Index, digests *typesys.SourceDigestSet) error {
	sources := newSourceCache()
	_, runtime, err := runtimeFromIndexWithSourceDigests(index, digests, sources)
	if err != nil {
		return err
	}
	methods := runtime.Methods
	classes := runtime.Classes
	triggers := runtime.Triggers
	triggerErrors := runtime.TriggerErrors
	if len(triggerErrors) > 0 {
		return triggerErrors[0]
	}
	return registerBaseRuntime(machine, methods, classes, triggers)
}

// RegisterProjectRuntimeForRequest compiles project classes, methods, and
// triggers for request-scoped server execution. The VM runs static initializers
// lazily when a class is first used, matching request-scoped Apex behavior while
// avoiding eager setup in unrelated project code.
func RegisterProjectRuntimeForRequest(machine *vm.VM, index typesys.Index) error {
	return RegisterProjectRuntimeForRequestWithSourceDigests(machine, index, nil)
}

// RegisterProjectRuntimeForRequestWithSourceDigests compiles a request-scoped
// runtime from an explicit immutable source generation. A nil digest set keeps
// the documented legacy live-read behavior of RegisterProjectRuntimeForRequest.
func RegisterProjectRuntimeForRequestWithSourceDigests(machine *vm.VM, index typesys.Index, digests *typesys.SourceDigestSet) error {
	sources := newSourceCache()
	_, runtime, err := runtimeFromIndexWithSourceDigests(index, digests, sources)
	if err != nil {
		return err
	}
	return RegisterCompiledProjectRuntimeForRequest(machine, compiledProjectRuntimeFromEntry(runtime))
}

func CompileProjectRuntimeForRequest(index typesys.Index) CompiledProjectRuntime {
	runtime, _ := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
	return runtime
}

// CompileProjectRuntimeForRequestWithSourceDigests reuses the exact source
// snapshot that produced index when building runtime cache keys.
func CompileProjectRuntimeForRequestWithSourceDigests(index typesys.Index, digests *typesys.SourceDigestSet) (CompiledProjectRuntime, error) {
	sources := newSourceCache()
	_, runtime, err := runtimeFromIndexWithSourceDigests(index, digests, sources)
	if err != nil {
		return CompiledProjectRuntime{}, err
	}
	return compiledProjectRuntimeFromEntry(runtime), nil
}

func RegisterCompiledProjectRuntimeForRequest(machine *vm.VM, runtime CompiledProjectRuntime) error {
	registerVisualforcePages(machine, runtime.PageNames)
	return registerBaseRuntime(machine, runtime.Methods, runtime.Classes, runtime.Triggers)
}

func RegisterTestMethodForRequest(machine *vm.VM, index typesys.Index, className, methodName string) error {
	className = strings.TrimSpace(className)
	methodName = strings.TrimSpace(methodName)
	if className == "" || methodName == "" {
		return fmt.Errorf("test method requires class and method")
	}
	var selected *TestCase
	for _, testCase := range Discover(index, Options{}) {
		if testCase.ClassName == className && testCase.MethodName == methodName {
			copy := testCase
			selected = &copy
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("test method %s.%s not found", className, methodName)
	}
	methods, errs := compileTestMethods([]TestCase{*selected})
	key := testCaseKey(*selected)
	if err := errs[key]; err != nil {
		return err
	}
	method, ok := methods[key]
	if !ok {
		return fmt.Errorf("test method %s.%s was not compiled", className, methodName)
	}
	return registerTestRuntime(machine, []vm.Method{method})
}

func compiledProjectRuntimeFromEntry(runtime runtimeCacheEntry) CompiledProjectRuntime {
	return CompiledProjectRuntime{
		Methods:   runtime.Methods,
		Classes:   runtime.Classes,
		Triggers:  runtime.Triggers,
		PageNames: runtime.PageNames,
	}
}

func OrgFromIndex(index typesys.Index) storage.OrgState {
	return orgFromIndex(index)
}

func visualforcePageNames(index typesys.Index) []string {
	if index.Project.Root == "" {
		return nil
	}
	p, err := project.Load(index.Project.Root)
	if err != nil {
		return nil
	}
	names := make([]string, 0)
	seen := make(map[string]bool)
	appendVisualforcePageNames(&names, seen, p, false)
	return names
}

func appendVisualforcePageNames(names *[]string, seen map[string]bool, p project.Project, dependency bool) {
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status != "loaded" || dep.Project == nil {
			continue
		}
		appendVisualforcePageNames(names, seen, *dep.Project, true)
	}
	vf, err := visualforce.LoadProject(p)
	if err != nil {
		return
	}
	for _, page := range vf.Pages {
		appendVisualforcePageName(names, seen, page.Name)
		if dependency && p.Namespace != "" {
			appendVisualforcePageName(names, seen, visualforceNamespacedPageName(p.Namespace, page.Name))
		}
	}
}

func appendVisualforcePageName(names *[]string, seen map[string]bool, name string) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" || seen[key] {
		return
	}
	seen[key] = true
	*names = append(*names, name)
}

func visualforceNamespacedPageName(namespace, name string) string {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" || strings.Contains(name, "__") {
		return name
	}
	return namespace + "__" + name
}

func registerVisualforcePages(machine *vm.VM, names []string) {
	for _, name := range names {
		machine.RegisterPageReference(name)
	}
}

type sourceCache struct {
	mu                   sync.RWMutex
	expectedDigests      *typesys.SourceDigestSet
	expectedDigestLookup func(string) ([sha256.Size]byte, bool)
	snapshotErr          error
	files                map[string]string
	rawFiles             map[string]string
	apiVersions          map[string]string
	fileRemaps           map[string][]namespaceremap.Rule
	capturedFiles        map[string][sha256.Size]byte
	capturedMetadata     map[string]typesys.ApexMetadataInput
	metadataAuthority    runtimeMetadataAuthority
	generationValidator  func() error
}

// SourceSnapshotMismatchError reports that an authoritative source snapshot no
// longer matches the raw bytes available for compilation.
type SourceSnapshotMismatchError struct {
	File           string
	ExpectedSHA256 string
	ActualSHA256   string
	Cause          error
}

func (e *SourceSnapshotMismatchError) Error() string {
	if e == nil {
		return "source snapshot mismatch"
	}
	message := fmt.Sprintf("source snapshot mismatch for %s: expected sha256 %s", e.File, e.ExpectedSHA256)
	if e.ActualSHA256 != "" {
		return message + ", got " + e.ActualSHA256
	}
	if e.Cause != nil {
		return message + ": " + e.Cause.Error()
	}
	return message
}

func (e *SourceSnapshotMismatchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newSourceCache() *sourceCache {
	return &sourceCache{files: make(map[string]string), rawFiles: make(map[string]string), apiVersions: make(map[string]string)}
}

func sourceCacheFor(caches []*sourceCache) *sourceCache {
	if len(caches) > 0 && caches[0] != nil {
		return caches[0]
	}
	return newSourceCache()
}

func (cache *sourceCache) bindSourceDigests(digests *typesys.SourceDigestSet) {
	if cache == nil {
		return
	}
	cache.mu.Lock()
	if cache.expectedDigests != digests || cache.expectedDigestLookup != nil {
		cache.files = make(map[string]string)
		cache.rawFiles = make(map[string]string)
		cache.apiVersions = make(map[string]string)
		cache.capturedFiles = nil
		cache.capturedMetadata = nil
		cache.metadataAuthority = ""
		cache.snapshotErr = nil
	}
	// Legacy runtime builds intentionally refresh sidecar metadata on each
	// binding. Artifact-backed builds retain every captured API version,
	// including the empty version, so no live sidecar fallback can alter that
	// snapshot.
	if cache.metadataAuthority != runtimeMetadataAuthorityArtifact {
		cache.apiVersions = make(map[string]string)
	}
	cache.expectedDigests = digests
	cache.expectedDigestLookup = nil
	cache.mu.Unlock()
}

// seedBuildArtifacts binds the runtime source cache to the exact raw source
// occurrences that produced index. These bytes remain authoritative for
// runtime compilation; validateCapturedSourceGeneration separately proves the
// filesystem has not moved on before a cached runtime can be reused.
func (cache *sourceCache) seedBuildArtifacts(index typesys.Index, artifacts *typesys.BuildArtifacts) error {
	if cache == nil || artifacts == nil {
		return nil
	}
	if err := validateBuildArtifacts(index, artifacts); err != nil {
		return err
	}
	cache.bindSourceDigests(artifacts.SourceDigests)
	rawFiles := make(map[string]string)
	apiVersions := make(map[string]string)
	capturedFiles := make(map[string][sha256.Size]byte)
	capturedMetadata := make(map[string]typesys.ApexMetadataInput)
	seedType := func(typ typesys.TypeSymbol) error {
		if !typ.HasSourceSnapshot() {
			return nil
		}
		source, ok := artifacts.SourceForType(typ)
		if !ok {
			return incompleteSourceSnapshotError("missing type source " + typ.File)
		}
		rawFiles[typ.File] = source.RawString()
		capturedFiles[typ.File] = source.Digest()
		metadata, ok := artifacts.ApexMetadataForType(typ)
		if !ok {
			return incompleteSourceSnapshotError("missing Apex metadata input " + typ.File + "-meta.xml")
		}
		capturedMetadata[typ.File+"-meta.xml"] = metadata
		apiVersions[typ.File] = typ.EffectiveAPIVersion
		return nil
	}
	for _, typ := range index.Types {
		if err := seedType(typ); err != nil {
			return err
		}
	}
	for _, trigger := range index.Triggers {
		if !trigger.HasSourceSnapshot() {
			continue
		}
		source, ok := artifacts.SourceForTrigger(trigger)
		if !ok {
			return incompleteSourceSnapshotError("missing trigger source " + trigger.File)
		}
		rawFiles[trigger.File] = source.RawString()
		capturedFiles[trigger.File] = source.Digest()
		metadata, ok := artifacts.ApexMetadataForTrigger(trigger)
		if !ok {
			return incompleteSourceSnapshotError("missing Apex metadata input " + trigger.File + "-meta.xml")
		}
		capturedMetadata[trigger.File+"-meta.xml"] = metadata
		apiVersions[trigger.File] = trigger.EffectiveAPIVersion
	}
	cache.mu.Lock()
	if cache.rawFiles == nil {
		cache.rawFiles = make(map[string]string)
	}
	for file, source := range rawFiles {
		cache.rawFiles[file] = source
	}
	for file, version := range apiVersions {
		cache.apiVersions[file] = version
	}
	cache.capturedFiles = capturedFiles
	cache.capturedMetadata = capturedMetadata
	cache.metadataAuthority = runtimeMetadataAuthorityArtifact
	cache.snapshotErr = nil
	cache.mu.Unlock()
	return nil
}

func sourceBackedRuntimeFiles(index typesys.Index) []string {
	seen := make(map[string]bool)
	files := make([]string, 0, len(index.Types)+len(index.Triggers))
	appendFile := func(file string) {
		if file == "" || seen[file] {
			return
		}
		seen[file] = true
		files = append(files, file)
	}
	for _, typ := range index.Types {
		if typ.HasSourceSnapshot() {
			appendFile(typ.File)
		}
	}
	for _, trigger := range index.Triggers {
		if trigger.HasSourceSnapshot() {
			appendFile(trigger.File)
		}
	}
	sort.Strings(files)
	return files
}

func (cache *sourceCache) prepareSourceGeneration(index typesys.Index, digests *typesys.SourceDigestSet) (runtimeSourceGeneration, error) {
	if cache == nil {
		return runtimeSourceGeneration{}, fmt.Errorf("source cache is nil")
	}
	files := sourceBackedRuntimeFiles(index)
	cache.mu.RLock()
	artifact := cache.metadataAuthority == runtimeMetadataAuthorityArtifact
	cache.mu.RUnlock()
	if artifact {
		generation := cache.sourceGeneration()
		for _, file := range files {
			if _, ok := generation.inputs[file]; !ok {
				return runtimeSourceGeneration{}, incompleteSourceSnapshotError("missing captured source " + file)
			}
		}
		return generation, nil
	}

	rawFiles := make(map[string]string, len(files))
	capturedFiles := make(map[string][sha256.Size]byte, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file) // #nosec G304 -- indexed source path captured for one runtime invocation.
		if err != nil {
			if expected, ok := digestForFile(digests, file); ok {
				return runtimeSourceGeneration{}, sourceSnapshotMismatch(file, expected, nil, err)
			}
			return runtimeSourceGeneration{}, err
		}
		actual := sha256.Sum256(data)
		if expected, ok := digestForFile(digests, file); ok && expected != actual {
			return runtimeSourceGeneration{}, sourceSnapshotMismatch(file, expected, &actual, nil)
		}
		rawFiles[file] = string(data)
		capturedFiles[file] = actual
	}

	cache.mu.Lock()
	cache.files = make(map[string]string)
	cache.rawFiles = rawFiles
	cache.capturedFiles = capturedFiles
	cache.snapshotErr = nil
	cache.mu.Unlock()
	return cache.sourceGeneration(), nil
}

func digestForFile(digests *typesys.SourceDigestSet, file string) ([sha256.Size]byte, bool) {
	if digests == nil {
		return [sha256.Size]byte{}, false
	}
	return digests.Digest(file)
}

func (cache *sourceCache) sourceGeneration() runtimeSourceGeneration {
	if cache == nil {
		return runtimeSourceGeneration{}
	}
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	generation := runtimeSourceGeneration{inputs: make(map[string]runtimeSourceInput, len(cache.capturedFiles))}
	for file, digest := range cache.capturedFiles {
		raw, ok := cache.rawFiles[file]
		if !ok {
			continue
		}
		generation.inputs[file] = runtimeSourceInput{digest: digest, raw: raw}
	}
	return generation
}

func (cache *sourceCache) prepareMetadataGeneration(index typesys.Index) (runtimeMetadataGeneration, error) {
	if cache == nil {
		return runtimeMetadataGeneration{}, fmt.Errorf("source cache is nil")
	}
	cache.mu.RLock()
	artifact := cache.metadataAuthority == runtimeMetadataAuthorityArtifact
	cache.mu.RUnlock()
	if artifact {
		return cache.metadataGeneration(), nil
	}

	inputs := make(map[string]typesys.ApexMetadataInput)
	apiVersions := make(map[string]string)
	for _, file := range sourceBackedRuntimeFiles(index) {
		path := file + "-meta.xml"
		data, err := os.ReadFile(path) // #nosec G304 -- fixed companion of an indexed Apex source.
		if err != nil {
			if !os.IsNotExist(err) {
				return runtimeMetadataGeneration{}, &SourceSnapshotMismatchError{File: path, Cause: err}
			}
			inputs[path] = typesys.ApexMetadataInput{}
			apiVersions[file] = ""
			continue
		}
		inputs[path] = typesys.ApexMetadataInput{Present: true, Digest: sha256.Sum256(data)}
		apiVersions[file] = apiVersionFromApexMetadata(data)
	}

	cache.mu.Lock()
	cache.capturedMetadata = inputs
	cache.apiVersions = apiVersions
	cache.metadataAuthority = runtimeMetadataAuthorityLegacy
	cache.snapshotErr = nil
	cache.mu.Unlock()
	return cache.metadataGeneration(), nil
}

func (cache *sourceCache) metadataGeneration() runtimeMetadataGeneration {
	if cache == nil {
		return runtimeMetadataGeneration{}
	}
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	generation := runtimeMetadataGeneration{
		authority:   cache.metadataAuthority,
		inputs:      make(map[string]typesys.ApexMetadataInput, len(cache.capturedMetadata)),
		apiVersions: make(map[string]string, len(cache.apiVersions)),
	}
	for path, input := range cache.capturedMetadata {
		generation.inputs[path] = input
	}
	for file, version := range cache.apiVersions {
		generation.apiVersions[file] = version
	}
	return generation
}

// validateCapturedSourceGeneration prevents an older BuildArtifacts snapshot
// from returning an in-memory or disk runtime after its source generation has
// changed on disk. It deliberately reads the current physical files while
// compilation itself continues to use the captured arena bytes.
func (cache *sourceCache) validateCapturedSourceGeneration() error {
	if cache != nil && cache.generationValidator != nil {
		return cache.generationValidator()
	}
	return cache.validateCapturedSourceGenerationRaw()
}

func (cache *sourceCache) validateCapturedSourceGenerationRaw() error {
	if cache == nil {
		return nil
	}
	cache.mu.RLock()
	files := make(map[string][sha256.Size]byte, len(cache.capturedFiles))
	for file, digest := range cache.capturedFiles {
		files[file] = digest
	}
	metadata := make(map[string]typesys.ApexMetadataInput, len(cache.capturedMetadata))
	for path, input := range cache.capturedMetadata {
		metadata[path] = input
	}
	cache.mu.RUnlock()
	for file, expected := range files {
		data, err := os.ReadFile(file) // #nosec G304 -- source path is bound to a BuildArtifacts occurrence.
		if err != nil {
			return cache.recordSnapshotMismatch(file, expected, nil, err)
		}
		actual := sha256.Sum256(data)
		if actual != expected {
			return cache.recordSnapshotMismatch(file, expected, &actual, nil)
		}
	}
	for path, expected := range metadata {
		if err := validateApexMetadataInput(path, expected); err != nil {
			return err
		}
	}
	return cache.sourceSnapshotError()
}

func (cache *sourceCache) bindSourceDigestLookup(lookup func(string) ([sha256.Size]byte, bool)) {
	if cache == nil {
		return
	}
	cache.mu.Lock()
	cache.files = make(map[string]string)
	cache.rawFiles = make(map[string]string)
	cache.apiVersions = make(map[string]string)
	cache.capturedFiles = nil
	cache.capturedMetadata = nil
	cache.metadataAuthority = ""
	cache.snapshotErr = nil
	cache.expectedDigests = nil
	cache.expectedDigestLookup = lookup
	cache.mu.Unlock()
}

func (cache *sourceCache) expectedDigest(file string) ([sha256.Size]byte, bool) {
	if cache == nil {
		return [sha256.Size]byte{}, false
	}
	cache.mu.RLock()
	digests := cache.expectedDigests
	lookup := cache.expectedDigestLookup
	cache.mu.RUnlock()
	if lookup != nil {
		return lookup(file)
	}
	if digests == nil {
		return [sha256.Size]byte{}, false
	}
	return digests.Digest(file)
}

func (cache *sourceCache) recordSnapshotMismatch(file string, expected [sha256.Size]byte, actual *[sha256.Size]byte, cause error) error {
	mismatch := &SourceSnapshotMismatchError{
		File:           file,
		ExpectedSHA256: hex.EncodeToString(expected[:]),
		Cause:          cause,
	}
	if actual != nil {
		mismatch.ActualSHA256 = hex.EncodeToString(actual[:])
	}
	cache.mu.Lock()
	if cache.snapshotErr == nil {
		cache.snapshotErr = mismatch
	}
	err := cache.snapshotErr
	cache.mu.Unlock()
	return err
}

func (cache *sourceCache) sourceSnapshotError() error {
	if cache == nil {
		return nil
	}
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.snapshotErr
}

func (cache *sourceCache) retainedRawSource(file string) (string, bool) {
	if cache == nil {
		return "", false
	}
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	source, ok := cache.rawFiles[file]
	return source, ok
}

func (cache *sourceCache) hasArtifactCapturedSource(file string) bool {
	if cache == nil {
		return false
	}
	cache.mu.RLock()
	_, ok := cache.capturedFiles[file]
	artifact := cache.metadataAuthority == runtimeMetadataAuthorityArtifact
	cache.mu.RUnlock()
	return artifact && ok
}

func (cache *sourceCache) apexAPIVersion(file string) string {
	if cache == nil {
		return apiVersionForApexFile(file)
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if version, ok := cache.apiVersions[file]; ok {
		return version
	}
	version := apiVersionForApexFile(file)
	if cache.apiVersions == nil {
		cache.apiVersions = make(map[string]string)
	}
	cache.apiVersions[file] = version
	return version
}

func (cache *sourceCache) read(file string) (string, error) {
	if cache == nil {
		return "", fmt.Errorf("source cache is nil")
	}
	cache.mu.RLock()
	remaps := append([]namespaceremap.Rule(nil), cache.fileRemaps[file]...)
	cache.mu.RUnlock()
	return cache.readWithRemaps(file, remaps)
}

func (cache *sourceCache) configureNamespaceRemaps(types []typesys.TypeSymbol, triggerSets ...[]typesys.TriggerSymbol) {
	if cache == nil {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	fileRemaps := make(map[string][]namespaceremap.Rule)
	for _, typ := range types {
		if !typ.HasSourceSnapshot() || typ.File == "" || len(typ.SourceNamespaceRemaps) == 0 {
			continue
		}
		fileRemaps[typ.File] = append([]namespaceremap.Rule(nil), typ.SourceNamespaceRemaps...)
	}
	for _, triggers := range triggerSets {
		for _, trigger := range triggers {
			if !trigger.HasSourceSnapshot() || trigger.File == "" || len(trigger.SourceNamespaceRemaps) == 0 {
				continue
			}
			fileRemaps[trigger.File] = append([]namespaceremap.Rule(nil), trigger.SourceNamespaceRemaps...)
		}
	}
	cache.fileRemaps = fileRemaps
}

func (cache *sourceCache) readWithRemaps(file string, remaps []namespaceremap.Rule) (string, error) {
	if cache == nil {
		return "", fmt.Errorf("source cache is nil")
	}
	key := sourceCacheKey(file, remaps)
	cache.mu.RLock()
	if source, ok := cache.files[key]; ok {
		cache.mu.RUnlock()
		return source, nil
	}
	rawSource, rawOK := cache.rawFiles[file]
	cache.mu.RUnlock()

	if !rawOK {
		data, err := os.ReadFile(file) // #nosec G304 -- file is an indexed project source path bound to the loaded project snapshot.
		if err != nil {
			if expected, ok := cache.expectedDigest(file); ok {
				return "", cache.recordSnapshotMismatch(file, expected, nil, err)
			}
			return "", err
		}
		digest := sha256.Sum256(data)
		if expected, ok := cache.expectedDigest(file); ok && expected != digest {
			return "", cache.recordSnapshotMismatch(file, expected, &digest, nil)
		}
		rawSource = string(data)
	}
	source := rawSource
	if len(remaps) > 0 {
		source = namespaceremap.ApplySource(remaps, source)
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()
	if existing, ok := cache.files[key]; ok {
		return existing, nil
	}
	if cache.files == nil {
		cache.files = make(map[string]string)
	}
	if cache.rawFiles == nil {
		cache.rawFiles = make(map[string]string)
	}
	if _, ok := cache.rawFiles[file]; !ok {
		cache.rawFiles[file] = rawSource
	}
	cache.files[key] = source
	return source, nil
}

func sourceCacheKey(file string, remaps []namespaceremap.Rule) string {
	fingerprint := namespaceremap.Fingerprint(remaps)
	if fingerprint == "" {
		return file
	}
	return file + "\x00" + fingerprint
}

func passiveStandardRuntimeClasses(indexTypes []typesys.TypeSymbol, existing []vm.Class) []vm.Class {
	seen := make(map[string]bool, len(indexTypes)+len(existing))
	for _, typ := range indexTypes {
		seen[strings.ToLower(typeSymbolRuntimeName(typ))] = true
	}
	for _, class := range existing {
		seen[strings.ToLower(class.Name)] = true
	}
	var out []vm.Class
	for _, typ := range typesys.StandardPlatformSymbolView() {
		if typ.Kind != apexast.DeclarationClass && typ.Kind != apexast.DeclarationInterface && typ.Kind != apexast.DeclarationEnum {
			continue
		}
		name := typeSymbolRuntimeName(typ)
		if name == "" || seen[strings.ToLower(name)] || (typ.Kind != apexast.DeclarationEnum && !isPassiveStandardRuntimeType(name)) {
			continue
		}
		class := passiveRuntimeClassFromTypeSymbol(typ, name)
		out = append(out, class)
		seen[strings.ToLower(name)] = true
	}
	return out
}

func isPassiveStandardRuntimeType(name string) bool {
	dot := strings.IndexByte(name, '.')
	if dot <= 0 {
		return false
	}
	switch strings.ToLower(name[:dot]) {
	case "schema", "apexpages", "messaging", "dom", "system", "database", "test",
		"userinfo", "site", "network", "search", "approval", "security", "eventbus",
		"restcontext", "restrequest", "restresponse":
		return false
	default:
		return true
	}
}

func passiveRuntimeClassFromTypeSymbol(typ typesys.TypeSymbol, name string) vm.Class {
	class := vm.Class{
		Name:         name,
		Namespace:    typ.Namespace,
		SuperClass:   typ.SuperClass,
		Interfaces:   append([]string(nil), typ.Interfaces...),
		Access:       "global",
		Modifiers:    append([]string(nil), typ.Modifiers...),
		IsAbstract:   hasModifier(typ.Modifiers, "abstract"),
		IsInterface:  typ.Kind == apexast.DeclarationInterface,
		Fields:       make(map[string]vm.Field),
		StaticFields: make(map[string]vm.Field),
		Methods:      make(map[string]vm.Method),
	}
	for _, member := range typ.Members {
		switch member.Kind {
		case apexast.DeclarationField, apexast.DeclarationProperty:
			field := vm.Field{
				Name:      member.Name,
				Type:      member.Type,
				Static:    hasModifier(member.Modifiers, "static"),
				Access:    "global",
				Modifiers: append([]string(nil), member.Modifiers...),
				Property:  member.Kind == apexast.DeclarationProperty,
			}
			if field.Static && passiveEnumConstantField(typ, member) {
				field.Value = vm.Value{Kind: vm.ValueObject, Type: name, Text: member.Name}
				field.InitialValue = field.Value
				class.EnumValues = append(class.EnumValues, member.Name)
			} else {
				field.Value = vm.Null
				field.InitialValue = vm.Null
			}
			if field.Static {
				class.StaticFields[field.Name] = field
				class.StaticFieldOrder = append(class.StaticFieldOrder, field.Name)
			} else {
				class.Fields[field.Name] = field
				class.FieldOrder = append(class.FieldOrder, field.Name)
			}
		case apexast.DeclarationConstructor:
			class.Constructors = append(class.Constructors, passiveRuntimeConstructorFromMember(name, member))
		case apexast.DeclarationMethod:
			if passiveGeneratedRuntimeMethod(member) {
				method := passiveRuntimeMethodFromMember(name, member)
				class.Methods[methodShortName(method.Name)+methodParamKey(method.Params)] = method
			}
		}
	}
	return class
}

func passiveRuntimeConstructorFromMember(className string, member typesys.MemberSymbol) vm.Method {
	params := make([]vm.Param, 0, len(member.Parameters))
	for _, param := range member.Parameters {
		params = append(params, vm.Param{Name: param.Name, Type: param.Type})
	}
	return vm.Method{
		Name:          className + ".<init>",
		ClassName:     className,
		ReturnType:    "void",
		Params:        params,
		IsConstructor: true,
		Access:        "global",
	}
}

func passiveRuntimeMethodFromMember(className string, member typesys.MemberSymbol) vm.Method {
	params := make([]vm.Param, 0, len(member.Parameters))
	for i, param := range member.Parameters {
		name := strings.TrimSpace(param.Name)
		if name == "" {
			name = "arg" + fmt.Sprint(i)
		}
		params = append(params, vm.Param{Name: name, Type: param.Type})
	}
	return vm.Method{
		Name:       className + "." + member.Name,
		ClassName:  className,
		ReturnType: member.Type,
		Params:     params,
		IsStatic:   hasModifier(member.Modifiers, "static"),
		Access:     "global",
		Modifiers:  passiveGeneratedMethodModifiers(member),
	}
}

func passiveGeneratedMethodModifiers(member typesys.MemberSymbol) []string {
	modifiers := []string{"passive-generated"}
	if hasModifier(member.Modifiers, "static") {
		modifiers = append(modifiers, "static")
	}
	return modifiers
}

func passiveGeneratedRuntimeMethod(member typesys.MemberSymbol) bool {
	if hasModifier(member.Modifiers, "static") || passiveFluentGeneratedMethod(member) {
		return true
	}
	switch strings.ToLower(member.Name) {
	case "clone", "equals", "hashcode", "ordinal", "tostring":
		return false
	default:
		return true
	}
}

func passiveFluentGeneratedMethod(member typesys.MemberSymbol) bool {
	if !strings.Contains(member.Type, ".") {
		return false
	}
	name := strings.ToLower(member.Name)
	if name == "build" {
		return true
	}
	if len(member.Parameters) == 0 {
		return false
	}
	return name != "clone" && name != "equals" && name != "hashcode" && name != "ordinal" &&
		name != "tostring" && !strings.HasPrefix(name, "get") && !strings.HasPrefix(name, "set") &&
		!strings.HasPrefix(name, "is")
}

func passiveEnumConstantField(typ typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if typ.Kind == apexast.DeclarationEnum {
		return true
	}
	return member.Type == "Object" && member.Name == strings.ToUpper(member.Name)
}

func typeSymbolRuntimeName(typ typesys.TypeSymbol) string {
	if typ.Namespace == "" || strings.Contains(typ.Name, ".") {
		return typ.Name
	}
	return typ.Namespace + "." + typ.Name
}

func projectMethodsByClass(methods map[string]vm.Method) map[string][]vm.Method {
	out := make(map[string][]vm.Method)
	for _, method := range methods {
		key := projectMethodOwnerKey(method.ClassName, method.File)
		out[key] = append(out[key], method)
	}
	return out
}

func projectMethodOwnerKey(className, file string) string {
	return strings.ToLower(strings.TrimSpace(className)) + "\x00" + filepath.Clean(file)
}

func knownTypeNames(types []typesys.TypeSymbol) map[string]bool {
	out := make(map[string]bool, len(types))
	for _, typ := range types {
		out[typ.Name] = true
	}
	return out
}

func qualifyNestedTypeNames(owner string, names []string, known map[string]bool) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, qualifyNestedTypeName(owner, name, known))
	}
	return out
}

func qualifyNestedTypeNameInType(owner, name string, known map[string]bool) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}
	if strings.HasSuffix(name, "[]") {
		element := strings.TrimSpace(strings.TrimSuffix(name, "[]"))
		return qualifyNestedTypeNameInType(owner, element, known) + "[]"
	}
	if open := strings.IndexByte(name, '<'); open > 0 && strings.HasSuffix(name, ">") {
		base := strings.TrimSpace(name[:open])
		args := splitTypeArguments(name[open+1 : len(name)-1])
		for i, arg := range args {
			args[i] = qualifyNestedTypeNameInType(owner, arg, known)
		}
		return base + "<" + strings.Join(args, ",") + ">"
	}
	return qualifyNestedTypeName(owner, name, known)
}

func splitTypeArguments(args string) []string {
	var out []string
	start := 0
	depth := 0
	for i, ch := range args {
		switch ch {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(args[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(args[start:]))
	return out
}

func qualifyNestedTypeName(owner, name string, known map[string]bool) string {
	if name == "" || known[name] || strings.Contains(name, ".") {
		return name
	}
	for {
		candidate := owner + "." + name
		if known[candidate] {
			return candidate
		}
		dot := strings.LastIndex(owner, ".")
		if dot < 0 {
			return name
		}
		owner = owner[:dot]
	}
}

func attachPropertyAccessors(field *vm.Field, className, file string, member typesys.MemberSymbol, source string) {
	for _, accessor := range member.Accessors {
		if accessor.Kind == "set" {
			field.HasSetter = true
		}
		if !accessor.HasBody {
			continue
		}
		method, err := compilePropertyAccessor(className, file, member, accessor, source)
		if err != nil {
			continue
		}
		switch accessor.Kind {
		case "get":
			field.Getter = &method
		case "set":
			field.Setter = &method
		}
	}
}

func projectMethodMapKey(method vm.Method) string {
	return method.Name + methodParamKey(method.Params) + "\x00" + filepath.Clean(method.File)
}

func compileWorkers(total int) int {
	if total < 64 {
		return 1
	}
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		return 1
	}
	if workers > total {
		workers = total
	}
	if workers > 8 {
		workers = 8
	}
	return workers
}

func unsupportedProjectMethod(className, methodName, returnType string, modifiers []string, file string, r diagnostic.Range, source string, cause error) (vm.Method, bool) {
	methodSource, err := extractMethodSource(source, r)
	if err != nil {
		return vm.Method{}, false
	}
	params, err := parseParams(methodSource)
	if err != nil {
		params = nil
	}
	return vm.Method{
		Name:            className + "." + methodName,
		ReturnType:      returnType,
		Params:          params,
		ClassName:       className,
		IsStatic:        hasModifier(modifiers, "static"),
		Access:          accessModifier(modifiers),
		Modifiers:       modifiers,
		File:            file,
		Line:            r.Start.Line,
		Column:          r.Start.Column,
		Unsupported:     cause.Error(),
		RuntimeLowering: isRuntimeLoweringError(cause),
	}, true
}

func isRuntimeLoweringError(err error) bool {
	var loweringErr *vm.RuntimeLoweringError
	return errors.As(err, &loweringErr)
}

func compileTestSetupMethods(index typesys.Index, caches ...*sourceCache) (map[string][]vm.Method, map[string]error, map[string][]ir.Program, map[string]error) {
	return compileTestSetupMethodsForClasses(index, nil, caches...)
}

func compileTestSetupMethodsForClasses(index typesys.Index, selectedClasses map[string]bool, caches ...*sourceCache) (map[string][]vm.Method, map[string]error, map[string][]ir.Program, map[string]error) {
	out := make(map[string][]vm.Method)
	errs := make(map[string]error)
	programs := make(map[string][]ir.Program)
	programErrs := make(map[string]error)
	sources := sourceCacheFor(caches)
	for _, typ := range index.Types {
		if !typ.HasSourceSnapshot() {
			continue
		}
		if typ.Dependency {
			continue
		}
		if typ.Kind != apexast.DeclarationClass {
			continue
		}
		if selectedClasses != nil && !selectedClasses[typ.Name] {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod || !isTestSetup(member.Modifiers) {
				continue
			}
			source, err := sources.read(typ.File)
			if err != nil {
				errs[typ.Name] = err
				continue
			}
			method, err := compileProjectMethod(typ.Name, member.Name, member.Type, member.Modifiers, typ.File, member.Range, source)
			if err != nil {
				errs[typ.Name] = err
				continue
			}
			method.IsStatic = true
			out[typ.Name] = append(out[typ.Name], method)
			program, err := vm.CompileAnonymous(method.Name + "();")
			if err != nil {
				programErrs[typ.Name] = err
				continue
			}
			programs[typ.Name] = append(programs[typ.Name], program)
		}
	}
	return out, errs, programs, programErrs
}

func compileTestMethods(cases []TestCase, caches ...*sourceCache) (map[string]vm.Method, map[string]error) {
	methods := make(map[string]vm.Method, len(cases))
	errs := make(map[string]error)
	sources := sourceCacheFor(caches)
	for _, testCase := range cases {
		key := testCaseKey(testCase)
		source, err := sources.read(testCase.File)
		if err != nil {
			errs[key] = err
			continue
		}
		returnType := testCase.ReturnType
		if returnType == "" {
			returnType = "void"
		}
		modifiers := testCase.Modifiers
		if len(modifiers) == 0 {
			modifiers = []string{"static"}
		}
		method, err := compileProjectMethod(testCase.ClassName, testCase.MethodName, returnType, modifiers, testCase.File, testCase.Range, source)
		if err != nil {
			errs[key] = err
			continue
		}
		methods[key] = method
	}
	return methods, errs
}

func compileTestInvokePrograms(cases []TestCase) (map[string]ir.Program, map[string]error) {
	programs := make(map[string]ir.Program, len(cases))
	errs := make(map[string]error)
	for _, testCase := range cases {
		key := testCaseKey(testCase)
		program, err := vm.CompileAnonymous(testCase.MethodName + "();")
		if err != nil {
			errs[key] = err
			continue
		}
		programs[key] = program
	}
	return programs, errs
}

func testCaseKey(testCase TestCase) string {
	return testCase.ClassName + "." + testCase.MethodName
}

var standardApexTestOrgCache struct {
	once sync.Once
	org  storage.OrgState
}

func standardApexTestOrg() storage.OrgState {
	standardApexTestOrgCache.once.Do(func() {
		org := storage.NewOrgState()
		org.OrgID = "00D000000000001"
		for _, objectName := range storage.KnownStandardObjectNames() {
			storage.EnsureStandardObject(&org, objectName)
		}
		standardApexTestOrgCache.org = org
	})
	return standardApexTestOrgCache.org.Clone()
}

func orgFromIndex(index typesys.Index, caches ...*sourceCache) storage.OrgState {
	sources := sourceCacheFor(caches)
	sources.configureNamespaceRemaps(index.Types, index.Triggers)
	caches = []*sourceCache{sources}
	org := standardApexTestOrg()
	org.Namespace = index.Project.Namespace
	org.OrgID = "00D000000000001"
	registry := sobject.BuildDescribeRegistry(schemaFromIndex(index))
	for name, describe := range registry.Objects {
		org.Objects[name] = storage.ObjectState{
			Definition: sobject.ToObjectDefinition(describe),
			Records:    make(map[storage.ID]storage.Record),
		}
		if storage.IsKnownStandardObject(name) {
			storage.EnsureStandardObject(&org, name)
		}
	}
	features := []string(nil)
	if index.Project.Root != "" {
		features = project.OrgShapeFeatures(index.Project.Root)
		storage.ApplyOrgShape(&org, features)
	}
	applyProjectReferencedStandardFields(&org, index, caches...)
	foldPersonAccountObjectIntoAccount(&org)
	applyApexClassRecords(&org, index, caches...)
	applyCustomMetadataRecordsBestEffort(&org, index.CustomMetadataRecords)
	var loadedProject *project.Project
	if index.Project.Root != "" {
		if p, err := project.Load(index.Project.Root); err == nil {
			loadedProject = &p
			applyCustomApplicationRecords(&org, p.ApplicationFiles)
			applyCustomNotificationTypeRecords(&org, p.Root)
			_ = resource.ApplyProject(&org, p)
			if automationIndex, err := automation.LoadProject(p); err == nil {
				automation.ApplyToOrg(&org, automationIndex)
			}
			applyProjectReferencedRecordTypes(&org, p, caches...)
			applyManagedDependencyReferencedRecordTypes(&org, p, caches...)
			applyProjectDataRelationshipReferences(&org, p)
		}
	}
	storage.EnsureDeterministicPlatformData(&org)
	if len(features) > 0 {
		storage.ApplyOrgShape(&org, features)
	}
	if loadedProject != nil {
		if applyProjectProfileRecordTypes(&org, *loadedProject) {
			storage.EnsureDeterministicPlatformData(&org)
		}
		permissionSetMetadataCache := make(map[string]permissionSetMetadataCacheEntry)
		applyProjectProfileRecordTypeDefaults(&org, *loadedProject)
		applyProjectProfileRecords(&org, *loadedProject, permissionSetMetadataCache)
		applyProjectPermissionSetRecords(&org, *loadedProject, permissionSetMetadataCache)
		storage.EnsureDeterministicPlatformData(&org)
		applyProjectPermissionSetGroupRecords(&org, *loadedProject)
	}
	applyReferencedSyntheticFieldSets(&org, index, caches...)
	normalizeOrgKeyPrefixes(&org)
	return org
}

func applyCustomMetadataRecordsBestEffort(org *storage.OrgState, records []schema.CustomMetadataRecord) {
	if org == nil || len(records) == 0 {
		return
	}
	if err := storage.ApplyCustomMetadataRecords(org, records); err == nil {
		return
	}
	pending := append([]schema.CustomMetadataRecord(nil), records...)
	for len(pending) > 0 {
		next := make([]schema.CustomMetadataRecord, 0, len(pending))
		progressed := false
		for _, record := range pending {
			if err := storage.ApplyCustomMetadataRecords(org, []schema.CustomMetadataRecord{record}); err != nil {
				next = append(next, record)
				continue
			}
			progressed = true
		}
		if !progressed {
			return
		}
		pending = next
	}
}

func foldPersonAccountObjectIntoAccount(org *storage.OrgState) {
	if org == nil {
		return
	}
	personAccount, ok := org.Objects["PersonAccount"]
	if !ok {
		return
	}
	account, ok := org.Objects["Account"]
	if !ok {
		delete(org.Objects, "PersonAccount")
		return
	}
	storage.EnsureStandardObjectFieldsForFeatures(&account.Definition, []string{"PersonAccounts"})
	for _, recordType := range personAccount.Definition.RecordTypes {
		if profileRecordTypeExists(account.Definition.RecordTypes, recordType.DeveloperName) {
			continue
		}
		account.Definition.RecordTypes = append(account.Definition.RecordTypes, recordType)
	}
	org.Objects["Account"] = account
	if recordTypes, ok := org.Objects["RecordType"]; ok {
		for id, record := range recordTypes.Records {
			if strings.EqualFold(record.Fields["SobjectType"].String, "PersonAccount") {
				delete(recordTypes.Records, id)
			}
		}
		org.Objects["RecordType"] = recordTypes
	}
	delete(org.Objects, "PersonAccount")
}

var apexStaticFieldReferencePattern = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z][A-Za-z0-9_]*)\b`)
var apexSchemaSObjectTypeFieldReferencePattern = regexp.MustCompile(`(?i:\bSchema\.SObjectType\.)([A-Za-z_][A-Za-z0-9_]*)(?i:\.fields\.)([A-Za-z][A-Za-z0-9_]*)\b`)
var apexSObjectTypeFieldReferencePattern = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)(?i:\.SObjectType\.fields\.)([A-Za-z][A-Za-z0-9_]*)\b`)
var apexSObjectLiteralPattern = regexp.MustCompile(`(?s)\b([A-Za-z_][A-Za-z0-9_]*)\s*\((.*?)\)`)
var apexNewSObjectLiteralPattern = regexp.MustCompile(`(?s)\bnew\s+([A-Za-z_][A-Za-z0-9_]*)\s*\((.*?)\)`)
var apexNamedArgumentPattern = regexp.MustCompile(`\b([A-Za-z][A-Za-z0-9_]*)\s*=`)
var apexNamedArgumentSObjectIDPattern = regexp.MustCompile(`\b([A-Za-z][A-Za-z0-9_]*)\s*=\s*([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*(?i:Id)\b`)
var apexTypedVariablePattern = regexp.MustCompile(`(?i)\b(?:(?:public|private|protected|global|static|final|transient|testvisible|with|without|inherited|sharing)\s+)*([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)?)\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
var apexVariableFieldReferencePattern = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z][A-Za-z0-9_]*)\b`)
var apexVariableFieldBooleanRightLiteralPattern = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z][A-Za-z0-9_]*)\s*(?:==|!=|=)\s*(?i:\btrue\b|\bfalse\b)`)
var apexVariableFieldBooleanLeftLiteralPattern = regexp.MustCompile(`(?i:\btrue\b|\bfalse\b)\s*(?:==|!=)\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z][A-Za-z0-9_]*)\b`)
var apexVariableFieldBooleanNegationPattern = regexp.MustCompile(`!\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z][A-Za-z0-9_]*)\b`)
var apexVariableFieldNumericRightLiteralPattern = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z][A-Za-z0-9_]*)\s*(?:==|!=|<=|>=|<|>|=)\s*-?(?:\d+(?:\.\d+)?|\.\d+)\b`)
var apexVariableFieldNumericLeftLiteralPattern = regexp.MustCompile(`\b-?(?:\d+(?:\.\d+)?|\.\d+)\s*(?:==|!=|<=|>=|<|>)\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z][A-Za-z0-9_]*)\b`)
var apexCustomSettingGetAllPattern = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)?)\s*\.\s*(?i:getAll)\s*\(`)
var apexExplicitParentChildRelationshipFromPattern = regexp.MustCompile(`(?i:\bFROM\s+)([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*([A-Za-z_][A-Za-z0-9_]*__r)\b`)

type projectChildRelationshipSourceReference struct {
	ParentObject      string
	ChildObject       string
	ChildRelationship string
}

var projectReferencedStandardFieldCache sync.Map

type projectReferencedStandardFieldSet struct {
	Fields   map[string]map[string]storage.Field
	Features []string
}

func apexReferenceScanSource(source string) string {
	out := []byte(source)
	mask := func(start, end int) {
		if start < 0 {
			start = 0
		}
		if end > len(out) {
			end = len(out)
		}
		for i := start; i < end; i++ {
			if out[i] != '\n' && out[i] != '\r' {
				out[i] = ' '
			}
		}
	}
	for i := 0; i < len(out); {
		switch {
		case i+1 < len(out) && out[i] == '/' && out[i+1] == '/':
			start := i
			i += 2
			for i < len(out) && out[i] != '\n' && out[i] != '\r' {
				i++
			}
			mask(start, i)
		case i+1 < len(out) && out[i] == '/' && out[i+1] == '*':
			start := i
			i += 2
			for i+1 < len(out) && !(out[i] == '*' && out[i+1] == '/') {
				i++
			}
			if i+1 < len(out) {
				i += 2
			}
			mask(start, i)
		case out[i] == '\'':
			start := i
			i++
			for i < len(out) {
				if out[i] == '\'' {
					if i+1 < len(out) && out[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				if out[i] == '\\' && i+1 < len(out) {
					i += 2
					continue
				}
				i++
			}
			mask(start, i)
		default:
			i++
		}
	}
	return string(out)
}

func recordProjectReferencedSObjectLiteralFields(org *storage.OrgState, inferred map[string]map[string]storage.Field, childRelationshipLookup map[string]struct{}, scanSource string, pattern *regexp.Regexp) {
	for _, match := range pattern.FindAllStringSubmatchIndex(scanSource, -1) {
		if len(match) != 6 {
			continue
		}
		objectName := scanSource[match[2]:match[3]]
		if _, ok := org.Objects[objectName]; !ok {
			continue
		}
		body := scanSource[match[4]:match[5]]
		for _, argMatch := range apexNamedArgumentPattern.FindAllStringSubmatchIndex(body, -1) {
			if len(argMatch) != 4 {
				continue
			}
			fieldName := body[argMatch[2]:argMatch[3]]
			recordProjectReferencedStandardField(org, inferred, childRelationshipLookup, objectName, fieldName)
		}
	}
}

func recordProjectReferencedSObjectLiteralLookupFields(org *storage.OrgState, inferred map[string]map[string]storage.Field, childRelationshipLookup map[string]struct{}, scanSource string, pattern *regexp.Regexp, varTypes map[string]string) {
	for _, match := range pattern.FindAllStringSubmatchIndex(scanSource, -1) {
		if len(match) != 6 {
			continue
		}
		objectName := scanSource[match[2]:match[3]]
		if _, ok := org.Objects[objectName]; !ok {
			continue
		}
		body := scanSource[match[4]:match[5]]
		for _, argMatch := range apexNamedArgumentSObjectIDPattern.FindAllStringSubmatchIndex(body, -1) {
			if len(argMatch) != 6 {
				continue
			}
			parentObjectName, ok := varTypes[body[argMatch[4]:argMatch[5]]]
			if !ok {
				continue
			}
			recordProjectReferencedLookupFieldWithChildRelationship(org, inferred, childRelationshipLookup, objectName, body[argMatch[2]:argMatch[3]], parentObjectName, "")
		}
	}
}

func projectChildRelationshipSourceReferences(source string) []projectChildRelationshipSourceReference {
	var refs []projectChildRelationshipSourceReference
	for _, match := range apexExplicitParentChildRelationshipFromPattern.FindAllStringSubmatchIndex(source, -1) {
		if len(match) != 6 {
			continue
		}
		parentObject := source[match[2]:match[3]]
		childRelationship := source[match[4]:match[5]]
		childObject := dataRelationshipLookupFieldName(childRelationship)
		if childObject == "" {
			continue
		}
		refs = append(refs, projectChildRelationshipSourceReference{
			ParentObject:      parentObject,
			ChildObject:       childObject,
			ChildRelationship: childRelationship,
		})
	}
	return refs
}

func applyProjectReferencedSourceChildRelationships(org *storage.OrgState, inferred map[string]map[string]storage.Field, refs map[string]projectChildRelationshipSourceReference, childRelationshipLookup map[string]struct{}) {
	for _, ref := range refs {
		canonicalChild, ok := storage.ResolveObjectName(*org, ref.ChildObject)
		if !ok {
			continue
		}
		canonicalParent, ok := storage.ResolveObjectName(*org, ref.ParentObject)
		if !ok {
			continue
		}
		if projectReferencedNameIsChildRelationship(*org, canonicalParent, ref.ChildRelationship, childRelationshipLookup) {
			continue
		}
		fieldName, ok := projectReferencedChildLookupFieldCandidate(inferred[canonicalChild], ref.ChildRelationship, canonicalParent)
		if !ok {
			continue
		}
		recordProjectReferencedLookupFieldWithChildRelationship(org, inferred, childRelationshipLookup, canonicalChild, fieldName, canonicalParent, ref.ChildRelationship)
	}
}

func projectReferencedChildLookupFieldCandidate(fields map[string]storage.Field, childRelationship, parentObject string) (string, bool) {
	parentBase := relationshipFieldBase(parentObject)
	if parentBase == "" {
		return "", false
	}
	prefix := relationshipNamespacePrefix(childRelationship)
	for fieldName := range fields {
		if projectReferencedFieldMatchesParentObject(fieldName, prefix, parentBase) {
			return fieldName, true
		}
	}
	return "", false
}

func projectReferencedFieldMatchesParentObject(fieldName, prefix, parentBase string) bool {
	candidate := parentBase + "__c"
	if strings.EqualFold(fieldName, candidate) {
		return true
	}
	if prefix != "" && strings.EqualFold(fieldName, prefix+candidate) {
		return true
	}
	return false
}

func relationshipNamespacePrefix(name string) string {
	if first := strings.Index(name, "__"); first > 0 {
		return name[:first+2]
	}
	return ""
}

func relationshipFieldBase(objectName string) string {
	name := strings.TrimSpace(objectName)
	if name == "" {
		return ""
	}
	if hasSuffixFold(name, "__c") || hasSuffixFold(name, "__mdt") || hasSuffixFold(name, "__e") {
		name = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(name, "__c"), "__mdt"), "__e")
		if second := strings.Index(name, "__"); second > 0 {
			name = name[second+2:]
		}
	}
	return name
}

func apexMemberReferenceIsCall(source string, end int) bool {
	for end < len(source) {
		switch source[end] {
		case ' ', '\t', '\r', '\n':
			end++
			continue
		case '(':
			return true
		default:
			return false
		}
	}
	return false
}

func recordProjectReferencedStandardField(org *storage.OrgState, inferred map[string]map[string]storage.Field, childRelationshipLookup map[string]struct{}, objectName, fieldName string) {
	if strings.EqualFold(fieldName, "SObjectType") || strings.EqualFold(fieldName, "Fields") {
		return
	}
	if _, ok := org.Objects[objectName]; !ok && isCustomDataObjectKey(objectName) {
		ensurePermissionReferencedObject(org, objectName)
	}
	state, ok := org.Objects[objectName]
	if !ok {
		return
	}
	if _, ok := storage.ResolveFieldName(state.Definition, org.Namespace, fieldName); ok {
		return
	}
	if parentRelationshipKnown(state.Definition, fieldName) {
		return
	}
	if projectReferencedNameIsChildRelationship(*org, objectName, fieldName, childRelationshipLookup) {
		return
	}
	if inferred[objectName] == nil {
		inferred[objectName] = make(map[string]storage.Field)
	}
	if _, _, ok := projectReferencedInferredField(inferred[objectName], fieldName); ok {
		return
	}
	inferred[objectName][fieldName] = inferredReferencedField(fieldName)
}

func recordProjectReferencedBooleanField(org *storage.OrgState, inferred map[string]map[string]storage.Field, childRelationshipLookup map[string]struct{}, objectName, fieldName string) {
	recordProjectReferencedStandardField(org, inferred, childRelationshipLookup, objectName, fieldName)
}

func recordProjectReferencedNumericField(org *storage.OrgState, inferred map[string]map[string]storage.Field, childRelationshipLookup map[string]struct{}, objectName, fieldName string) {
	recordProjectReferencedStandardField(org, inferred, childRelationshipLookup, objectName, fieldName)
}

func recordProjectReferencedListCustomSetting(org *storage.OrgState, objectName string) {
	if org == nil || !hasSuffixFold(objectName, "__c") {
		return
	}
	ensurePermissionReferencedObject(org, objectName)
	canonicalObject, ok := storage.ResolveObjectName(*org, objectName)
	if !ok {
		return
	}
	state := org.Objects[canonicalObject]
	if state.Definition.Metadata == nil {
		state.Definition.Metadata = make(map[string]string)
	}
	if state.Definition.Metadata["kind"] == "" {
		state.Definition.Metadata["kind"] = "customSetting"
	}
	if state.Definition.Metadata["kind"] != "customSetting" {
		return
	}
	if state.Definition.Metadata["customSettingsType"] == "" {
		state.Definition.Metadata["customSettingsType"] = "List"
	}
	if state.Definition.Fields == nil {
		state.Definition.Fields = make(map[string]storage.Field)
	}
	if _, ok := storage.ResolveFieldName(state.Definition, org.Namespace, "Name"); !ok {
		state.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Name", Type: storage.FieldString, DisplayType: "STRING"}
	}
	org.Objects[canonicalObject] = state
}

func recordProjectReferencedLookupField(org *storage.OrgState, inferred map[string]map[string]storage.Field, childRelationshipLookup map[string]struct{}, objectName, fieldName, parentObjectName string) {
	recordProjectReferencedLookupFieldWithChildRelationship(org, inferred, childRelationshipLookup, objectName, fieldName, parentObjectName, "")
}

func recordProjectReferencedLookupFieldWithChildRelationship(org *storage.OrgState, inferred map[string]map[string]storage.Field, childRelationshipLookup map[string]struct{}, objectName, fieldName, parentObjectName, childRelationshipName string) {
	if strings.EqualFold(fieldName, "SObjectType") || strings.EqualFold(fieldName, "Fields") {
		return
	}
	canonicalParentObject, ok := storage.ResolveObjectName(*org, parentObjectName)
	if !ok {
		return
	}
	state, ok := org.Objects[objectName]
	if !ok {
		return
	}
	if _, ok := storage.ResolveFieldName(state.Definition, org.Namespace, fieldName); ok {
		return
	}
	if projectReferencedNameIsChildRelationship(*org, objectName, fieldName, childRelationshipLookup) {
		return
	}
	field := storage.Field{
		APIName:     fieldName,
		Label:       fieldName,
		Type:        storage.FieldReference,
		DisplayType: string(storage.FieldReference),
		ReferenceTo: []string{canonicalParentObject},
	}
	field.RelationshipName = storage.ParentRelationshipName(field)
	field.ChildRelationshipName = childRelationshipName
	if field.RelationshipName == "" || parentRelationshipKnown(state.Definition, field.RelationshipName) {
		return
	}
	if inferred[objectName] == nil {
		inferred[objectName] = make(map[string]storage.Field)
	}
	if existingName, existing, ok := projectReferencedInferredField(inferred[objectName], fieldName); ok {
		if existing.Type == storage.FieldReference {
			if childRelationshipName != "" && existing.ChildRelationshipName == "" {
				existing.ChildRelationshipName = childRelationshipName
				if existing.RelationshipName == "" {
					existing.RelationshipName = storage.ParentRelationshipName(existing)
				}
				if len(existing.ReferenceTo) == 0 {
					existing.ReferenceTo = []string{canonicalParentObject}
				}
				inferred[objectName][existingName] = existing
			}
			return
		}
		delete(inferred[objectName], existingName)
	}
	inferred[objectName][fieldName] = field
}

func projectReferencedInferredField(fields map[string]storage.Field, fieldName string) (string, storage.Field, bool) {
	for existingName, field := range fields {
		if strings.EqualFold(existingName, fieldName) {
			return existingName, field, true
		}
	}
	return "", storage.Field{}, false
}

func parentRelationshipKnown(definition storage.ObjectDefinition, name string) bool {
	for _, relation := range definition.Relations {
		if strings.EqualFold(relation.ParentRelationship, name) {
			return true
		}
	}
	return false
}

func applyReferencedStandardFieldSet(org *storage.OrgState, fields map[string]map[string]storage.Field, childRelationshipLookup map[string]struct{}) {
	for objectName, objectFields := range fields {
		state, ok := org.Objects[objectName]
		if !ok {
			continue
		}
		if state.Definition.Fields == nil {
			state.Definition.Fields = make(map[string]storage.Field)
		}
		for fieldName, field := range objectFields {
			if _, ok := storage.ResolveFieldName(state.Definition, org.Namespace, fieldName); ok {
				continue
			}
			if projectReferencedNameIsChildRelationship(*org, objectName, fieldName, childRelationshipLookup) {
				continue
			}
			state.Definition.Fields[fieldName] = field
			if field.Type == storage.FieldReference && field.RelationshipName != "" && len(field.ReferenceTo) > 0 && !parentRelationshipKnown(state.Definition, field.RelationshipName) {
				state.Definition.Relations = append(state.Definition.Relations, storage.Relationship{
					Field:              field.APIName,
					ParentObjects:      append([]string(nil), field.ReferenceTo...),
					ParentRelationship: field.RelationshipName,
					ChildRelationship:  field.ChildRelationshipName,
					Polymorphic:        len(field.ReferenceTo) > 1,
				})
			}
		}
		org.Objects[objectName] = state
	}
}

var apexFieldSetConstantPattern = regexp.MustCompile(`(?im)\b(?:public|private|protected|global|static|final|\s)*String\s+([A-Za-z_][A-Za-z0-9_]*field_?set[A-Za-z0-9_]*)\s*=\s*(?:[^\r\n;]*\+\s*)?'([^'\r\n;]+)'`)
var apexGetSObjectTypeReturnPattern = regexp.MustCompile(`(?is)\bgetSObjectType\s*\(\s*\)\s*\{[^}]*?\breturn\s+([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)?)\.SObjectType\b`)
var apexSObjectReturnTypePattern = regexp.MustCompile(`(?i)\b(?:public|private|protected|global|webservice|testmethod|static|final|virtual|override|abstract|with|without|inherited|sharing|\s)+([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)?)\s+[A-Za-z_][A-Za-z0-9_]*\s*\(`)
var apexNewSObjectPattern = regexp.MustCompile(`(?i)\bnew\s+([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)?)\s*\(`)
var apexTypedNamePattern = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)?)\s+[A-Za-z_][A-Za-z0-9_]*\b`)

func applyReferencedSyntheticFieldSets(org *storage.OrgState, index typesys.Index, caches ...*sourceCache) {
	if org == nil || len(index.Types) == 0 {
		return
	}
	refs := referencedSyntheticFieldSetReferences(org, index, caches...)
	if len(refs) == 0 {
		return
	}
	for _, ref := range refs {
		if fieldSetMetadataExists(org.Metadata.FieldSets, org.Namespace, ref.ObjectName, ref.FieldSetName) {
			continue
		}
		state := org.Objects[ref.ObjectName]
		org.Metadata.FieldSets = append(org.Metadata.FieldSets, storage.FieldSetMetadata{
			ObjectName: ref.ObjectName,
			Name:       ref.FieldSetName,
			Label:      storage.StripNamespaceToken(org.Namespace, ref.FieldSetName),
			Fields:     syntheticFieldSetMembers(state.Definition, ref.FieldSetName),
		})
	}
}

type syntheticFieldSetReference struct {
	ObjectName    string
	FieldSetName  string
	sortableLower string
}

func referencedSyntheticFieldSetReferences(org *storage.OrgState, index typesys.Index, caches ...*sourceCache) []syntheticFieldSetReference {
	cache := sourceCacheFor(caches)
	fileHasNonTestType := make(map[string]bool)
	fileHasTestTopLevelType := make(map[string]bool)
	for _, typ := range index.Types {
		if !typ.HasSourceSnapshot() || typ.File == "" {
			continue
		}
		if typ.IsTest && typeNameMatchesFileBase(typ.Name, typ.File) {
			fileHasTestTopLevelType[typ.File] = true
			continue
		}
		if !typ.IsTest {
			fileHasNonTestType[typ.File] = true
		}
	}
	seenFiles := make(map[string]bool)
	seenRefs := make(map[string]bool)
	var refs []syntheticFieldSetReference
	for _, typ := range index.Types {
		if !typ.HasSourceSnapshot() || typ.File == "" || seenFiles[typ.File] {
			continue
		}
		seenFiles[typ.File] = true
		if fileHasTestTopLevelType[typ.File] || !fileHasNonTestType[typ.File] {
			continue
		}
		source, err := cache.read(typ.File)
		if err != nil {
			continue
		}
		seenNames := make(map[string]bool)
		seenObjects := make(map[string]bool)
		var names []string
		var objectNames []string
		fieldSetMatches := apexFieldSetConstantPattern.FindAllStringSubmatch(source, -1)
		for _, match := range fieldSetMatches {
			if len(match) != 3 || hasPrefixFold(match[1], "error") {
				continue
			}
			name := strings.TrimSpace(match[2])
			if name == "" || seenNames[strings.ToLower(name)] {
				continue
			}
			seenNames[strings.ToLower(name)] = true
			names = append(names, name)
		}
		if len(names) == 0 {
			continue
		}
		addObject := func(objectName string) {
			canonical, ok := storage.ResolveObjectName(*org, objectName)
			if !ok || seenObjects[canonical] {
				return
			}
			if len(syntheticFieldSetMembers(org.Objects[canonical].Definition, "")) == 0 {
				return
			}
			seenObjects[canonical] = true
			objectNames = append(objectNames, canonical)
		}
		for _, match := range apexGetSObjectTypeReturnPattern.FindAllStringSubmatch(source, -1) {
			if len(match) == 2 {
				addObject(match[1])
			}
		}
		for _, match := range apexSObjectReturnTypePattern.FindAllStringSubmatch(source, -1) {
			if len(match) == 2 {
				addObject(match[1])
			}
		}
		for _, match := range apexNewSObjectPattern.FindAllStringSubmatch(source, -1) {
			if len(match) == 2 {
				addObject(match[1])
			}
		}
		for _, match := range apexTypedNamePattern.FindAllStringSubmatch(source, -1) {
			if len(match) == 2 {
				addObject(match[1])
			}
		}
		sort.Strings(names)
		sort.Strings(objectNames)
		for _, objectName := range objectNames {
			for _, name := range names {
				key := strings.ToLower(objectName) + "\x00" + strings.ToLower(name)
				if seenRefs[key] {
					continue
				}
				seenRefs[key] = true
				refs = append(refs, syntheticFieldSetReference{
					ObjectName:    objectName,
					FieldSetName:  name,
					sortableLower: key,
				})
			}
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].sortableLower < refs[j].sortableLower
	})
	return refs
}

func fieldSetMetadataExists(fieldSets []storage.FieldSetMetadata, namespace, objectName, name string) bool {
	for _, fieldSet := range fieldSets {
		if metadataNameMatches(namespace, objectName, fieldSet.ObjectName) && fieldSetNameMatches(namespace, fieldSet.Name, name) {
			return true
		}
	}
	return false
}

func fieldSetNameMatches(namespace, left, right string) bool {
	for _, leftAlias := range metadataNameAliases(namespace, left) {
		for _, rightAlias := range metadataNameAliases(namespace, right) {
			if strings.EqualFold(leftAlias, rightAlias) {
				return true
			}
		}
	}
	return false
}

func metadataNameMatches(namespace, left, right string) bool {
	for _, leftAlias := range metadataNameAliases(namespace, left) {
		for _, rightAlias := range metadataNameAliases(namespace, right) {
			if strings.EqualFold(leftAlias, rightAlias) {
				return true
			}
		}
	}
	return false
}

func metadataNameAliases(namespace, name string) []string {
	seen := make(map[string]bool)
	var aliases []string
	add := func(alias string) {
		alias = strings.TrimSpace(alias)
		if alias == "" || seen[strings.ToLower(alias)] {
			return
		}
		seen[strings.ToLower(alias)] = true
		aliases = append(aliases, alias)
	}
	add(name)
	if namespace != "" && !hasPrefixFold(name, namespace+"__") {
		add(namespace + "__" + name)
	}
	if namespace != "" {
		add(storage.StripNamespaceToken(namespace, name))
	}
	add(stripAnyNamespaceToken(name))
	return aliases
}

func typeNameMatchesFileBase(typeName, file string) bool {
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	if base == "" {
		return false
	}
	parts := strings.Split(typeName, ".")
	shortName := parts[len(parts)-1]
	return strings.EqualFold(shortName, base)
}

func hasPrefixFold(value, prefix string) bool {
	return len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix)
}

func stripAnyNamespaceToken(name string) string {
	parts := strings.Split(name, "__")
	if len(parts) >= 3 {
		return strings.Join(parts[1:], "__")
	}
	return name
}

func syntheticFieldSetMembers(definition storage.ObjectDefinition, fieldSetName string) []storage.FieldSetMemberMetadata {
	var fieldNames []string
	for fieldName, field := range definition.Fields {
		if !syntheticFieldSetCanEditField(fieldName, field) {
			continue
		}
		if syntheticFieldSetIsAddressLike(fieldSetName) && !syntheticFieldSetFieldIsAddressLike(fieldName, field) {
			continue
		}
		fieldNames = append(fieldNames, fieldName)
	}
	sort.Strings(fieldNames)
	members := make([]storage.FieldSetMemberMetadata, 0, len(fieldNames))
	for _, fieldName := range fieldNames {
		field := definition.Fields[fieldName]
		members = append(members, storage.FieldSetMemberMetadata{
			Field:    fieldName,
			Required: syntheticFieldSetMemberRequired(definition.APIName, fieldName, field),
		})
	}
	return members
}

func syntheticFieldSetMemberRequired(objectName, fieldName string, field storage.Field) bool {
	if field.Required {
		return true
	}
	return strings.EqualFold(objectName, "Account") &&
		(strings.EqualFold(fieldName, "Name") || strings.EqualFold(fieldName, "LastName") ||
			strings.EqualFold(field.APIName, "Name") || strings.EqualFold(field.APIName, "LastName"))
}

func syntheticFieldSetIsAddressLike(name string) bool {
	return strings.Contains(strings.ToLower(stripAnyNamespaceToken(name)), "address")
}

func syntheticFieldSetFieldIsAddressLike(fieldName string, field storage.Field) bool {
	haystack := strings.ToLower(fieldName + " " + field.APIName + " " + field.Label)
	for _, token := range []string{"address", "street", "city", "state", "province", "country", "postal", "zip", "billing", "mailing", "shipping"} {
		if strings.Contains(haystack, token) {
			return true
		}
	}
	return false
}

func syntheticFieldSetCanEditField(fieldName string, field storage.Field) bool {
	if strings.EqualFold(fieldName, "Id") || strings.EqualFold(field.APIName, "Id") || field.Formula != "" || field.AutoNumber {
		return false
	}
	if field.Type == storage.FieldBoolean {
		return false
	}
	if !storage.FieldFlagValue(field.Createable, true) && !storage.FieldFlagValue(field.Updateable, true) {
		return false
	}
	return true
}

func projectReferencedNameIsChildRelationship(org storage.OrgState, objectName, name string, relationshipLookup map[string]struct{}) bool {
	parentName, ok := storage.ResolveObjectName(org, objectName)
	if !ok {
		parentName = objectName
	}
	if relationshipLookup == nil {
		relationshipLookup = projectReferencedChildRelationshipLookup(org)
	}
	_, ok = relationshipLookup[strings.ToLower(parentName)+"\x00"+strings.ToLower(name)]
	return ok
}

func projectReferencedChildRelationshipLookup(org storage.OrgState) map[string]struct{} {
	lookup := make(map[string]struct{}, len(org.Objects))
	parentNames := make(map[string]string)
	for _, child := range org.Objects {
		for _, relation := range child.Definition.Relations {
			if relation.ChildRelationship == "" {
				continue
			}
			relationName := strings.ToLower(relation.ChildRelationship)
			for _, parent := range relation.ParentObjects {
				parentName := resolvedChildRelationshipParentName(org, parentNames, parent)
				lookup[strings.ToLower(parentName)+"\x00"+relationName] = struct{}{}
			}
		}
	}
	return lookup
}

func resolvedChildRelationshipParentName(org storage.OrgState, names map[string]string, parent string) string {
	key := strings.ToLower(strings.TrimSpace(parent))
	if cached, ok := names[key]; ok {
		return cached
	}
	parentName, ok := storage.ResolveObjectName(org, parent)
	if !ok {
		parentName = parent
	}
	names[key] = parentName
	return parentName
}

func inferredReferencedField(fieldName string) storage.Field {
	return storage.Field{APIName: fieldName, Label: fieldName, Type: storage.FieldAny}
}

func hasSuffixFold(value, suffix string) bool {
	if len(value) < len(suffix) {
		return false
	}
	return strings.EqualFold(value[len(value)-len(suffix):], suffix)
}

type profileRecordTypeVisibilityXML struct {
	Default    bool   `xml:"default"`
	RecordType string `xml:"recordType"`
	Visible    bool   `xml:"visible"`
}

type profileXML struct {
	RecordTypeVisibilities []profileRecordTypeVisibilityXML `xml:"recordTypeVisibilities"`
}

type projectRecordTypeReference struct {
	ObjectName    string
	DeveloperName string
	Name          string
}

type projectDataRelationshipReference struct {
	ChildObject        string
	ParentRelationship string
}

type projectDataFieldReference struct {
	ObjectName string
	FieldName  string
}

var apexRecordTypeInfoCallRE = regexp.MustCompile(`(?is)(?:(?:Schema\s*\.\s*)?SObjectType\s*\.\s*([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)*)(?:\s*\.\s*getDescribe\s*\(\s*\))?|([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)*)\s*\.\s*SObjectType(?:\s*\.\s*getDescribe\s*\(\s*\))?)\s*\.\s*getRecordTypeInfosBy(Name|DeveloperName)\s*\(\s*\)\s*\.\s*get\s*\(\s*['"]([^'"]+)['"]\s*\)`)
var dataRecordTypeQueryRE = regexp.MustCompile(`(?is)FROM\s+RecordType\b[^"\n]*\bSObjectType\s*=\s*'([^']+)'[^"\n]*\bName\s*=\s*'([^']+)'`)
var projectRecordTypeConfigRE = regexp.MustCompile(`(?im)\brecord_type:\s*([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)?)\.([A-Za-z_][A-Za-z0-9_]*)\b`)
var apexGetSObjectTypeReturnRE = regexp.MustCompile(`(?is)\bgetSObjectType\s*\(\s*\)\s*\{.*?\breturn\s+([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)*)\s*\.\s*SObjectType\s*;`)
var apexStaticFinalStringRE = regexp.MustCompile(`(?is)\bstatic\s+final\s+String\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*['"]([^'"]+)['"]`)
var apexGetRecordTypeIdCallRE = regexp.MustCompile(`(?is)\bgetRecordTypeId\s*\(\s*(?:'([^']+)'|"([^"]+)"|([A-Za-z_][A-Za-z0-9_]*))\s*\)`)
var apexRecordTypeStringMethodRE = regexp.MustCompile(`(?is)\b([A-Za-z_][A-Za-z0-9_]*)\s*\(\s*String\s+([A-Za-z_][A-Za-z0-9_]*)\s*\)\s*\{([^{}]*)\}`)

func scanRecordTypesFromApexSource(source string) []projectRecordTypeReference {
	var refs []projectRecordTypeReference
	for _, match := range apexRecordTypeInfoCallRE.FindAllStringSubmatch(source, -1) {
		objectName := match[1]
		if objectName == "" {
			objectName = match[2]
		}
		kind := strings.ToLower(match[3])
		value := strings.TrimSpace(match[4])
		ref := projectRecordTypeReference{ObjectName: objectName}
		if kind == "developername" {
			ref.DeveloperName = value
			ref.Name = recordTypeLabelFromDeveloperName(value)
		} else {
			ref.DeveloperName = recordTypeDeveloperNameFromLabel(value)
			ref.Name = value
		}
		refs = append(refs, ref)
	}
	return append(refs, projectReferencedTestDataHelperRecordTypes(source)...)
}

func parallelScanProjectReferencedRecordTypes(files []string, cache *sourceCache) []projectRecordTypeReference {
	if len(files) == 0 {
		return nil
	}
	workers := compileWorkers(len(files))
	if workers <= 1 {
		var merged []projectRecordTypeReference
		for _, file := range files {
			source, err := cache.read(file)
			if err != nil {
				continue
			}
			merged = append(merged, scanRecordTypesFromApexSource(source)...)
		}
		return merged
	}
	jobs := make(chan string)
	results := make(chan []projectRecordTypeReference, len(files))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range jobs {
				source, err := cache.read(file)
				if err != nil {
					continue
				}
				results <- scanRecordTypesFromApexSource(source)
			}
		}()
	}
	go func() {
		for _, file := range files {
			jobs <- file
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	var chunks [][]projectRecordTypeReference
	for chunk := range results {
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}
	}
	return mergeProjectRecordTypeReferences(chunks...)
}

func mergeProjectRecordTypeReferences(chunks ...[]projectRecordTypeReference) []projectRecordTypeReference {
	seen := make(map[string]bool)
	var refs []projectRecordTypeReference
	add := func(ref projectRecordTypeReference) {
		ref.ObjectName = strings.TrimSpace(ref.ObjectName)
		ref.DeveloperName = strings.TrimSpace(ref.DeveloperName)
		ref.Name = strings.TrimSpace(ref.Name)
		if ref.ObjectName == "" || ref.DeveloperName == "" || ref.Name == "" {
			return
		}
		key := strings.ToLower(ref.ObjectName) + "\x00" + strings.ToLower(ref.DeveloperName)
		if seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, ref)
	}
	for _, chunk := range chunks {
		for _, ref := range chunk {
			add(ref)
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].ObjectName != refs[j].ObjectName {
			return refs[i].ObjectName < refs[j].ObjectName
		}
		return refs[i].DeveloperName < refs[j].DeveloperName
	})
	return refs
}

func projectReferencedRecordTypes(p project.Project, caches ...*sourceCache) []projectRecordTypeReference {
	cache := sourceCacheFor(caches)
	var staticRefs []projectRecordTypeReference
	for _, file := range projectDataJSONFiles(p.Root) {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, match := range dataRecordTypeQueryRE.FindAllStringSubmatch(string(data), -1) {
			name := strings.TrimSpace(match[2])
			staticRefs = append(staticRefs, projectRecordTypeReference{
				ObjectName:    match[1],
				DeveloperName: recordTypeDeveloperNameFromLabel(name),
				Name:          name,
			})
		}
	}
	for _, file := range []string{filepath.Join(p.Root, "cumulusci.yml")} {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, match := range projectRecordTypeConfigRE.FindAllStringSubmatch(string(data), -1) {
			staticRefs = append(staticRefs, projectRecordTypeReference{
				ObjectName:    match[1],
				DeveloperName: match[2],
				Name:          recordTypeLabelFromDeveloperName(match[2]),
			})
		}
	}
	return mergeProjectRecordTypeReferences(
		parallelScanProjectReferencedRecordTypes(p.ApexFiles, cache),
		staticRefs,
	)
}

func projectReferencedTestDataHelperRecordTypes(source string) []projectRecordTypeReference {
	objectMatch := apexGetSObjectTypeReturnRE.FindStringSubmatch(source)
	if len(objectMatch) != 2 {
		return nil
	}
	objectName := objectMatch[1]
	constants := make(map[string]string)
	for _, match := range apexStaticFinalStringRE.FindAllStringSubmatch(source, -1) {
		if len(match) == 3 {
			constants[match[1]] = match[2]
		}
	}
	var refs []projectRecordTypeReference
	for _, match := range apexGetRecordTypeIdCallRE.FindAllStringSubmatch(source, -1) {
		if name := projectRecordTypeNameFromCallArg(match, constants); name != "" {
			refs = append(refs, projectRecordTypeReference{
				ObjectName:    objectName,
				DeveloperName: recordTypeDeveloperNameFromLabel(name),
				Name:          name,
			})
		}
	}
	methods := apexRecordTypeForwardingMethods(source)
	callPatterns := make(map[string]*regexp.Regexp, len(methods))
	for _, forwardingMethod := range methods {
		callRE := callPatterns[forwardingMethod]
		if callRE == nil {
			callRE = regexp.MustCompile(`(?is)\b` + regexp.QuoteMeta(forwardingMethod) + `\s*\(\s*(?:'([^']+)'|"([^"]+)"|([A-Za-z_][A-Za-z0-9_]*))\s*\)`)
			callPatterns[forwardingMethod] = callRE
		}
		for _, match := range callRE.FindAllStringSubmatch(source, -1) {
			if name := projectRecordTypeNameFromCallArg(match, constants); name != "" {
				refs = append(refs, projectRecordTypeReference{
					ObjectName:    objectName,
					DeveloperName: recordTypeDeveloperNameFromLabel(name),
					Name:          name,
				})
			}
		}
	}
	return refs
}

func apexRecordTypeForwardingMethods(source string) []string {
	seen := make(map[string]bool)
	var methods []string
	for _, match := range apexRecordTypeStringMethodRE.FindAllStringSubmatch(source, -1) {
		if len(match) < 4 || seen[match[1]] {
			continue
		}
		if !regexp.MustCompile(`(?is)\bgetRecordTypeId\s*\(\s*` + regexp.QuoteMeta(match[2]) + `\s*\)`).MatchString(match[3]) {
			continue
		}
		seen[match[1]] = true
		methods = append(methods, match[1])
	}
	return methods
}

func projectRecordTypeNameFromCallArg(match []string, constants map[string]string) string {
	if len(match) != 4 {
		return ""
	}
	name := match[1]
	if name == "" {
		name = match[2]
	}
	if name == "" && match[3] != "" {
		name = constants[match[3]]
	}
	return strings.TrimSpace(name)
}

func projectDataFieldReferences(root string) []projectDataFieldReference {
	seen := make(map[string]bool)
	var refs []projectDataFieldReference
	add := func(objectName, fieldName string, value any) {
		objectName = strings.TrimSpace(objectName)
		fieldName = strings.TrimSpace(fieldName)
		if objectName == "" || fieldName == "" || strings.Contains(fieldName, ".") || fieldName == "attributes" {
			return
		}
		key := strings.ToLower(objectName) + "\x00" + strings.ToLower(fieldName)
		if seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, projectDataFieldReference{ObjectName: objectName, FieldName: fieldName})
	}
	for _, file := range projectDataJSONFiles(root) {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			continue
		}
		collectDataFieldReferences(value, "", add)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].ObjectName != refs[j].ObjectName {
			return refs[i].ObjectName < refs[j].ObjectName
		}
		return refs[i].FieldName < refs[j].FieldName
	})
	return refs
}

func collectDataFieldReferences(value any, currentObject string, add func(objectName, fieldName string, value any)) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectDataFieldReferences(item, currentObject, add)
		}
	case map[string]any:
		for key, child := range typed {
			if currentObject != "" && !strings.Contains(key, ".") {
				add(currentObject, key, child)
			}
			if isCustomDataObjectKey(key) && !isFieldDescribeDescriptor(child) {
				collectDataFieldReferences(child, key, add)
				continue
			}
			if key == "records" {
				collectDataFieldReferences(child, "", add)
				continue
			}
			if currentObject == "" {
				collectDataFieldReferences(child, "", add)
			}
		}
	}
}

func projectDataRelationshipReferences(root string) []projectDataRelationshipReference {
	seen := make(map[string]bool)
	var refs []projectDataRelationshipReference
	add := func(childObject, parentRelationship string) {
		childObject = strings.TrimSpace(childObject)
		parentRelationship = strings.TrimSpace(parentRelationship)
		if childObject == "" || parentRelationship == "" || dataRelationshipLookupFieldName(parentRelationship) == "" {
			return
		}
		key := strings.ToLower(childObject) + "\x00" + strings.ToLower(parentRelationship)
		if seen[key] {
			return
		}
		seen[key] = true
		refs = append(refs, projectDataRelationshipReference{ChildObject: childObject, ParentRelationship: parentRelationship})
	}
	for _, file := range projectDataJSONFiles(root) {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			continue
		}
		collectDataRelationshipReferences(value, "", add)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].ChildObject != refs[j].ChildObject {
			return refs[i].ChildObject < refs[j].ChildObject
		}
		return refs[i].ParentRelationship < refs[j].ParentRelationship
	})
	return refs
}

func collectDataRelationshipReferences(value any, currentObject string, add func(childObject, parentRelationship string)) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectDataRelationshipReferences(item, currentObject, add)
		}
	case map[string]any:
		for key, child := range typed {
			if currentObject != "" {
				if relationship, _, ok := strings.Cut(key, "."); ok {
					add(currentObject, relationship)
				}
			}
			if isCustomDataObjectKey(key) {
				collectDataRelationshipReferences(child, key, add)
				continue
			}
			if key == "records" {
				collectDataRelationshipReferences(child, "", add)
				continue
			}
			if currentObject == "" {
				collectDataRelationshipReferences(child, "", add)
			}
		}
	}
}

func dataRelationshipLookupFieldName(relationshipName string) string {
	name := strings.TrimSpace(relationshipName)
	if hasSuffixFold(name, "__r") {
		return name[:len(name)-len("__r")] + "__c"
	}
	return ""
}

func isCustomDataObjectKey(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(lower, "__c") || strings.HasSuffix(lower, "__e") || strings.HasSuffix(lower, "__mdt")
}

// isFieldDescribeDescriptor reports whether a JSON value is a UI-API field
// wrapper (a getObjectInfo field descriptor or a getRecord field value) rather
// than a nested record. UI-API keys its `fields` map by field API name (which
// ends in __c), so the field name would otherwise be mistaken for a nested
// custom object and synthesized into a phantom object. Describe descriptors
// always carry both `apiName` and `dataType`; record field values carry `value`
// and `displayValue`.
func isFieldDescribeDescriptor(value any) bool {
	descriptor, ok := value.(map[string]any)
	if !ok {
		return false
	}
	if _, hasAPIName := descriptor["apiName"]; hasAPIName {
		if _, hasDataType := descriptor["dataType"]; hasDataType {
			return true
		}
	}
	if _, hasValue := descriptor["value"]; hasValue {
		if _, hasDisplay := descriptor["displayValue"]; hasDisplay {
			return true
		}
	}
	return false
}

func projectDataJSONFiles(root string) []string {
	if root == "" {
		return nil
	}
	var files []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path != root && d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || shouldSkipProjectDataDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		if strings.Contains(filepath.ToSlash(path), "/data/") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func shouldSkipProjectDataDir(name string) bool {
	switch strings.ToLower(name) {
	case "node_modules", "vendor", "dist", "bin", "coverage", "__tests__", "__mocks__":
		return true
	default:
		return false
	}
}

func recordTypeDeveloperNameFromLabel(label string) string {
	var out strings.Builder
	capNext := true
	for _, r := range strings.TrimSpace(label) {
		alphaNum := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if alphaNum {
			if capNext && r >= 'a' && r <= 'z' {
				r -= 'a' - 'A'
			}
			out.WriteRune(r)
			capNext = false
			continue
		}
		if r == '_' {
			out.WriteRune(r)
		}
		capNext = true
	}
	return out.String()
}

func recordTypeLabelFromDeveloperName(developerName string) string {
	developerName = strings.TrimSpace(developerName)
	if developerName == "" {
		return ""
	}
	var out strings.Builder
	var prev rune
	for i, r := range developerName {
		if r == '_' {
			out.WriteRune(' ')
			prev = ' '
			continue
		}
		if i > 0 && prev != ' ' && r >= 'A' && r <= 'Z' && prev >= 'a' && prev <= 'z' {
			out.WriteRune(' ')
		}
		out.WriteRune(r)
		prev = r
	}
	return out.String()
}

type projectProfileRecordTypeVisibility struct {
	ObjectName    string
	DeveloperName string
	Name          string
	Default       bool
	PersonAccount bool
}

func loadProfileRecordTypeVisibilities(file string) map[string]projectProfileRecordTypeVisibility {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var raw profileXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	out := make(map[string]projectProfileRecordTypeVisibility)
	for _, visibility := range raw.RecordTypeVisibilities {
		if !visibility.Visible {
			continue
		}
		objectName, developerName, ok := strings.Cut(strings.TrimSpace(visibility.RecordType), ".")
		if !ok || objectName == "" || developerName == "" {
			continue
		}
		developerName = stripRecordTypeNamespaceToken(developerName)
		key := strings.ToLower(objectName) + "\x00" + strings.ToLower(developerName)
		if _, exists := out[key]; exists {
			continue
		}
		out[key] = projectProfileRecordTypeVisibility{
			ObjectName:    objectName,
			DeveloperName: developerName,
			Name:          strings.ReplaceAll(developerName, "_", " "),
			Default:       visibility.Default,
			PersonAccount: strings.EqualFold(objectName, "PersonAccount"),
		}
	}
	return out
}

func profileRecordTypeObjectName(org storage.OrgState, objectName string) (string, bool) {
	if strings.EqualFold(objectName, "PersonAccount") {
		return storage.ResolveObjectName(org, "Account")
	}
	return storage.ResolveObjectName(org, objectName)
}

func stripRecordTypeNamespaceToken(name string) string {
	name = strings.TrimSpace(name)
	idx := strings.Index(name, "__")
	if idx <= 0 || idx+2 >= len(name) {
		return name
	}
	return name[idx+2:]
}

func profileRecordTypeExists(recordTypes []storage.RecordTypeInfo, developerName string) bool {
	developerName = strings.TrimSpace(developerName)
	strippedDeveloperName := stripRecordTypeNamespaceToken(developerName)
	for _, recordType := range recordTypes {
		if profileRecordTypeNameMatches(recordType.DeveloperName, developerName, strippedDeveloperName) ||
			profileRecordTypeNameMatches(recordType.Name, developerName, strippedDeveloperName) {
			return true
		}
	}
	return false
}

func profileRecordTypeNameMatches(candidate, name, strippedName string) bool {
	candidate = strings.TrimSpace(candidate)
	if strings.EqualFold(candidate, name) {
		return true
	}
	return strippedName != "" && strings.EqualFold(stripRecordTypeNamespaceToken(candidate), strippedName)
}

func projectProfileRecordTypeDefaults(files []string) map[string]string {
	defaults := make(map[string]string)
	for _, file := range sortedProfileFilesForDefaults(files) {
		for objectName, developerName := range loadProfileRecordTypeDefaults(file) {
			if _, exists := defaults[objectName]; !exists {
				defaults[objectName] = developerName
			}
		}
	}
	return defaults
}

func sortedProfileFilesForDefaults(files []string) []string {
	out := append([]string(nil), files...)
	sort.SliceStable(out, func(i, j int) bool {
		leftAdmin := strings.EqualFold(profileNameFromPath(out[i]), "Admin")
		rightAdmin := strings.EqualFold(profileNameFromPath(out[j]), "Admin")
		if leftAdmin != rightAdmin {
			return leftAdmin
		}
		return out[i] < out[j]
	})
	return out
}

func loadProfileRecordTypeDefaults(file string) map[string]string {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var raw profileXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	out := make(map[string]string)
	for _, visibility := range raw.RecordTypeVisibilities {
		if !visibility.Default || !visibility.Visible {
			continue
		}
		objectName, developerName, ok := strings.Cut(strings.TrimSpace(visibility.RecordType), ".")
		if !ok || objectName == "" || developerName == "" {
			continue
		}
		if strings.EqualFold(objectName, "PersonAccount") {
			continue
		}
		developerName = stripRecordTypeNamespaceToken(developerName)
		if _, exists := out[objectName]; !exists {
			out[objectName] = developerName
		}
	}
	return out
}

func profileNameFromPath(path string) string {
	name := filepath.Base(path)
	for _, suffix := range []string{".profile-meta.xml", ".profile"} {
		if hasSuffixFold(name, suffix) {
			return strings.TrimSpace(name[:len(name)-len(suffix)])
		}
	}
	return ""
}

func profileRecordExists(state storage.ObjectState, name string) bool {
	for _, record := range state.Records {
		if strings.EqualFold(record.Fields["Name"].String, name) {
			return true
		}
	}
	return false
}

func permissionSetCustomPermissionValues(metadata permissionSetMetadata) []storage.Value {
	var out []storage.Value
	for _, permission := range metadata.CustomPermission {
		name := strings.TrimSpace(permission.Name)
		if name == "" || !permission.Enabled {
			continue
		}
		out = append(out, storage.StringValue(name))
	}
	return out
}

type permissionSetMetadataCacheEntry struct {
	metadata permissionSetMetadata
	ok       bool
}

func readPermissionSetMetadata(file string, cache map[string]permissionSetMetadataCacheEntry) (permissionSetMetadata, bool) {
	if cache == nil {
		return readPermissionSetMetadataNoCache(file)
	}
	if cached, ok := cache[file]; ok {
		return cached.metadata, cached.ok
	}
	metadata, ok := readPermissionSetMetadataNoCache(file)
	cache[file] = permissionSetMetadataCacheEntry{
		metadata: metadata,
		ok:       ok,
	}
	return metadata, ok
}

func readPermissionSetMetadataNoCache(file string) (permissionSetMetadata, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return permissionSetMetadata{}, false
	}
	var metadata permissionSetMetadata
	if err := xml.Unmarshal(data, &metadata); err != nil {
		return permissionSetMetadata{}, false
	}
	return metadata, true
}

func ensurePermissionReferencedObject(org *storage.OrgState, objectName string) {
	if org == nil || strings.TrimSpace(objectName) == "" {
		return
	}
	if _, ok := storage.ResolveObjectName(*org, objectName); ok {
		return
	}
	if !strings.Contains(objectName, "__") {
		return
	}
	storage.EnsureStandardObject(org, objectName)
}

func ensurePermissionReferencedObjectField(org *storage.OrgState, objectName, qualifiedFieldName string) {
	ensureProjectDataReferencedObjectField(org, objectName, qualifiedFieldName)
}

func ensureProjectDataReferencedObjectField(org *storage.OrgState, objectName, qualifiedFieldName string) {
	if org == nil || strings.TrimSpace(objectName) == "" || strings.TrimSpace(qualifiedFieldName) == "" {
		return
	}
	ensurePermissionReferencedObject(org, objectName)
	resolvedObjectName, ok := storage.ResolveObjectName(*org, objectName)
	if !ok {
		return
	}
	state := org.Objects[resolvedObjectName]
	if state.Definition.Fields == nil {
		state.Definition.Fields = make(map[string]storage.Field)
	}
	fieldName := permissionFieldLocalName(qualifiedFieldName)
	if fieldName == "" {
		return
	}
	if _, ok := storage.ResolveFieldName(state.Definition, org.Namespace, fieldName); ok {
		return
	}
	state.Definition.Fields[fieldName] = inferredReferencedField(fieldName)
	org.Objects[resolvedObjectName] = state
}

func permissionFieldLocalName(fieldName string) string {
	if idx := strings.IndexByte(fieldName, '.'); idx >= 0 && idx < len(fieldName)-1 {
		return strings.TrimSpace(fieldName[idx+1:])
	}
	return strings.TrimSpace(fieldName)
}

func metadataNameFromPath(path string, suffixes ...string) string {
	name := filepath.Base(path)
	lower := strings.ToLower(name)
	for _, suffix := range suffixes {
		if strings.HasSuffix(lower, suffix) {
			return strings.TrimSpace(name[:len(name)-len(suffix)])
		}
	}
	return ""
}

func recordFieldExists(state storage.ObjectState, fieldName, value string) bool {
	_, ok := recordFieldID(state, fieldName, value)
	return ok
}

func recordFieldID(state storage.ObjectState, fieldName, value string) (storage.ID, bool) {
	for _, record := range state.Records {
		if strings.EqualFold(record.Fields[fieldName].String, value) {
			return record.ID, true
		}
	}
	return "", false
}

func objectPermissionRecordKeys(state storage.ObjectState) map[string]bool {
	keys := make(map[string]bool, len(state.Records))
	for _, record := range state.Records {
		parent, hasParent := record.GetField("ParentId")
		objectValue, hasObject := record.GetField("SObjectType")
		if hasParent && hasObject {
			key := objectPermissionRecordKey(storageRecordKeyText(parent), storageRecordKeyText(objectValue))
			if key != "" {
				keys[key] = true
			}
		}
	}
	return keys
}

func objectPermissionRecordKey(parentID, objectName string) string {
	parentID = strings.ToLower(strings.TrimSpace(parentID))
	objectName = strings.ToLower(strings.TrimSpace(objectName))
	if parentID == "" || objectName == "" {
		return ""
	}
	return parentID + "\x00" + objectName
}

func fieldPermissionRecordExists(state storage.ObjectState, parentID, objectName, fieldName string) bool {
	return fieldPermissionRecordKeys(state)[fieldPermissionRecordKey(parentID, objectName, fieldName)]
}

func fieldPermissionRecordKeys(state storage.ObjectState) map[string]bool {
	keys := make(map[string]bool, len(state.Records))
	for _, record := range state.Records {
		parent, hasParent := record.GetField("ParentId")
		objectValue, hasObject := record.GetField("SObjectType")
		fieldValue, hasField := record.GetField("Field")
		if hasParent && hasObject && hasField {
			key := fieldPermissionRecordKey(storageRecordKeyText(parent), storageRecordKeyText(objectValue), storageRecordKeyText(fieldValue))
			if key != "" {
				keys[key] = true
			}
		}
	}
	return keys
}

func fieldPermissionRecordKey(parentID, objectName, fieldName string) string {
	parentID = strings.ToLower(strings.TrimSpace(parentID))
	objectName = strings.ToLower(strings.TrimSpace(objectName))
	fieldName = strings.ToLower(strings.TrimSpace(fieldName))
	if parentID == "" || objectName == "" || fieldName == "" {
		return ""
	}
	return parentID + "\x00" + objectName + "\x00" + fieldName
}

func storageRecordKeyText(value storage.Value) string {
	switch value.Kind {
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueString:
		return value.String
	default:
		return ""
	}
}

func fieldPermissionObjectName(fieldName string) string {
	if idx := strings.IndexByte(fieldName, '.'); idx > 0 {
		return fieldName[:idx]
	}
	return ""
}

func storageIDValueEqualsText(value storage.Value, text string) bool {
	switch value.Kind {
	case storage.ValueID:
		return strings.EqualFold(string(value.ID), text)
	case storage.ValueString:
		return strings.EqualFold(value.String, text)
	default:
		return false
	}
}

func storageStringValueEqualsText(value storage.Value, text string) bool {
	switch value.Kind {
	case storage.ValueString:
		return strings.EqualFold(value.String, text)
	case storage.ValueID:
		return strings.EqualFold(string(value.ID), text)
	default:
		return false
	}
}

func normalizeOrgKeyPrefixes(org *storage.OrgState) {
	storage.EnsureUniqueKeyPrefixes(org)
}

func applyApexClassRecords(org *storage.OrgState, index typesys.Index, caches ...*sourceCache) {
	if org == nil {
		return
	}
	definition := storage.ObjectDefinition{
		APIName:   "ApexClass",
		Label:     "Apex Class",
		KeyPrefix: "01p",
		Fields: map[string]storage.Field{
			"Id":              {APIName: "Id", Label: "Record ID", Type: storage.FieldID},
			"Name":            {APIName: "Name", Label: "Class Name", Type: storage.FieldString},
			"NamespacePrefix": {APIName: "NamespacePrefix", Label: "Namespace Prefix", Type: storage.FieldString},
			"Body":            {APIName: "Body", Label: "Body", Type: storage.FieldString},
		},
	}
	storage.EnsureStandardObjectFields(&definition)
	state := org.Objects["ApexClass"]
	state.Definition = definition
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	generator := storage.NewIDGenerator(map[string]string{"ApexClass": "01p"})
	sources := sourceCacheFor(caches)
	for _, typ := range index.Types {
		if !typ.HasSourceSnapshot() || typ.Kind != apexast.DeclarationClass || strings.Contains(typ.Name, ".") {
			continue
		}
		id, err := generator.Next("ApexClass")
		if err != nil {
			continue
		}
		source := ""
		if typ.File != "" {
			if data, err := sources.read(typ.File); err == nil {
				source = data
			}
		}
		if apexClassBodyHiddenFromProject(index.Project.Namespace, typ) {
			source = ""
		}
		state.Records[id] = storage.Record{
			ID:     id,
			Object: "ApexClass",
			Fields: map[string]storage.Value{
				"Name":                  {Kind: storage.ValueString, String: typ.Name},
				"NamespacePrefix":       {Kind: storage.ValueString, String: typ.Namespace},
				"Body":                  {Kind: storage.ValueString, String: source},
				"ApiVersion":            storage.DecimalValue("65.0"),
				"LengthWithoutComments": storage.IntegerValue(int64(len(source))),
			},
		}
	}
	org.Objects["ApexClass"] = state
}

func apexClassBodyHiddenFromProject(projectNamespace string, typ typesys.TypeSymbol) bool {
	if !typ.Dependency {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(projectNamespace), strings.TrimSpace(typ.Namespace))
}

type customApplicationMetadata struct {
	Label string `xml:"label"`
}

func applyCustomApplicationRecords(org *storage.OrgState, applicationFiles []string) {
	if org == nil || len(applicationFiles) == 0 {
		return
	}
	storage.EnsureStandardObject(org, "CustomApplication")
	storage.EnsureStandardObject(org, "AppMenuItem")
	apps := org.Objects["CustomApplication"]
	if apps.Records == nil {
		apps.Records = make(map[storage.ID]storage.Record)
	}
	menu := org.Objects["AppMenuItem"]
	if menu.Records == nil {
		menu.Records = make(map[storage.ID]storage.Record)
	}
	paths := append([]string(nil), applicationFiles...)
	sort.Strings(paths)
	appSeq := len(apps.Records) + 1
	menuSeq := len(menu.Records) + 1
	for _, path := range paths {
		developerName := applicationDeveloperName(path)
		if developerName == "" || customApplicationExists(apps, developerName) {
			continue
		}
		label := developerName
		if data, err := os.ReadFile(path); err == nil {
			var meta customApplicationMetadata
			if xml.Unmarshal(data, &meta) == nil && strings.TrimSpace(meta.Label) != "" {
				label = strings.TrimSpace(meta.Label)
			}
		}
		appID := storage.ID(fmt.Sprintf("02u%012d", appSeq))
		menuID := storage.ID(fmt.Sprintf("0DS%012d", menuSeq))
		appSeq++
		menuSeq++
		apps.Records[appID] = storage.Record{
			ID:     appID,
			Object: "CustomApplication",
			Fields: map[string]storage.Value{
				"DeveloperName": storage.StringValue(developerName),
				"Label":         storage.StringValue(label),
				"Name":          storage.StringValue(developerName),
			},
		}
		menu.Records[menuID] = storage.Record{
			ID:     menuID,
			Object: "AppMenuItem",
			Fields: map[string]storage.Value{
				"ApplicationId": storage.IDValue(appID),
				"Label":         storage.StringValue(label),
				"Name":          storage.StringValue(developerName),
				"Type":          storage.StringValue("TabSet"),
				"SortOrder":     storage.IntegerValue(int64(menuSeq - 1)),
			},
		}
	}
	org.Objects["CustomApplication"] = apps
	org.Objects["AppMenuItem"] = menu
}

func applicationDeveloperName(path string) string {
	name := filepath.Base(path)
	for _, suffix := range []string{".app-meta.xml", ".app"} {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix)
		}
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func customApplicationExists(state storage.ObjectState, developerName string) bool {
	for _, record := range state.Records {
		if value, ok := record.GetField("DeveloperName"); ok && value.Kind == storage.ValueString && strings.EqualFold(value.String, developerName) {
			return true
		}
		if value, ok := record.GetField("Name"); ok && value.Kind == storage.ValueString && strings.EqualFold(value.String, developerName) {
			return true
		}
	}
	return false
}

type customNotificationTypeMetadata struct {
	Name        string `xml:"customNotifTypeName"`
	MasterLabel string `xml:"masterLabel"`
}

func applyCustomNotificationTypeRecords(org *storage.OrgState, root string) {
	if org == nil || strings.TrimSpace(root) == "" {
		return
	}
	var files []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() {
			return nil
		}
		lower := strings.ToLower(filepath.ToSlash(path))
		if strings.HasSuffix(lower, ".notiftype-meta.xml") || strings.HasSuffix(lower, ".notiftype") {
			files = append(files, path)
		}
		return nil
	})
	if len(files) == 0 {
		return
	}
	storage.EnsureStandardObject(org, "CustomNotificationType")
	state := org.Objects["CustomNotificationType"]
	if state.Definition.Fields == nil {
		state.Definition.Fields = make(map[string]storage.Field)
	}
	for _, field := range []string{"DeveloperName", "MasterLabel"} {
		if _, ok := state.Definition.Fields[field]; !ok {
			state.Definition.Fields[field] = storage.Field{APIName: field, Type: storage.FieldString, Filterable: storage.BoolFlag(true)}
		}
	}
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	sort.Strings(files)
	seq := len(state.Records) + 1
	for _, path := range files {
		developerName := notificationTypeDeveloperName(path)
		label := developerName
		if data, err := os.ReadFile(path); err == nil {
			var meta customNotificationTypeMetadata
			if xml.Unmarshal(data, &meta) == nil {
				if strings.TrimSpace(meta.MasterLabel) != "" {
					label = strings.TrimSpace(meta.MasterLabel)
				} else if strings.TrimSpace(meta.Name) != "" {
					label = strings.TrimSpace(meta.Name)
				}
			}
		}
		if developerName == "" || customNotificationTypeExists(state, developerName) {
			continue
		}
		id := storage.ID(fmt.Sprintf("0ML%012d", seq))
		seq++
		state.Records[id] = storage.Record{
			ID:     id,
			Object: "CustomNotificationType",
			Fields: map[string]storage.Value{
				"DeveloperName": storage.StringValue(developerName),
				"MasterLabel":   storage.StringValue(label),
			},
		}
	}
	org.Objects["CustomNotificationType"] = state
}

func notificationTypeDeveloperName(path string) string {
	name := filepath.Base(path)
	for _, suffix := range []string{".notiftype-meta.xml", ".notiftype"} {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix)
		}
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func customNotificationTypeExists(state storage.ObjectState, developerName string) bool {
	for _, record := range state.Records {
		if value, ok := record.GetField("DeveloperName"); ok && value.Kind == storage.ValueString && strings.EqualFold(value.String, developerName) {
			return true
		}
	}
	return false
}

func schemaFromIndex(index typesys.Index) schema.Schema {
	return schema.Schema{Objects: append([]schema.Object(nil), index.Objects...)}
}

func triggerEventParts(event string) (string, string) {
	event = strings.ToLower(strings.ReplaceAll(event, " ", ""))
	for _, timing := range []string{"before", "after"} {
		if strings.HasPrefix(event, timing) {
			return timing, strings.TrimPrefix(event, timing)
		}
	}
	return "", ""
}

func compilePropertyAccessor(className, file string, member typesys.MemberSymbol, accessor apexast.Accessor, source string) (vm.Method, error) {
	body, err := extractMethodBody(source, accessor.Range)
	if err != nil {
		return vm.Method{}, err
	}
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return vm.Method{}, err
	}
	method := vm.Method{
		Name:       className + "." + member.Name + "." + accessor.Kind,
		ReturnType: "void",
		Program:    program,
		ClassName:  className,
		IsStatic:   hasModifier(member.Modifiers, "static"),
		Access:     accessModifier(accessor.Modifiers),
		Modifiers:  accessor.Modifiers,
		File:       file,
		Line:       accessor.Range.Start.Line,
		Column:     accessor.Range.Start.Column,
	}
	if accessor.Kind == "get" {
		method.ReturnType = member.Type
	}
	if accessor.Kind == "set" {
		method.Params = []vm.Param{{Name: "value", Type: member.Type}}
	}
	return method, nil
}

func extractMethodSource(source string, r diagnostic.Range) (string, error) {
	text, err := extractSourceRange(source, r)
	if err != nil {
		return "", err
	}
	if open := strings.IndexByte(text, '('); open >= 0 && findMatchingParen(text, open) >= 0 {
		return text, nil
	}
	start, ok := sourceBytePosition(source, r.Start)
	if !ok {
		return "", fmt.Errorf("source range is unavailable")
	}
	lineStart := strings.LastIndexAny(source[:start], "\r\n")
	if lineStart < 0 {
		lineStart = 0
	} else {
		lineStart++
	}
	lineEnd := strings.IndexByte(source[start:], '{')
	if lineEnd < 0 {
		lineEnd = strings.IndexAny(source[start:], "\r\n")
	}
	if lineEnd < 0 {
		lineEnd = len(source) - start
	}
	return source[lineStart : start+lineEnd], nil
}

func extractSourceRange(source string, r diagnostic.Range) (string, error) {
	start, startOK := sourceBytePosition(source, r.Start)
	end, endOK := sourceBytePosition(source, r.End)
	if !startOK || !endOK {
		return "", fmt.Errorf("source range is unavailable")
	}
	if start < 0 || start >= len(source) || end <= start || end > len(source) {
		return "", fmt.Errorf("source range is unavailable")
	}
	return source[start:end], nil
}

func typeDeclarationSource(source string, r diagnostic.Range) (string, error) {
	text, err := extractSourceRange(source, r)
	if err != nil {
		return "", err
	}
	if typeHeaderHasDeclarationKeyword(typeHeader(text)) {
		return text, nil
	}
	start, startOK := sourceBytePosition(source, r.Start)
	end, endOK := sourceBytePosition(source, r.End)
	if !startOK || !endOK || start < 0 || start >= len(source) || end <= start || end > len(source) {
		return text, nil
	}
	lineStart := strings.LastIndexAny(source[:start], "\r\n")
	if lineStart < 0 {
		lineStart = 0
	} else {
		lineStart++
	}
	candidate := source[lineStart:end]
	if typeHeaderHasDeclarationKeyword(typeHeader(candidate)) {
		return candidate, nil
	}
	return text, nil
}

func typeHeaderHasDeclarationKeyword(header string) bool {
	for _, field := range strings.Fields(header) {
		switch strings.ToLower(field) {
		case "class", "interface", "enum":
			return true
		}
	}
	return false
}

func sourceBytePosition(source string, pos diagnostic.Position) (int, bool) {
	if pos.Offset < 0 {
		return 0, false
	}
	if pos.Offset > len(source) {
		return 0, false
	}
	return pos.Offset, true
}

func compileFieldInitializer(typeName, fieldName string, r diagnostic.Range, source string) (vm.Value, bool) {
	const resultName = "gladeFieldInitializerValue"

	expr, ok := fieldInitializerExpr(fieldName, r, source)
	if !ok {
		return vm.Value{}, false
	}
	if expr == "" {
		return vm.Value{}, false
	}
	if !canEvaluateFieldInitializerEagerly(expr) {
		return vm.Value{}, false
	}
	program, err := vm.CompileAnonymous(typeName + " " + resultName + " = " + expr + ";")
	if err != nil {
		return vm.Value{}, false
	}
	machine := vm.New(nil)
	result, err := machine.Execute(program)
	if err != nil {
		return vm.Value{}, false
	}
	value, ok := result.Vars[resultName]
	return value, ok
}

func canEvaluateFieldInitializerEagerly(expr string) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}
	if strings.ContainsAny(expr, "().[]?:+-*/<>") {
		return false
	}
	return true
}

func compileFieldInitializerMethod(className, fieldName string, static bool, file string, r diagnostic.Range, source string) (vm.Method, bool) {
	expr, ok := fieldInitializerExpr(fieldName, r, source)
	if !ok || expr == "" {
		return vm.Method{}, false
	}
	program, err := vm.CompileAnonymous(fieldName + " = " + expr + ";")
	if err != nil {
		return vm.Method{}, false
	}
	name := className + ".<field_init>." + fieldName
	if static {
		name = className + ".<static_field_init>." + fieldName
	}
	return vm.Method{
		Name:       name,
		ReturnType: "void",
		Program:    program,
		ClassName:  className,
		IsStatic:   static,
		File:       file,
		Line:       r.Start.Line,
		Column:     r.Start.Column,
	}, true
}

func fieldInitializerExpr(fieldName string, r diagnostic.Range, source string) (string, bool) {
	fieldSource, err := extractSourceRange(source, r)
	if err != nil {
		return "", false
	}
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(fieldName) + `\b\s*=`)
	if !pattern.MatchString(fieldSource) {
		if stmt, ok := fieldDeclarationStatementSource(source, r); ok {
			fieldSource = stmt
		}
	}
	matches := pattern.FindAllStringIndex(fieldSource, -1)
	if len(matches) == 0 {
		return "", false
	}
	eq := matches[len(matches)-1][1]
	expr := strings.TrimSpace(fieldSource[eq:])
	expr = strings.TrimRight(expr, ";,")
	return strings.TrimSpace(expr), true
}

func fieldDeclarationStatementSource(source string, r diagnostic.Range) (string, bool) {
	start, ok := sourceBytePosition(source, r.Start)
	if !ok || start < 0 || start >= len(source) {
		return "", false
	}
	lineStart := strings.LastIndexAny(source[:start], "\r\n")
	if lineStart < 0 {
		lineStart = 0
	} else {
		lineStart++
	}
	stmtEnd := strings.IndexByte(source[start:], ';')
	if stmtEnd < 0 {
		return "", false
	}
	end := start + stmtEnd + 1
	if end <= lineStart || end > len(source) {
		return "", false
	}
	return source[lineStart:end], true
}

func methodShortName(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

func methodParamKey(params []vm.Param) string {
	var b strings.Builder
	b.WriteString("#")
	for _, param := range params {
		b.WriteString(param.Type)
		b.WriteString(";")
	}
	return b.String()
}

func accessModifier(modifiers []string) string {
	for _, modifier := range modifiers {
		switch strings.ToLower(modifier) {
		case "public", "private", "protected", "global", "webservice":
			return strings.ToLower(modifier)
		}
	}
	return ""
}

func parseExtends(typeSource string) string {
	header := typeHeader(typeSource)
	fields := strings.FieldsFunc(header, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ','
	})
	for i, field := range fields {
		if strings.EqualFold(field, "extends") && i+1 < len(fields) {
			return strings.TrimSpace(fields[i+1])
		}
	}
	return ""
}

func parseImplements(typeSource string) []string {
	header := typeHeader(typeSource)
	i := strings.Index(strings.ToLower(header), "implements")
	if i < 0 {
		return nil
	}
	raw := strings.TrimSpace(header[i+len("implements"):])
	raw = strings.TrimSuffix(raw, "{")
	var out []string
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func parseEnumValues(typeSource string) []string {
	open := strings.IndexByte(typeSource, '{')
	close := strings.LastIndexByte(typeSource, '}')
	if open < 0 || close <= open {
		return nil
	}
	body := typeSource[open+1 : close]
	if semi := strings.IndexByte(body, ';'); semi >= 0 {
		body = body[:semi]
	}
	body = stripApexComments(body)
	var out []string
	for _, part := range strings.Split(body, ",") {
		name := strings.TrimSpace(part)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func typeHeader(typeSource string) string {
	if i := strings.IndexByte(typeSource, '{'); i >= 0 {
		return typeSource[:i]
	}
	return typeSource
}

func parseParams(methodSource string) ([]vm.Param, error) {
	header := methodSource
	if body := strings.IndexByte(methodSource, '{'); body >= 0 {
		header = methodSource[:body]
	}
	open := strings.LastIndexByte(header, '(')
	if open < 0 {
		return nil, fmt.Errorf("method parameter list is unavailable")
	}
	close := findMatchingParen(methodSource, open)
	if close < 0 {
		return nil, fmt.Errorf("method parameter list is incomplete")
	}
	raw := strings.TrimSpace(methodSource[open+1 : close])
	if raw == "" {
		return nil, nil
	}
	raw = stripApexComments(raw)
	parts := splitTopLevelCommas(raw)
	params := make([]vm.Param, 0, len(parts))
	for _, part := range parts {
		paramType, paramName, ok := parseParamTypeAndName(part)
		if !ok {
			return nil, fmt.Errorf("unsupported parameter %q", part)
		}
		params = append(params, vm.Param{
			Type: paramType,
			Name: paramName,
		})
	}
	return params, nil
}

func parseParamTypeAndName(part string) (string, string, bool) {
	part = strings.TrimSpace(part)
	for {
		fields := strings.Fields(part)
		if len(fields) == 0 || !strings.EqualFold(fields[0], "final") {
			break
		}
		part = strings.TrimSpace(strings.TrimPrefix(part, fields[0]))
	}
	end := len(part)
	i := end - 1
	for i >= 0 && isApexIdentifierPart(part[i]) {
		i--
	}
	if i == end-1 {
		return "", "", false
	}
	name := part[i+1:]
	typeName := strings.TrimSpace(part[:i+1])
	if typeName == "" || !isApexIdentifierStart(name[0]) {
		return "", "", false
	}
	return typeName, name, true
}

func isApexIdentifierStart(ch byte) bool {
	return ch == '_' || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

func isApexIdentifierPart(ch byte) bool {
	return isApexIdentifierStart(ch) || (ch >= '0' && ch <= '9')
}

func stripApexComments(source string) string {
	var b strings.Builder
	b.Grow(len(source))
	for i := 0; i < len(source); i++ {
		if source[i] == '\'' {
			end := skipApexString(source, i)
			b.WriteString(source[i : end+1])
			i = end
			continue
		}
		if source[i] == '/' && i+1 < len(source) {
			switch source[i+1] {
			case '/':
				end := skipLineComment(source, i)
				for ; i <= end && i < len(source); i++ {
					if source[i] == '\r' || source[i] == '\n' {
						b.WriteByte(source[i])
					} else {
						b.WriteByte(' ')
					}
				}
				i--
				continue
			case '*':
				end := skipBlockComment(source, i)
				for ; i <= end && i < len(source); i++ {
					if source[i] == '\r' || source[i] == '\n' {
						b.WriteByte(source[i])
					} else {
						b.WriteByte(' ')
					}
				}
				i--
				continue
			}
		}
		b.WriteByte(source[i])
	}
	return b.String()
}

func findMatchingParen(source string, open int) int {
	depth := 0
	for i := open; i < len(source); i++ {
		switch source[i] {
		case '\'':
			i = skipApexString(source, i)
		case '/':
			if i+1 < len(source) && source[i+1] == '/' {
				i = skipLineComment(source, i)
			} else if i+1 < len(source) && source[i+1] == '*' {
				i = skipBlockComment(source, i)
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitTopLevelCommas(raw string) []string {
	var parts []string
	start := 0
	angleDepth := 0
	for i, r := range raw {
		switch r {
		case '<':
			angleDepth++
		case '>':
			if angleDepth > 0 {
				angleDepth--
			}
		case ',':
			if angleDepth == 0 {
				parts = append(parts, raw[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, raw[start:])
	return parts
}

func extractMethodBody(source string, r diagnostic.Range) (string, error) {
	start := r.Start.Offset
	end := r.End.Offset
	if start < 0 || start >= len(source) || end <= start || end > len(source) {
		return "", fmt.Errorf("method source range is unavailable")
	}
	text := source[start:end]
	open := strings.IndexByte(text, '{')
	if open < 0 {
		lineStart := strings.LastIndexAny(source[:start], "\r\n")
		if lineStart < 0 {
			lineStart = 0
		} else {
			lineStart++
		}
		text = source[lineStart:]
		open = strings.IndexByte(text, '{')
		if open < 0 {
			return "", fmt.Errorf("test method has no executable body")
		}
		start = lineStart
	}
	if body, ok := extractMethodBodyFromText(source, start, text, open); ok {
		return body, nil
	}
	text = source[start:]
	if body, ok := extractMethodBodyFromText(source, start, text, open); ok {
		return body, nil
	}
	return "", fmt.Errorf("test method body is incomplete")
}

func extractMethodBodyFromText(source string, start int, text string, open int) (string, bool) {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '\'':
			i = skipApexString(text, i)
		case '/':
			if i+1 < len(text) && text[i+1] == '/' {
				i = skipLineComment(text, i)
			} else if i+1 < len(text) && text[i+1] == '*' {
				i = skipBlockComment(text, i)
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				bodyStart := start + open + 1
				return sourcePositionPrefix(source[:bodyStart]) + text[open+1:i], true
			}
		}
	}
	return "", false
}

func sourcePositionPrefix(source string) string {
	var b strings.Builder
	lastLineStart := 0
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\r':
			b.WriteByte('\r')
			if i+1 < len(source) && source[i+1] == '\n' {
				i++
				b.WriteByte('\n')
			}
			lastLineStart = i + 1
		case '\n':
			b.WriteByte('\n')
			lastLineStart = i + 1
		}
	}
	for i := lastLineStart; i < len(source); i++ {
		switch source[i] {
		case '\r', '\n':
			continue
		default:
			b.WriteByte(' ')
		}
	}
	return b.String()
}

func skipApexString(source string, start int) int {
	for i := start + 1; i < len(source); i++ {
		if source[i] == '\\' && i+1 < len(source) {
			i++
			continue
		}
		if source[i] == '\'' {
			if i+1 < len(source) && source[i+1] == '\'' {
				i++
				continue
			}
			return i
		}
	}
	return len(source) - 1
}

func skipLineComment(source string, start int) int {
	for i := start + 2; i < len(source); i++ {
		if source[i] == '\n' || source[i] == '\r' {
			return i
		}
	}
	return len(source) - 1
}

func skipBlockComment(source string, start int) int {
	for i := start + 2; i+1 < len(source); i++ {
		if source[i] == '*' && source[i+1] == '/' {
			return i + 1
		}
	}
	return len(source) - 1
}

func isTestSetup(modifiers []string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(strings.TrimPrefix(modifier, "@"), "TestSetup") {
			return true
		}
	}
	return false
}

func isSeeAllDataTest(modifiers []string) bool {
	for _, modifier := range modifiers {
		normalized := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(modifier, "@"), " ", ""))
		if strings.HasPrefix(normalized, "istest(") && strings.Contains(normalized, "seealldata=true") {
			return true
		}
	}
	return false
}

func hasModifier(modifiers []string, expected string) bool {
	for _, modifier := range modifiers {
		if strings.EqualFold(strings.TrimPrefix(modifier, "@"), expected) {
			return true
		}
	}
	return false
}

func problem(kind, message string, testCase TestCase) *testreport.Problem {
	return &testreport.Problem{
		Type:    kind,
		Message: message,
		Stack: []testreport.StackFrame{{
			Symbol: testCase.ClassName + "." + testCase.MethodName,
			File:   testCase.File,
			Line:   testCase.Range.Start.Line,
			Column: testCase.Range.Start.Column,
		}},
	}
}

func problemFromError(err error, testCase TestCase) *testreport.Problem {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return problem("Canceled", err.Error(), testCase)
	}
	var runtimeErr *vm.RuntimeError
	if errors.As(err, &runtimeErr) {
		stack := make([]testreport.StackFrame, 0, len(runtimeErr.Stack))
		for _, frame := range runtimeErr.Stack {
			stack = append(stack, testreport.StackFrame{
				Symbol: frame.Symbol,
				File:   frame.File,
				Line:   frame.Line,
				Column: frame.Column,
			})
		}
		if len(stack) == 0 {
			stack = problem("RuntimeError", err.Error(), testCase).Stack
		}
		kind := runtimeErr.Type
		if kind == "" {
			kind = "RuntimeError"
		}
		return &testreport.Problem{
			Type:    kind,
			Message: runtimeErr.Message,
			Stack:   stack,
		}
	}
	return problem("RuntimeError", err.Error(), testCase)
}
