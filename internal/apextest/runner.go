package apextest

import (
	"context"
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
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/profile"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/trace"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

type Options struct {
	Filter              string
	LimitMode           vm.LimitMode
	TraceBlocked        bool
	SlowTestThresholdMS int64
	TimeoutMS           int64
	Parallelism         int
	ParallelMethods     bool
	ClassDurationMS     map[string]int64
	Progress            func(TestProgress)
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
	filter := strings.ToLower(strings.TrimSpace(opts.Filter))
	for _, typ := range index.Types {
		if typ.Dependency {
			continue
		}
		if typ.Kind != apexast.DeclarationClass {
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
			if filter != "" && !strings.Contains(strings.ToLower(testName), filter) {
				continue
			}
			out = append(out, TestCase{
				ClassName:  typ.Name,
				MethodName: member.Name,
				File:       typ.File,
				Range:      member.Range,
				SeeAllData: isSeeAllDataTest(member.Modifiers),
				CostHint:   testCaseCostHint(typ.File),
			})
		}
	}
	return out
}

func Run(index typesys.Index, opts Options) testreport.Run {
	return RunContext(context.Background(), index, opts)
}

func RunContext(ctx context.Context, index typesys.Index, opts Options) testreport.Run {
	return RunCasesContext(ctx, index, opts, nil)
}

func RunCasesContext(ctx context.Context, index typesys.Index, opts Options, cases []TestCase) testreport.Run {
	started := time.Now()
	emitProgress := opts.Progress != nil
	if cases == nil {
		cases = Discover(index, opts)
	}
	sources := newSourceCache()
	if emitProgress {
		reportProgress(opts, TestProgress{Event: "compile_start"})
	}
	runtimeKey, runtime := runtimeFromIndex(index, sources)
	setups, setupErrors, setupInvokePrograms, setupInvokeErrors := compileTestSetupsCached(index, runtimeKey, testCaseClassSet(cases), sources)
	triggerErrors := runtime.TriggerErrors
	testMethods, testMethodErrors, testInvokePrograms, testInvokeErrors := compileTestsCached(index, runtimeKey, cases, sources)
	if emitProgress {
		reportProgress(opts, TestProgress{Event: "compile_done", DurationMS: time.Since(started).Milliseconds(), Status: "pass"})
	}
	org := runtime.Template.CloneRuntimeOrg()
	initializeTestOrg(&org)
	baseMachine := runtime.BaseMachine.CloneRuntime(nil)
	baseMachine.SetTraceEnabled(false)
	baseMachine.EnableTestContext()
	baseRuntimeErr := runtime.BaseErr
	if baseRuntimeErr == nil {
		baseRuntimeErr = registerTestRuntime(baseMachine, append(flattenSetupMethods(setups), methodMapValues(testMethods)...))
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
	if false && noSetupFastPath(setups, setupErrors, setupInvokeErrors) {
		for i := range planned {
			if results[i].Status != "" {
				continue
			}
			planned[i].SetupOrg = org
			planned[i].SetupShared = true
		}
		runTestPlans(ctx, planned, results, baseMachine, baseRuntimeErr, triggerErrors, opts)
		goto assemble
	}
	runTestPlansWithSetups(ctx, order, suiteIndexes, planned, results, baseMachine, baseRuntimeErr, setups, setupErrors, setupInvokePrograms, setupInvokeErrors, triggerErrors, org, opts)
assemble:
	for className, indexes := range suiteIndexes {
		for _, index := range indexes {
			suites[className] = append(suites[className], results[index])
		}
	}

	run := testreport.Run{Name: "glade test"}
	for _, name := range order {
		run.Suites = append(run.Suites, testreport.Suite{Name: name, Cases: suites[name]})
	}
	return run
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
	Methods       map[string]vm.Method
	Classes       []vm.Class
	Triggers      []vm.Trigger
	TriggerErrors []error
	Org           storage.OrgState
	Template      storage.RuntimeTemplate
	PageNames     []string
	BaseMachine   *vm.VM
	BaseErr       error
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
	runtimeCacheMu sync.RWMutex
	runtimeCache   = make(map[runtimeCacheKey]runtimeCacheEntry)
	setupCacheMu   sync.RWMutex
	setupCache     = make(map[string]setupCompileCacheEntry)
	testCacheMu    sync.RWMutex
	testCache      = make(map[string]testCompileCacheEntry)
)

func runtimeKey(index typesys.Index) runtimeCacheKey {
	h := fnv.New128a()
	write := func(s string) {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	seenFiles := make(map[string]bool)
	writeFileBody := func(file string) {
		if file == "" || seenFiles[file] {
			return
		}
		seenFiles[file] = true
		write("file:" + file)
		data, err := os.ReadFile(file)
		if err != nil {
			write("read-error:" + err.Error())
			return
		}
		_, _ = h.Write(data)
		_, _ = h.Write([]byte{0})
	}
	write(index.Project.Root)
	write(index.Project.Namespace)
	write(index.Project.SourceAPIVersion)
	write(fmt.Sprintf("types:%d triggers:%d objects:%d cmdt:%d", len(index.Types), len(index.Triggers), len(index.Objects), len(index.CustomMetadataRecords)))
	for _, typ := range index.Types {
		write(typ.File)
		write(typ.Name)
		write(typ.Namespace)
		writeFileBody(typ.File)
	}
	for _, trig := range index.Triggers {
		write(trig.File)
		write(trig.Name)
		write(trig.ObjectName)
		writeFileBody(trig.File)
	}
	return runtimeCacheKey(hex.EncodeToString(h.Sum(nil)))
}

func cloneRuntimeCacheEntry(in runtimeCacheEntry) runtimeCacheEntry {
	return runtimeCacheEntry{
		Methods:       in.Methods,
		Classes:       in.Classes,
		Triggers:      in.Triggers,
		TriggerErrors: in.TriggerErrors,
		Org:           in.Org,
		Template:      in.Template,
		PageNames:     in.PageNames,
		BaseMachine:   in.BaseMachine,
		BaseErr:       in.BaseErr,
	}
}

func runtimeFromIndex(index typesys.Index, sources sourceCache) (runtimeCacheKey, runtimeCacheEntry) {
	key := runtimeKey(index)
	runtimeCacheMu.RLock()
	if cached, ok := runtimeCache[key]; ok {
		runtimeCacheMu.RUnlock()
		return key, cloneRuntimeCacheEntry(cached)
	}
	runtimeCacheMu.RUnlock()

	methods := compileProjectMethods(index, sources)
	classes := compileProjectClasses(index, methods, sources)
	triggers, triggerErrors := compileProjectTriggers(index, sources)
	org := orgFromIndex(index, sources)
	template := storage.NewRuntimeTemplate(org)
	pageNames := visualforcePageNames(index)
	baseMachine := vm.New(nil)
	baseMachine.SetTraceEnabled(false)
	registerVisualforcePages(baseMachine, pageNames)
	baseErr := registerBaseRuntime(baseMachine, methods, classes, triggers)
	entry := runtimeCacheEntry{
		Methods:       methods,
		Classes:       classes,
		Triggers:      triggers,
		TriggerErrors: triggerErrors,
		Org:           org,
		Template:      template,
		PageNames:     pageNames,
		BaseMachine:   baseMachine,
		BaseErr:       baseErr,
	}
	runtimeCacheMu.Lock()
	runtimeCache[key] = entry
	runtimeCacheMu.Unlock()
	return key, cloneRuntimeCacheEntry(entry)
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

func compileTestSetupsCached(index typesys.Index, baseKey runtimeCacheKey, selectedClasses map[string]bool, sources sourceCache) (map[string][]vm.Method, map[string]error, map[string][]ir.Program, map[string]error) {
	key := string(baseKey) + "|setup|" + classSetKey(selectedClasses)
	setupCacheMu.RLock()
	if cached, ok := setupCache[key]; ok {
		setupCacheMu.RUnlock()
		return cached.Methods, cached.Errors, cached.Programs, cached.ProgramErrs
	}
	setupCacheMu.RUnlock()
	methods, errs, programs, programErrs := compileTestSetupMethodsForClasses(index, selectedClasses, sources)
	setupCacheMu.Lock()
	setupCache[key] = setupCompileCacheEntry{Methods: methods, Errors: errs, Programs: programs, ProgramErrs: programErrs}
	setupCacheMu.Unlock()
	return methods, errs, programs, programErrs
}

func compileTestsCached(index typesys.Index, baseKey runtimeCacheKey, cases []TestCase, sources sourceCache) (map[string]vm.Method, map[string]error, map[string]ir.Program, map[string]error) {
	key := string(baseKey) + "|tests|" + caseSetKey(cases)
	testCacheMu.RLock()
	if cached, ok := testCache[key]; ok {
		testCacheMu.RUnlock()
		return cached.Methods, cached.MethodErrs, cached.Programs, cached.ProgramErrs
	}
	testCacheMu.RUnlock()
	methods, methodErrs := compileTestMethods(cases, sources)
	programs, programErrs := compileTestInvokePrograms(cases)
	testCacheMu.Lock()
	testCache[key] = testCompileCacheEntry{Methods: methods, MethodErrs: methodErrs, Programs: programs, ProgramErrs: programErrs}
	testCacheMu.Unlock()
	return methods, methodErrs, programs, programErrs
}

func prepareTestSetups(ctx context.Context, classNames []string, baseMachine *vm.VM, baseRuntimeErr error, setups map[string][]vm.Method, setupErrors map[string]error, setupInvokePrograms map[string][]ir.Program, setupInvokeErrors map[string]error, triggerErrors []error, org storage.OrgState, opts Options) map[string]testSetupResult {
	results := make(map[string]testSetupResult, len(classNames))
	emitProgress := opts.Progress != nil
	parallelism := opts.Parallelism
	if parallelism <= 1 || len(classNames) <= 1 {
		for _, className := range classNames {
			if emitProgress {
				reportProgress(opts, TestProgress{Event: "setup_start", ClassName: className})
			}
			started := time.Now()
			setupCtx, setupCancel := testContext(ctx, opts.TimeoutMS)
			setupOrg, setupRandom, setupErr, shared := prepareTestSetupOrg(setupCtx, className, baseMachine, baseRuntimeErr, setups[className], setupErrors[className], setupInvokePrograms[className], setupInvokeErrors[className], triggerErrors, org, opts)
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
				started := time.Now()
				setupCtx, setupCancel := testContext(ctx, opts.TimeoutMS)
				setupOrg, setupRandom, setupErr, shared := prepareTestSetupOrg(setupCtx, className, baseMachine, baseRuntimeErr, setups[className], setupErrors[className], setupInvokePrograms[className], setupInvokeErrors[className], triggerErrors, org, opts)
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

func runTestPlans(ctx context.Context, planned []testCasePlan, results []testreport.Case, baseMachine *vm.VM, baseRuntimeErr error, triggerErrors []error, opts Options) {
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
		for i, plan := range planned {
			if results[i].Status != "" {
				continue
			}
			if emitProgress {
				reportProgress(opts, TestProgress{Event: "test_start", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName})
			}
			caseCtx, caseCancel := testContext(ctx, opts.TimeoutMS)
			cloneOrg := len(planned) > 1 || plan.SetupShared
			results[i] = runCase(caseCtx, plan.TestCase, plan.TestMethodErr, plan.InvokeProgram, plan.InvokeProgErr, baseMachine, baseRuntimeErr, plan.SetupErr, triggerErrors, plan.SetupOrg, plan.SetupRandom, opts, cloneOrg, nil)
			if caseCancel != nil {
				caseCancel()
			}
			if emitProgress {
				reportProgress(opts, TestProgress{Event: "test_done", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName, DurationMS: results[i].DurationMS, Status: string(results[i].Status)})
			}
		}
		return
	}
	if opts.ParallelMethods && len(classOrder) == 1 && len(planned) > 1 {
		runSingleClassTestPlans(ctx, planned, results, baseMachine, baseRuntimeErr, triggerErrors, opts, parallelism)
		return
	}
	if parallelism > len(classOrder) {
		parallelism = len(classOrder)
	}
	sortClassRunOrder(classOrder, classIndexes, opts.ClassDurationMS)
	jobs := make(chan string)
	var wg sync.WaitGroup
	for worker := 0; worker < parallelism; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for className := range jobs {
				for _, i := range classIndexes[className] {
					if results[i].Status != "" {
						continue
					}
					plan := planned[i]
					if emitProgress {
						reportProgress(opts, TestProgress{Event: "test_start", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName})
					}
					caseCtx, caseCancel := testContext(ctx, opts.TimeoutMS)
					cloneOrg := len(planned) > 1 || plan.SetupShared
					results[i] = runCase(caseCtx, plan.TestCase, plan.TestMethodErr, plan.InvokeProgram, plan.InvokeProgErr, baseMachine, baseRuntimeErr, plan.SetupErr, triggerErrors, plan.SetupOrg, plan.SetupRandom, opts, cloneOrg, nil)
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
}

func runTestPlansWithSetups(ctx context.Context, classOrder []string, classIndexes map[string][]int, planned []testCasePlan, results []testreport.Case, baseMachine *vm.VM, baseRuntimeErr error, setups map[string][]vm.Method, setupErrors map[string]error, setupInvokePrograms map[string][]ir.Program, setupInvokeErrors map[string]error, triggerErrors []error, org storage.OrgState, opts Options) {
	parallelism := opts.Parallelism
	if parallelism <= 1 {
		parallelism = 1
	}
	emitProgress := opts.Progress != nil
	if parallelism > len(classOrder) {
		parallelism = len(classOrder)
	}
	if parallelism > 1 {
		sortClassRunOrder(classOrder, classIndexes, opts.ClassDurationMS)
	}
	costHints := aggregateClassCostHints(classOrder, planned)
	dispatcher := newClassDispatcher(classOrder, costHints, opts.ClassDurationMS)
	defer dispatcher.close()
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
				started := time.Now()
				setupCtx, setupCancel := testContext(ctx, opts.TimeoutMS)
				setupOrg, setupRandom, setupErr, setupShared := prepareTestSetupOrg(setupCtx, className, baseMachine, baseRuntimeErr, setups[className], setupErrors[className], setupInvokePrograms[className], setupInvokeErrors[className], triggerErrors, org, opts)
				if setupCancel != nil {
					setupCancel()
				}
				if emitProgress {
					reportProgress(opts, TestProgress{Event: "setup_done", ClassName: className, DurationMS: time.Since(started).Milliseconds(), Status: progressStatus(setupErr)})
				}
				if opts.ParallelMethods && len(classIndexes[className]) > 1 {
					methodParallelism := methodParallelismForClassRun(opts.Parallelism, parallelism, len(classIndexes[className]))
					runClassMethodIndexes(ctx, classIndexes[className], planned, results, setupOrg, setupErr, setupRandom, setupShared, baseMachine, baseRuntimeErr, triggerErrors, opts, methodParallelism)
					dispatcher.recordObserved(className, time.Since(classStart).Milliseconds())
					continue
				}
				for _, i := range classIndexes[className] {
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
					caseCtx, caseCancel := testContext(ctx, opts.TimeoutMS)
					cloneOrg := len(classIndexes[className]) > 1 || plan.SetupShared
					if cloneOrg {
						recordCloneFallback()
					}
					results[i] = runCase(caseCtx, plan.TestCase, plan.TestMethodErr, plan.InvokeProgram, plan.InvokeProgErr, baseMachine, baseRuntimeErr, plan.SetupErr, triggerErrors, plan.SetupOrg, plan.SetupRandom, opts, cloneOrg, nil)
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

func methodParallelismForClassRun(totalParallelism, classParallelism, methods int) int {
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
	if parallelism > methods {
		parallelism = methods
	}
	return parallelism
}

func sortClassRunOrder(classOrder []string, classIndexes map[string][]int, classDurationMS map[string]int64) {
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

func runClassMethodIndexes(ctx context.Context, indexes []int, planned []testCasePlan, results []testreport.Case, setupOrg storage.OrgState, setupErr error, setupRandom uint64, setupShared bool, baseMachine *vm.VM, baseRuntimeErr error, triggerErrors []error, opts Options, parallelism int) {
	emitProgress := opts.Progress != nil
	if parallelism <= 1 {
		parallelism = 1
	}
	if parallelism > len(indexes) {
		parallelism = len(indexes)
	}
	jobs := make(chan int)
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
				plan.SetupErr = setupErr
				plan.SetupOrg = setupOrg
				plan.SetupRandom = setupRandom
				plan.SetupShared = setupShared
				if emitProgress {
					reportProgress(opts, TestProgress{Event: "test_start", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName})
				}
				caseCtx, caseCancel := testContext(ctx, opts.TimeoutMS)
				results[i] = runCase(caseCtx, plan.TestCase, plan.TestMethodErr, plan.InvokeProgram, plan.InvokeProgErr, baseMachine, baseRuntimeErr, plan.SetupErr, triggerErrors, plan.SetupOrg, plan.SetupRandom, opts, true, nil)
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

func runSingleClassTestPlans(ctx context.Context, planned []testCasePlan, results []testreport.Case, baseMachine *vm.VM, baseRuntimeErr error, triggerErrors []error, opts Options, parallelism int) {
	emitProgress := opts.Progress != nil
	if parallelism <= 1 {
		parallelism = 1
	}
	if parallelism > len(planned) {
		parallelism = len(planned)
	}
	jobs := make(chan int)
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
				if emitProgress {
					reportProgress(opts, TestProgress{Event: "test_start", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName})
				}
				caseCtx, caseCancel := testContext(ctx, opts.TimeoutMS)
				results[i] = runCase(caseCtx, plan.TestCase, plan.TestMethodErr, plan.InvokeProgram, plan.InvokeProgErr, baseMachine, baseRuntimeErr, plan.SetupErr, triggerErrors, plan.SetupOrg, plan.SetupRandom, opts, len(planned) > 1, nil)
				if caseCancel != nil {
					caseCancel()
				}
				if emitProgress {
					reportProgress(opts, TestProgress{Event: "test_done", ClassName: plan.TestCase.ClassName, MethodName: plan.TestCase.MethodName, DurationMS: results[i].DurationMS, Status: string(results[i].Status)})
				}
			}
		}()
	}
	for i := range planned {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
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

func prepareTestSetupOrg(ctx context.Context, className string, baseMachine *vm.VM, baseRuntimeErr error, setups []vm.Method, setupErr error, setupPrograms []ir.Program, setupProgramErr error, triggerErrors []error, org storage.OrgState, opts Options) (storage.OrgState, uint64, error, bool) {
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
	setupOrg := cloneRuntimeOrgForClass(org, className, "setup")
	machine := baseMachine.CloneRuntime(nil)
	machine.SetTraceEnabled(false)
	if opts.LimitMode != "" {
		machine.SetLimitMode(opts.LimitMode)
	}
	machine.SetOrg(&setupOrg)
	machine.SetContext(ctx)
	machine.EnableTestContext()
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

func runCase(ctx context.Context, testCase TestCase, testMethodErr error, invokeProgram ir.Program, invokeErr error, baseMachine *vm.VM, baseRuntimeErr error, setupErr error, triggerErrors []error, org storage.OrgState, setupRandom uint64, opts Options, cloneOrg bool, journal *storage.IsolationJournal) testreport.Case {
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
		out.DurationMS = time.Since(started).Milliseconds()
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
	machine := baseMachine.CloneRuntime(nil)
	machine.SetDeterministicRandomState(setupRandom)
	machine.SetTraceEnabled(opts.TraceBlocked || opts.SlowTestThresholdMS > 0)
	if opts.LimitMode != "" {
		machine.SetLimitMode(opts.LimitMode)
	}
	machine.SetContext(ctx)
	if cloneOrg {
		org = cloneRuntimeOrgForClass(org, testCase.ClassName, "test")
	}
	if journal != nil && !cloneOrg {
		machine.SetOrg(journal.Org())
	} else {
		machine.SetOrg(&org)
	}
	machine.SetIsolationJournal(journal)
	machine.EnableTestContext()
	machine.SetTestSeeAllData(testCase.SeeAllData)
	machine.ResetLimits()
	result, err := machine.ExecuteInClass(invokeProgram, testCase.ClassName)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			out.Status = testreport.StatusUnsupported
		} else {
			out.Status = testreport.StatusFail
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

func cloneRuntimeOrg(org storage.OrgState) storage.OrgState {
	recordCloneRuntimeOrg("", "")
	return org.CloneRollbackSnapshot()
}

func cloneRuntimeOrgForClass(org storage.OrgState, className, phase string) storage.OrgState {
	recordCloneRuntimeOrg(className, phase)
	return org.CloneRollbackSnapshot()
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
	sources := newSourceCache()
	_, runtime := runtimeFromIndex(index, sources)
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
	sources := newSourceCache()
	_, runtime := runtimeFromIndex(index, sources)
	methods := runtime.Methods
	classes := runtime.Classes
	triggers := runtime.Triggers
	registerVisualforcePages(machine, runtime.PageNames)
	return registerBaseRuntime(machine, methods, classes, triggers)
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

type sourceCache map[string]string

func newSourceCache() sourceCache {
	return make(sourceCache)
}

func sourceCacheFor(caches []sourceCache) sourceCache {
	if len(caches) > 0 && caches[0] != nil {
		return caches[0]
	}
	return newSourceCache()
}

func (cache sourceCache) read(file string) (string, error) {
	if source, ok := cache[file]; ok {
		return source, nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	source := string(data)
	cache[file] = source
	return source, nil
}

func compileProjectClasses(index typesys.Index, methods map[string]vm.Method, caches ...sourceCache) []vm.Class {
	var out []vm.Class
	sources := sourceCacheFor(caches)
	knownTypes := knownTypeNames(index.Types)
	methodsByClass := projectMethodsByClass(methods)
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass && typ.Kind != apexast.DeclarationInterface && typ.Kind != apexast.DeclarationEnum {
			continue
		}
		source, err := sources.read(typ.File)
		if err != nil {
			continue
		}
		class := vm.Class{
			Name:         typ.Name,
			Namespace:    typ.Namespace,
			Access:       accessModifier(typ.Modifiers),
			Modifiers:    append([]string(nil), typ.Modifiers...),
			IsAbstract:   hasModifier(typ.Modifiers, "abstract"),
			IsInterface:  typ.Kind == apexast.DeclarationInterface,
			IsTest:       typ.IsTest,
			Dependency:   typ.Dependency,
			Fields:       make(map[string]vm.Field),
			StaticFields: make(map[string]vm.Field),
			Methods:      make(map[string]vm.Method),
		}
		superClass := typ.SuperClass
		interfaces := append([]string(nil), typ.Interfaces...)
		typeSource, _ := typeDeclarationSource(source, typ.Range)
		if superClass == "" {
			superClass = parseExtends(typeSource)
		}
		if len(interfaces) == 0 {
			interfaces = parseImplements(typeSource)
		}
		class.SuperClass = qualifyNestedTypeName(typ.Name, superClass, knownTypes)
		class.Interfaces = qualifyNestedTypeNames(typ.Name, interfaces, knownTypes)
		if typ.Kind == apexast.DeclarationEnum {
			class.EnumValues = parseEnumValues(typeSource)
		}
		for _, method := range methodsByClass[projectMethodOwnerKey(typ.Name, typ.File)] {
			class.Methods[methodShortName(method.Name)+methodParamKey(method.Params)] = method
		}
		for _, member := range typ.Members {
			switch member.Kind {
			case apexast.DeclarationField, apexast.DeclarationProperty:
				field := vm.Field{
					Name:       member.Name,
					Type:       qualifyNestedTypeNameInType(typ.Name, member.Type, knownTypes),
					Static:     hasModifier(member.Modifiers, "static"),
					Access:     accessModifier(member.Modifiers),
					Modifiers:  append([]string(nil), member.Modifiers...),
					Property:   member.Kind == apexast.DeclarationProperty,
					File:       typ.File,
					Dependency: typ.Dependency,
				}
				if member.Kind == apexast.DeclarationProperty {
					attachPropertyAccessors(&field, typ.Name, typ.File, member, source)
				}
				if value, ok := compileFieldInitializer(member.Type, member.Name, member.Range, source); ok {
					field.Value = value
					field.InitialValue = value
				} else if initializer, ok := compileFieldInitializerMethod(typ.Name, field.Name, field.Static, typ.File, member.Range, source); ok {
					if field.Static {
						class.StaticInitializers = append(class.StaticInitializers, initializer)
					} else {
						class.InstanceInitializers = append(class.InstanceInitializers, initializer)
					}
				}
				if field.Static {
					class.StaticFields[field.Name] = field
					class.StaticFieldOrder = append(class.StaticFieldOrder, field.Name)
				} else {
					class.Fields[field.Name] = field
					class.FieldOrder = append(class.FieldOrder, field.Name)
				}
			case apexast.DeclarationConstructor:
				ctor, err := compileProjectConstructor(typ.Name, typ.File, member.Range, source)
				if err == nil {
					class.Constructors = append(class.Constructors, ctor)
				}
			case apexast.DeclarationInitializer:
				init, err := compileProjectInitializer(typ.Name, typ.File, member.Range, source, hasModifier(member.Modifiers, "static"))
				if err == nil {
					if init.IsStatic {
						class.StaticInitializers = append(class.StaticInitializers, init)
					} else {
						class.InstanceInitializers = append(class.InstanceInitializers, init)
					}
				}
			}
		}
		out = append(out, class)
	}
	out = append(out, passiveStandardRuntimeClasses(index.Types, out)...)
	return out
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
	for _, typ := range typesys.StandardPlatformSymbols() {
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

func compileProjectMethods(index typesys.Index, caches ...sourceCache) map[string]vm.Method {
	type methodCompileJob struct {
		ClassName  string
		Kind       apexast.DeclarationKind
		Member     typesys.MemberSymbol
		File       string
		Source     string
		Dependency bool
	}
	type methodCompileResult struct {
		Key    string
		Method vm.Method
	}
	sources := sourceCacheFor(caches)
	var jobs []methodCompileJob
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass && typ.Kind != apexast.DeclarationInterface {
			continue
		}
		source := ""
		sourceLoaded := false
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod || member.IsTest || isTestSetup(member.Modifiers) {
				continue
			}
			if !sourceLoaded {
				loaded, err := sources.read(typ.File)
				if err != nil {
					continue
				}
				source = loaded
				sourceLoaded = true
			}
			jobs = append(jobs, methodCompileJob{
				ClassName:  typ.Name,
				Kind:       typ.Kind,
				Member:     member,
				File:       typ.File,
				Source:     source,
				Dependency: typ.Dependency,
			})
		}
	}
	results := make([]methodCompileResult, len(jobs))
	compile := func(i int) {
		job := jobs[i]
		member := job.Member
		if job.Kind == apexast.DeclarationInterface {
			method, err := compileProjectMethodSignature(job.ClassName, member.Name, member.Type, append(member.Modifiers, "abstract"), job.File, member.Range, job.Source)
			if err == nil {
				method.Dependency = job.Dependency
				results[i] = methodCompileResult{Key: projectMethodMapKey(method), Method: method}
			}
			return
		}
		method, err := compileProjectMethod(job.ClassName, member.Name, member.Type, member.Modifiers, job.File, member.Range, job.Source)
		if err != nil {
			if unsupported, ok := unsupportedProjectMethod(job.ClassName, member.Name, member.Type, member.Modifiers, job.File, member.Range, job.Source, err); ok {
				unsupported.Dependency = job.Dependency
				results[i] = methodCompileResult{Key: projectMethodMapKey(unsupported), Method: unsupported}
			}
			return
		}
		method.Dependency = job.Dependency
		results[i] = methodCompileResult{Key: projectMethodMapKey(method), Method: method}
	}
	workers := compileWorkers(len(jobs))
	if workers <= 1 {
		for i := range jobs {
			compile(i)
		}
	} else {
		work := make(chan int)
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for index := range work {
					compile(index)
				}
			}()
		}
		for i := range jobs {
			work <- i
		}
		close(work)
		wg.Wait()
	}

	out := make(map[string]vm.Method, len(results))
	for _, result := range results {
		if result.Key != "" {
			out[result.Key] = result.Method
		}
	}
	return out
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

func compileProjectMethodSignature(className, methodName, returnType string, modifiers []string, file string, r diagnostic.Range, source string) (vm.Method, error) {
	methodSource, err := extractMethodSource(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	params, err := parseParams(methodSource)
	if err != nil {
		return vm.Method{}, err
	}
	return vm.Method{
		Name:       className + "." + methodName,
		ReturnType: returnType,
		Params:     params,
		ClassName:  className,
		IsStatic:   hasModifier(modifiers, "static"),
		Access:     accessModifier(modifiers),
		Modifiers:  modifiers,
		File:       file,
		Line:       r.Start.Line,
		Column:     r.Start.Column,
	}, nil
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
		Name:        className + "." + methodName,
		ReturnType:  returnType,
		Params:      params,
		ClassName:   className,
		IsStatic:    hasModifier(modifiers, "static"),
		Access:      accessModifier(modifiers),
		Modifiers:   modifiers,
		File:        file,
		Line:        r.Start.Line,
		Column:      r.Start.Column,
		Unsupported: cause.Error(),
	}, true
}

func compileTestSetupMethods(index typesys.Index, caches ...sourceCache) (map[string][]vm.Method, map[string]error, map[string][]ir.Program, map[string]error) {
	return compileTestSetupMethodsForClasses(index, nil, caches...)
}

func compileTestSetupMethodsForClasses(index typesys.Index, selectedClasses map[string]bool, caches ...sourceCache) (map[string][]vm.Method, map[string]error, map[string][]ir.Program, map[string]error) {
	out := make(map[string][]vm.Method)
	errs := make(map[string]error)
	programs := make(map[string][]ir.Program)
	programErrs := make(map[string]error)
	sources := sourceCacheFor(caches)
	for _, typ := range index.Types {
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

func compileTestMethods(cases []TestCase, caches ...sourceCache) (map[string]vm.Method, map[string]error) {
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
		method, err := compileProjectMethod(testCase.ClassName, testCase.MethodName, "void", []string{"static"}, testCase.File, testCase.Range, source)
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
		program, err := vm.CompileAnonymous(testCase.ClassName + "." + testCase.MethodName + "();")
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

func compileProjectTriggers(index typesys.Index, caches ...sourceCache) ([]vm.Trigger, []error) {
	var out []vm.Trigger
	var errs []error
	sources := sourceCacheFor(caches)
	for _, trigger := range index.Triggers {
		source, err := sources.read(trigger.File)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		body, err := extractMethodBody(source, trigger.Range)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		program, err := vm.CompileAnonymous(body)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, event := range trigger.Events {
			timing, op := triggerEventParts(event)
			if timing == "" || op == "" {
				continue
			}
			out = append(out, vm.Trigger{
				Name:      trigger.Name,
				Namespace: trigger.Namespace,
				Object:    trigger.ObjectName,
				Timing:    timing,
				Operation: op,
				Program:   program,
				File:      trigger.File,
				Line:      trigger.Range.Start.Line,
				Column:    trigger.Range.Start.Column,
			})
		}
	}
	return out, errs
}

func orgFromIndex(index typesys.Index, caches ...sourceCache) storage.OrgState {
	org := storage.NewOrgState()
	org.Namespace = index.Project.Namespace
	registry := sobject.BuildDescribeRegistry(schemaFromIndex(index))
	for name, describe := range registry.Objects {
		org.Objects[name] = storage.ObjectState{
			Definition: sobject.ToObjectDefinition(describe),
			Records:    make(map[storage.ID]storage.Record),
		}
	}
	for _, objectName := range storage.KnownStandardObjectNames() {
		storage.EnsureStandardObject(&org, objectName)
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
			applyProjectReferencedRecordTypes(&org, p)
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
		applyProjectProfileRecordTypeDefaults(&org, *loadedProject)
		applyProjectProfileRecords(&org, *loadedProject)
		applyProjectPermissionSetRecords(&org, *loadedProject)
		storage.EnsureDeterministicPlatformData(&org)
		applyProjectPermissionSetGroupRecords(&org, *loadedProject)
	}
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
var apexTypedVariablePattern = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_]*)\b`)
var apexVariableFieldReferencePattern = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z][A-Za-z0-9_]*)\b`)
var apexVariableFieldBooleanRightLiteralPattern = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z][A-Za-z0-9_]*)\s*(?:==|!=|=)\s*(?i:\btrue\b|\bfalse\b)`)
var apexVariableFieldBooleanLeftLiteralPattern = regexp.MustCompile(`(?i:\btrue\b|\bfalse\b)\s*(?:==|!=)\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z][A-Za-z0-9_]*)\b`)

var projectReferencedStandardFieldCache sync.Map

func applyProjectReferencedStandardFields(org *storage.OrgState, index typesys.Index, caches ...sourceCache) {
	if org == nil {
		return
	}
	cacheKey := ""
	if index.Project.Root != "" {
		cacheKey = index.Project.Root + "|" + fmt.Sprint(len(index.Types))
		if cached, ok := projectReferencedStandardFieldCache.Load(cacheKey); ok {
			applyReferencedStandardFieldSet(org, cached.(map[string]map[string]storage.Field))
			return
		}
	}
	inferred := make(map[string]map[string]storage.Field)
	hints := make(map[string]map[string]storage.FieldType)
	cache := sourceCacheFor(caches)
	seenFiles := make(map[string]bool)
	for _, typ := range index.Types {
		if typ.File == "" || typ.Dependency || seenFiles[typ.File] {
			continue
		}
		seenFiles[typ.File] = true
		source, err := cache.read(typ.File)
		if err != nil {
			continue
		}
		scanSource := apexReferenceScanSource(source)
		varTypes := make(map[string]string)
		for _, match := range apexTypedVariablePattern.FindAllStringSubmatch(scanSource, -1) {
			if len(match) != 3 {
				continue
			}
			if _, ok := org.Objects[match[1]]; ok {
				varTypes[match[2]] = match[1]
			}
		}
		for _, match := range apexVariableFieldBooleanRightLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1) {
			if len(match) != 6 {
				continue
			}
			objectName, ok := varTypes[scanSource[match[2]:match[3]]]
			if !ok {
				continue
			}
			recordProjectReferencedStandardFieldHint(hints, objectName, scanSource[match[4]:match[5]], storage.FieldBoolean)
		}
		for _, match := range apexVariableFieldBooleanLeftLiteralPattern.FindAllStringSubmatchIndex(scanSource, -1) {
			if len(match) != 6 {
				continue
			}
			objectName, ok := varTypes[scanSource[match[2]:match[3]]]
			if !ok {
				continue
			}
			recordProjectReferencedStandardFieldHint(hints, objectName, scanSource[match[4]:match[5]], storage.FieldBoolean)
		}
		for _, match := range apexSchemaSObjectTypeFieldReferencePattern.FindAllStringSubmatchIndex(scanSource, -1) {
			if len(match) != 6 {
				continue
			}
			objectName := scanSource[match[2]:match[3]]
			fieldName := scanSource[match[4]:match[5]]
			recordProjectReferencedStandardField(org, inferred, objectName, fieldName, projectReferencedStandardFieldHint(hints, objectName, fieldName))
		}
		for _, match := range apexSObjectTypeFieldReferencePattern.FindAllStringSubmatchIndex(scanSource, -1) {
			if len(match) != 6 {
				continue
			}
			objectName := scanSource[match[2]:match[3]]
			fieldName := scanSource[match[4]:match[5]]
			recordProjectReferencedStandardField(org, inferred, objectName, fieldName, projectReferencedStandardFieldHint(hints, objectName, fieldName))
		}
		recordProjectReferencedSObjectLiteralFields(org, inferred, hints, scanSource, apexNewSObjectLiteralPattern)
		recordProjectReferencedSObjectLiteralFields(org, inferred, hints, scanSource, apexSObjectLiteralPattern)
		for _, match := range apexStaticFieldReferencePattern.FindAllStringSubmatchIndex(scanSource, -1) {
			if len(match) != 6 || apexMemberReferenceIsCall(scanSource, match[1]) {
				continue
			}
			objectName := scanSource[match[2]:match[3]]
			fieldName := scanSource[match[4]:match[5]]
			recordProjectReferencedStandardField(org, inferred, objectName, fieldName, projectReferencedStandardFieldHint(hints, objectName, fieldName))
		}
		for _, match := range apexVariableFieldReferencePattern.FindAllStringSubmatchIndex(scanSource, -1) {
			if len(match) != 6 || apexMemberReferenceIsCall(scanSource, match[1]) {
				continue
			}
			objectName, ok := varTypes[scanSource[match[2]:match[3]]]
			if !ok {
				continue
			}
			fieldName := scanSource[match[4]:match[5]]
			recordProjectReferencedStandardField(org, inferred, objectName, fieldName, projectReferencedStandardFieldHint(hints, objectName, fieldName))
		}
	}
	if cacheKey != "" {
		projectReferencedStandardFieldCache.Store(cacheKey, inferred)
	}
	applyReferencedStandardFieldSet(org, inferred)
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

func apexNamedArgumentBoolLiteral(rest string) bool {
	rest = strings.TrimSpace(rest)
	lower := strings.ToLower(rest)
	return strings.HasPrefix(lower, "true") || strings.HasPrefix(lower, "false")
}

func apexNamedArgumentNumericLiteral(rest string) bool {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return false
	}
	if rest[0] == '-' || rest[0] == '+' {
		rest = strings.TrimSpace(rest[1:])
	}
	if rest == "" || rest[0] < '0' || rest[0] > '9' {
		return false
	}
	for i := 1; i < len(rest); i++ {
		if rest[i] >= '0' && rest[i] <= '9' {
			continue
		}
		if rest[i] == '.' && i+1 < len(rest) && rest[i+1] >= '0' && rest[i+1] <= '9' {
			continue
		}
		return rest[i] == ',' || rest[i] == ')' || rest[i] == ' ' || rest[i] == '\t' || rest[i] == '\r' || rest[i] == '\n'
	}
	return true
}

func recordProjectReferencedSObjectLiteralFields(org *storage.OrgState, inferred map[string]map[string]storage.Field, hints map[string]map[string]storage.FieldType, scanSource string, pattern *regexp.Regexp) {
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
			hintedType := projectReferencedStandardFieldHint(hints, objectName, fieldName)
			if hintedType == "" && apexNamedArgumentBoolLiteral(body[argMatch[1]:]) {
				hintedType = storage.FieldBoolean
			}
			if hintedType == "" && apexNamedArgumentNumericLiteral(body[argMatch[1]:]) {
				hintedType = storage.FieldDecimal
			}
			recordProjectReferencedStandardField(org, inferred, objectName, fieldName, hintedType)
		}
	}
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

func recordProjectReferencedStandardFieldHint(hints map[string]map[string]storage.FieldType, objectName, fieldName string, fieldType storage.FieldType) {
	if hints[objectName] == nil {
		hints[objectName] = make(map[string]storage.FieldType)
	}
	hints[objectName][fieldName] = fieldType
}

func projectReferencedStandardFieldHint(hints map[string]map[string]storage.FieldType, objectName, fieldName string) storage.FieldType {
	if hints[objectName] == nil {
		return ""
	}
	return hints[objectName][fieldName]
}

func recordProjectReferencedStandardField(org *storage.OrgState, inferred map[string]map[string]storage.Field, objectName, fieldName string, hintedType storage.FieldType) {
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
	if existingName, ok := storage.ResolveFieldName(state.Definition, org.Namespace, fieldName); ok {
		if hintedType != "" {
			existing := state.Definition.Fields[existingName]
			replacement := inferredReferencedStandardField(existing.APIName, hintedType)
			if projectReferencedFieldCanUpgrade(existing, replacement) {
				state.Definition.Fields[existingName] = replacement
				org.Objects[objectName] = state
			}
		}
		return
	}
	if parentRelationshipKnown(state.Definition, fieldName) {
		return
	}
	if projectReferencedNameIsChildRelationship(*org, objectName, fieldName) {
		return
	}
	if inferred[objectName] == nil {
		inferred[objectName] = make(map[string]storage.Field)
	}
	field := inferredReferencedField(*org, objectName, fieldName, hintedType)
	if existingName, existing, ok := projectReferencedInferredField(inferred[objectName], fieldName); ok {
		if existing.Type != storage.FieldString && field.Type == storage.FieldString {
			return
		}
		if existing.Type == storage.FieldString && field.Type != storage.FieldString {
			delete(inferred[objectName], existingName)
			inferred[objectName][fieldName] = field
		}
		return
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

func applyReferencedStandardFieldSet(org *storage.OrgState, fields map[string]map[string]storage.Field) {
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
			if projectReferencedNameIsChildRelationship(*org, objectName, fieldName) {
				continue
			}
			state.Definition.Fields[fieldName] = field
			if field.Type == storage.FieldReference && field.RelationshipName != "" && len(field.ReferenceTo) > 0 && !parentRelationshipKnown(state.Definition, field.RelationshipName) {
				state.Definition.Relations = append(state.Definition.Relations, storage.Relationship{
					Field:              field.APIName,
					ParentObjects:      append([]string(nil), field.ReferenceTo...),
					ParentRelationship: field.RelationshipName,
					Polymorphic:        len(field.ReferenceTo) > 1,
				})
			}
		}
		org.Objects[objectName] = state
	}
}

func projectReferencedNameIsChildRelationship(org storage.OrgState, objectName, name string) bool {
	parentName, ok := storage.ResolveObjectName(org, objectName)
	if !ok {
		parentName = objectName
	}
	for _, child := range org.Objects {
		for _, relation := range child.Definition.Relations {
			if relation.ChildRelationship == "" || !strings.EqualFold(relation.ChildRelationship, name) {
				continue
			}
			for _, parent := range relation.ParentObjects {
				resolvedParent, ok := storage.ResolveObjectName(org, parent)
				if !ok {
					resolvedParent = parent
				}
				if strings.EqualFold(resolvedParent, parentName) {
					return true
				}
			}
		}
	}
	return false
}

func inferredReferencedField(org storage.OrgState, objectName, fieldName string, hintedType storage.FieldType) storage.Field {
	field := inferredReferencedStandardField(fieldName, hintedType)
	if field.Type == storage.FieldBoolean {
		return field
	}
	if target := inferredLookupTarget(org, objectName, fieldName); target != "" {
		field.Type = storage.FieldReference
		field.DisplayType = "REFERENCE"
		field.ReferenceTo = []string{target}
		field.RelationshipName = inferredLookupRelationshipName(fieldName)
		field.ChildRelationshipName = inferredLookupChildRelationshipName(objectName, fieldName, target)
	}
	return field
}

func inferredReferencedStandardField(fieldName string, hintedType storage.FieldType) storage.Field {
	field := storage.Field{APIName: fieldName, Label: fieldName, Type: storage.FieldString, DisplayType: "STRING"}
	if hintedType == storage.FieldBoolean {
		field.Type = storage.FieldBoolean
		field.DisplayType = "BOOLEAN"
		field.DefaultValue = "false"
		return field
	}
	if hintedType == storage.FieldDecimal {
		field.Type = storage.FieldDecimal
		field.DisplayType = "DOUBLE"
		field.DefaultValue = "0"
		return field
	}
	switch {
	case strings.HasSuffix(fieldName, "Id"):
		field.Type = storage.FieldReference
		field.DisplayType = "REFERENCE"
		field.RelationshipName = strings.TrimSuffix(fieldName, "Id")
	case inferredBooleanFieldName(fieldName):
		field.Type = storage.FieldBoolean
		field.DisplayType = "BOOLEAN"
		field.DefaultValue = "false"
	case strings.Contains(fieldName, "Date"):
		field.Type = storage.FieldDate
		field.DisplayType = "DATE"
	case inferredNumericFieldName(fieldName), strings.Contains(fieldName, "Amount"), strings.Contains(fieldName, "Balance"), strings.Contains(fieldName, "Mrr"),
		strings.Contains(fieldName, "Price"), strings.Contains(fieldName, "Quantity"),
		strings.Contains(fieldName, "Shipping"), strings.Contains(fieldName, "Tax"), strings.Contains(fieldName, "Total"):
		field.Type = storage.FieldDecimal
		field.DisplayType = "DOUBLE"
		field.DefaultValue = "0"
	}
	return field
}

func inferredBooleanFieldName(fieldName string) bool {
	base := storage.StripAnyNamespaceToken(fieldName)
	lower := strings.ToLower(base)
	for _, suffix := range []string{"__c", "__e", "__mdt"} {
		if strings.HasSuffix(lower, suffix) {
			base = base[:len(base)-len(suffix)]
			lower = lower[:len(lower)-len(suffix)]
			break
		}
	}
	if strings.HasPrefix(base, "Is") || strings.HasPrefix(base, "Has") || strings.HasPrefix(base, "Allow") ||
		strings.HasSuffix(base, "Enabled") || strings.HasSuffix(base, "Disabled") {
		return true
	}
	switch lower {
	case "private", "active", "enabled", "disabled", "default", "primary", "hidden", "trackinventory":
		return true
	default:
		return false
	}
}

func inferredNumericFieldName(fieldName string) bool {
	base := storage.StripAnyNamespaceToken(fieldName)
	lower := strings.ToLower(base)
	for _, suffix := range []string{"__c", "__e", "__mdt"} {
		if strings.HasSuffix(lower, suffix) {
			base = base[:len(base)-len(suffix)]
			lower = lower[:len(lower)-len(suffix)]
			break
		}
	}
	compact := strings.ReplaceAll(lower, "_", "")
	return strings.HasPrefix(compact, "numberof")
}

func inferredLookupTarget(org storage.OrgState, objectName, fieldName string) string {
	if !strings.HasSuffix(strings.ToLower(fieldName), "__c") {
		return ""
	}
	candidates := []string{fieldName}
	if namespace := namespaceFromSchemaName(objectName); namespace != "" {
		candidates = append(candidates, storage.NamespaceTokenName(namespace, fieldName))
	}
	if org.Namespace != "" {
		candidates = append(candidates, storage.NamespaceTokenName(org.Namespace, fieldName))
	}
	for _, candidate := range candidates {
		if resolved, ok := storage.ResolveObjectName(org, candidate); ok {
			return resolved
		}
	}
	base := storage.StripAnyNamespaceToken(fieldName)
	if target := inferredStandardLookupTarget(org, base); target != "" {
		return target
	}
	if strings.HasSuffix(strings.ToLower(base), "__c") {
		candidate := base[:len(base)-len("__c")]
		if resolved, ok := storage.ResolveObjectName(org, candidate); ok {
			return resolved
		}
		numberedCandidate := strings.TrimRight(candidate, "0123456789")
		if numberedCandidate != candidate && numberedCandidate != "" {
			if namespace := namespaceFromSchemaName(objectName); namespace != "" {
				if resolved, ok := storage.ResolveObjectName(org, storage.NamespaceTokenName(namespace, numberedCandidate+"__c")); ok {
					return resolved
				}
			}
			if org.Namespace != "" {
				if resolved, ok := storage.ResolveObjectName(org, storage.NamespaceTokenName(org.Namespace, numberedCandidate+"__c")); ok {
					return resolved
				}
			}
			if resolved, ok := storage.ResolveObjectName(org, numberedCandidate); ok {
				return resolved
			}
		}
		if target := inferredCustomLookupTargetFromFieldSuffix(org, objectName, fieldName); target != "" {
			return target
		}
	}
	return ""
}

func inferredCustomLookupTargetFromFieldSuffix(org storage.OrgState, objectName, fieldName string) string {
	fieldBase := customRelationshipBaseName(fieldName)
	if fieldBase == "" {
		return ""
	}
	fieldNamespace := namespaceFromSchemaName(fieldName)
	objectNamespace := namespaceFromSchemaName(objectName)
	type candidate struct {
		name  string
		score int
		size  int
	}
	var candidates []candidate
	for candidateName := range org.Objects {
		candidateBase := customRelationshipBaseName(candidateName)
		if candidateBase == "" || strings.EqualFold(candidateBase, fieldBase) || !strings.HasSuffix(strings.ToLower(fieldBase), strings.ToLower(candidateBase)) {
			continue
		}
		score := 0
		candidateNamespace := namespaceFromSchemaName(candidateName)
		if candidateNamespace != "" && strings.EqualFold(candidateNamespace, fieldNamespace) {
			score += 2
		}
		if candidateNamespace != "" && strings.EqualFold(candidateNamespace, objectNamespace) {
			score++
		}
		candidates = append(candidates, candidate{name: candidateName, score: score, size: len(candidateBase)})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].size != candidates[j].size {
			return candidates[i].size > candidates[j].size
		}
		return candidates[i].name < candidates[j].name
	})
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0].name
}

func inferredStandardLookupTarget(org storage.OrgState, fieldName string) string {
	base := strings.ToLower(strings.TrimSpace(fieldName))
	if strings.HasSuffix(base, "__c") {
		base = base[:len(base)-len("__c")]
	}
	var target string
	switch base {
	case "account", "account2", "customer", "customeraccount", "buyeraccount":
		target = "Account"
	case "contact", "contact2", "customercontact":
		target = "Contact"
	}
	if target == "" {
		return ""
	}
	if resolved, ok := storage.ResolveObjectName(org, target); ok {
		return resolved
	}
	return target
}

func inferredLookupRelationshipName(fieldName string) string {
	if strings.HasSuffix(strings.ToLower(fieldName), "__c") {
		return fieldName[:len(fieldName)-len("__c")] + "__r"
	}
	return strings.TrimSuffix(fieldName, "Id")
}

func inferredLookupChildRelationshipName(objectName, fieldName, parentObject string) string {
	childBase := customRelationshipBaseName(objectName)
	parentBase := customRelationshipBaseName(parentObject)
	fieldBase := customRelationshipBaseName(fieldName)
	if childBase == "" || parentBase == "" || fieldBase == "" {
		return ""
	}
	suffix := ""
	if len(fieldBase) >= len(parentBase) && strings.EqualFold(fieldBase[:len(parentBase)], parentBase) {
		suffix = fieldBase[len(parentBase):]
	}
	childRelationshipBase := pluralizeCustomRelationshipBase(childBase) + suffix
	if suffix == "" && len(fieldBase) > len(parentBase) && strings.HasSuffix(strings.ToLower(fieldBase), strings.ToLower(parentBase)) {
		childRelationshipBase = pluralizeCustomRelationshipBase(fieldBase)
	}
	childRelationship := childRelationshipBase + "__r"
	if namespace := namespaceFromSchemaName(objectName); namespace != "" {
		return storage.NamespaceTokenName(namespace, childRelationship)
	}
	if namespace := namespaceFromSchemaName(fieldName); namespace != "" {
		return storage.NamespaceTokenName(namespace, childRelationship)
	}
	return childRelationship
}

func customRelationshipBaseName(name string) string {
	base := storage.StripAnyNamespaceToken(strings.TrimSpace(name))
	lower := strings.ToLower(base)
	for _, suffix := range []string{"__c", "__r", "__e", "__mdt"} {
		if strings.HasSuffix(lower, suffix) {
			return base[:len(base)-len(suffix)]
		}
	}
	return ""
}

func pluralizeCustomRelationshipBase(name string) string {
	if strings.HasSuffix(name, "ys") && len(name) > 2 {
		return strings.TrimSuffix(name, "ys") + "ies"
	}
	if strings.HasSuffix(name, "s") {
		return name
	}
	if strings.HasSuffix(name, "y") && len(name) > 1 {
		return strings.TrimSuffix(name, "y") + "ies"
	}
	return name + "s"
}

func namespaceFromSchemaName(name string) string {
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	if first <= 0 || first >= last {
		return ""
	}
	return name[:first]
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
	Value      any
}

var apexRecordTypeInfoCallRE = regexp.MustCompile(`(?is)(?:(?:Schema\s*\.\s*)?SObjectType\s*\.\s*([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)*)(?:\s*\.\s*getDescribe\s*\(\s*\))?|([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)*)\s*\.\s*SObjectType(?:\s*\.\s*getDescribe\s*\(\s*\))?)\s*\.\s*getRecordTypeInfosBy(Name|DeveloperName)\s*\(\s*\)\s*\.\s*get\s*\(\s*['"]([^'"]+)['"]\s*\)`)
var dataRecordTypeQueryRE = regexp.MustCompile(`(?is)FROM\s+RecordType\b[^"\n]*\bSObjectType\s*=\s*'([^']+)'[^"\n]*\bName\s*=\s*'([^']+)'`)
var apexGetSObjectTypeReturnRE = regexp.MustCompile(`(?is)\bgetSObjectType\s*\(\s*\)\s*\{.*?\breturn\s+([A-Za-z_][A-Za-z0-9_]*(?:__[A-Za-z0-9_]+)*)\s*\.\s*SObjectType\s*;`)
var apexStaticFinalStringRE = regexp.MustCompile(`(?is)\bstatic\s+final\s+String\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*['"]([^'"]+)['"]`)
var apexGetRecordTypeIdCallRE = regexp.MustCompile(`(?is)\bgetRecordTypeId\s*\(\s*(?:'([^']+)'|"([^"]+)"|([A-Za-z_][A-Za-z0-9_]*))\s*\)`)
var apexRecordTypeStringMethodRE = regexp.MustCompile(`(?is)\b([A-Za-z_][A-Za-z0-9_]*)\s*\(\s*String\s+([A-Za-z_][A-Za-z0-9_]*)\s*\)\s*\{([^{}]*)\}`)

func applyProjectReferencedRecordTypes(org *storage.OrgState, p project.Project) {
	if org == nil {
		return
	}
	for _, ref := range projectReferencedRecordTypes(p) {
		canonicalObject, ok := storage.ResolveObjectName(*org, ref.ObjectName)
		if !ok {
			continue
		}
		state := org.Objects[canonicalObject]
		if profileRecordTypeExists(state.Definition.RecordTypes, ref.DeveloperName) || profileRecordTypeExists(state.Definition.RecordTypes, ref.Name) {
			continue
		}
		state.Definition.RecordTypes = append(state.Definition.RecordTypes, storage.RecordTypeInfo{
			DeveloperName: ref.DeveloperName,
			Name:          ref.Name,
			Active:        true,
			Available:     true,
		})
		org.Objects[canonicalObject] = state
	}
}

func projectReferencedRecordTypes(p project.Project) []projectRecordTypeReference {
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
	for _, file := range p.ApexFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		source := string(data)
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
			add(ref)
		}
		for _, ref := range projectReferencedTestDataHelperRecordTypes(source) {
			add(ref)
		}
	}
	for _, file := range projectDataJSONFiles(p.Root) {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, match := range dataRecordTypeQueryRE.FindAllStringSubmatch(string(data), -1) {
			name := strings.TrimSpace(match[2])
			add(projectRecordTypeReference{
				ObjectName:    match[1],
				DeveloperName: recordTypeDeveloperNameFromLabel(name),
				Name:          name,
			})
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
	for _, forwardingMethod := range apexRecordTypeForwardingMethods(source) {
		callRE := regexp.MustCompile(`(?is)\b` + regexp.QuoteMeta(forwardingMethod) + `\s*\(\s*(?:'([^']+)'|"([^"]+)"|([A-Za-z_][A-Za-z0-9_]*))\s*\)`)
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

func applyProjectDataRelationshipReferences(org *storage.OrgState, p project.Project) {
	if org == nil {
		return
	}
	for _, ref := range projectDataFieldReferences(p.Root) {
		ensureProjectDataReferencedObjectField(org, ref.ObjectName, ref.FieldName, projectDataFieldHint(ref.Value))
	}
	for _, ref := range projectDataRelationshipReferences(p.Root) {
		fieldName := dataRelationshipLookupFieldName(ref.ParentRelationship)
		if fieldName == "" {
			continue
		}
		parentObject := dataRelationshipParentObjectName(*org, ref.ChildObject, ref.ParentRelationship)
		if parentObject != "" {
			ensurePermissionReferencedObject(org, parentObject)
		}
		ensurePermissionReferencedObjectField(org, ref.ChildObject, fieldName)
	}
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
		refs = append(refs, projectDataFieldReference{ObjectName: objectName, FieldName: fieldName, Value: value})
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
			if isCustomDataObjectKey(key) {
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

func projectDataFieldHint(value any) storage.FieldType {
	switch value.(type) {
	case bool:
		return storage.FieldBoolean
	case float64:
		return storage.FieldDecimal
	default:
		return ""
	}
}

func dataRelationshipFieldIsReference(org storage.OrgState, objectName, fieldName, parentObject string) bool {
	resolvedObject, ok := storage.ResolveObjectName(org, objectName)
	if !ok {
		return false
	}
	state := org.Objects[resolvedObject]
	resolvedField, ok := storage.ResolveFieldName(state.Definition, org.Namespace, fieldName)
	if !ok {
		return false
	}
	field := state.Definition.Fields[resolvedField]
	if field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
		return false
	}
	if parentObject == "" {
		return true
	}
	for _, target := range field.ReferenceTo {
		if strings.EqualFold(target, parentObject) {
			return true
		}
	}
	return false
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
	if strings.HasSuffix(strings.ToLower(name), "__r") {
		return name[:len(name)-len("__r")] + "__c"
	}
	return ""
}

func dataRelationshipParentObjectName(org storage.OrgState, childObject, relationshipName string) string {
	fieldName := dataRelationshipLookupFieldName(relationshipName)
	if fieldName == "" {
		return ""
	}
	base := storage.StripAnyNamespaceToken(fieldName)
	if strings.HasSuffix(strings.ToLower(base), "__c") {
		base = base[:len(base)-len("__c")]
	}
	if base == "" {
		return ""
	}
	if namespace := namespaceFromSchemaName(fieldName); namespace != "" {
		exact := storage.NamespaceTokenName(namespace, base+"__c")
		if resolved, ok := storage.ResolveObjectName(org, exact); ok {
			return resolved
		}
		if numbered := numberedCustomObjectName(org, namespace, base); numbered != "" {
			return numbered
		}
		if target := inferredCustomLookupTargetFromFieldSuffix(org, childObject, fieldName); target != "" {
			return target
		}
		return exact
	}
	if namespace := namespaceFromSchemaName(childObject); namespace != "" {
		exact := storage.NamespaceTokenName(namespace, base+"__c")
		if resolved, ok := storage.ResolveObjectName(org, exact); ok {
			return resolved
		}
		if numbered := numberedCustomObjectName(org, namespace, base); numbered != "" {
			return numbered
		}
		if target := inferredCustomLookupTargetFromFieldSuffix(org, childObject, fieldName); target != "" {
			return target
		}
		return exact
	}
	return base + "__c"
}

func numberedCustomObjectName(org storage.OrgState, namespace, base string) string {
	numberedBase := strings.TrimRight(base, "0123456789")
	if numberedBase == "" || numberedBase == base {
		return ""
	}
	candidate := storage.NamespaceTokenName(namespace, numberedBase+"__c")
	if resolved, ok := storage.ResolveObjectName(org, candidate); ok {
		return resolved
	}
	return candidate
}

func isCustomDataObjectKey(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(lower, "__c") || strings.HasSuffix(lower, "__e") || strings.HasSuffix(lower, "__mdt")
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
	case "node_modules", "vendor", "dist", "bin", "coverage":
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

func applyProjectProfileRecordTypes(org *storage.OrgState, p project.Project) bool {
	if org == nil || len(p.ProfileFiles) == 0 {
		return false
	}
	changed := false
	for _, file := range sortedProfileFilesForDefaults(p.ProfileFiles) {
		for _, visibility := range loadProfileRecordTypeVisibilities(file) {
			canonicalObject, ok := profileRecordTypeObjectName(*org, visibility.ObjectName)
			if !ok {
				continue
			}
			state := org.Objects[canonicalObject]
			if profileRecordTypeExists(state.Definition.RecordTypes, visibility.DeveloperName) {
				continue
			}
			state.Definition.RecordTypes = append(state.Definition.RecordTypes, storage.RecordTypeInfo{
				DeveloperName: visibility.DeveloperName,
				Name:          visibility.Name,
				Active:        true,
				Available:     true,
				Default:       visibility.Default && !visibility.PersonAccount,
			})
			org.Objects[canonicalObject] = state
			changed = true
		}
	}
	return changed
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
	for _, recordType := range recordTypes {
		if strings.EqualFold(recordType.DeveloperName, developerName) {
			return true
		}
		if strings.EqualFold(recordType.Name, developerName) {
			return true
		}
	}
	return false
}

func applyProjectProfileRecordTypeDefaults(org *storage.OrgState, p project.Project) {
	if org == nil || len(p.ProfileFiles) == 0 {
		return
	}
	defaults := projectProfileRecordTypeDefaults(p.ProfileFiles)
	for objectName, developerName := range defaults {
		canonicalObject, ok := profileRecordTypeObjectName(*org, objectName)
		if !ok {
			continue
		}
		state := org.Objects[canonicalObject]
		changed := false
		for i := range state.Definition.RecordTypes {
			recordType := &state.Definition.RecordTypes[i]
			isDefault := strings.EqualFold(recordType.DeveloperName, developerName)
			if recordType.Default != isDefault {
				recordType.Default = isDefault
				changed = true
			}
		}
		if changed {
			org.Objects[canonicalObject] = state
		}
	}
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

func applyProjectProfileRecords(org *storage.OrgState, p project.Project) {
	if org == nil || len(p.ProfileFiles) == 0 {
		return
	}
	state, ok := org.Objects["Profile"]
	if !ok {
		return
	}
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	generator := storage.NewIDGenerator(storage.StandardKeyPrefixes())
	generator.Sequences = org.IDSequences
	for _, file := range p.ProfileFiles {
		name := profileNameFromPath(file)
		if name == "" || profileRecordExists(state, name) {
			continue
		}
		id, err := generator.Next("Profile")
		if err != nil {
			continue
		}
		state.Records[id] = storage.Record{
			ID:     id,
			Object: "Profile",
			Fields: map[string]storage.Value{"Name": storage.StringValue(name)},
		}
	}
	org.IDSequences = generator.Sequences
	org.Objects["Profile"] = state
}

func profileNameFromPath(path string) string {
	name := filepath.Base(path)
	for _, suffix := range []string{".profile-meta.xml", ".profile"} {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
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

func applyProjectPermissionSetRecords(org *storage.OrgState, p project.Project) {
	if org == nil || len(p.PermissionSetFiles) == 0 {
		return
	}
	storage.EnsureStandardObject(org, "PermissionSet")
	state := org.Objects["PermissionSet"]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	generator := storage.NewIDGenerator(storage.StandardKeyPrefixes())
	generator.Sequences = org.IDSequences
	for _, file := range p.PermissionSetFiles {
		name := metadataNameFromPath(file, ".permissionset-meta.xml", ".permissionset")
		if name == "" {
			continue
		}
		id, exists := recordFieldID(state, "Name", name)
		if !exists {
			nextID, err := generator.Next("PermissionSet")
			if err != nil {
				continue
			}
			id = nextID
			label := strings.ReplaceAll(name, "_", " ")
			if metadata, ok := readPermissionSetMetadata(file); ok && strings.TrimSpace(metadata.Label) != "" {
				label = strings.TrimSpace(metadata.Label)
			}
			state.Records[id] = storage.Record{
				ID:     id,
				Object: "PermissionSet",
				Fields: map[string]storage.Value{
					"Name":             storage.StringValue(name),
					"Label":            storage.StringValue(label),
					"Type":             storage.StringValue("Regular"),
					"IsOwnedByProfile": storage.BooleanValue(false),
				},
			}
		}
		if metadata, ok := readPermissionSetMetadata(file); ok {
			customPermissions := permissionSetCustomPermissionValues(metadata)
			if len(customPermissions) > 0 {
				record := state.Records[id]
				if record.Fields == nil {
					record.Fields = make(map[string]storage.Value)
				}
				record.Fields["CustomPermissions"] = storage.ListValue(customPermissions...)
				state.Records[id] = record
			}
		}
		applyProjectPermissionSetMetadataPermissions(org, file, string(id), &generator)
	}
	org.IDSequences = generator.Sequences
	org.Objects["PermissionSet"] = state
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

func readPermissionSetMetadata(file string) (permissionSetMetadata, bool) {
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

func applyProjectPermissionSetMetadataPermissions(org *storage.OrgState, file, parentID string, generator *storage.IDGenerator) {
	if org == nil || strings.TrimSpace(parentID) == "" || generator == nil {
		return
	}
	metadata, ok := readPermissionSetMetadata(file)
	if !ok {
		return
	}
	storage.EnsureStandardObject(org, "ObjectPermissions")
	storage.EnsureStandardObject(org, "FieldPermissions")
	objectState := org.Objects["ObjectPermissions"]
	if objectState.Records == nil {
		objectState.Records = make(map[storage.ID]storage.Record)
	}
	fieldState := org.Objects["FieldPermissions"]
	if fieldState.Records == nil {
		fieldState.Records = make(map[storage.ID]storage.Record)
	}
	fieldPermissionKeys := fieldPermissionRecordKeys(fieldState)
	for _, permission := range metadata.ObjectPermission {
		objectName := strings.TrimSpace(permission.Object)
		if objectName == "" || objectPermissionRecordExists(objectState, parentID, objectName) {
			continue
		}
		ensurePermissionReferencedObject(org, objectName)
		id, err := generator.Next("ObjectPermissions")
		if err != nil {
			continue
		}
		objectState.Records[id] = storage.Record{
			ID:     id,
			Object: "ObjectPermissions",
			Fields: map[string]storage.Value{
				"ParentId":                    storage.IDValue(storage.ID(parentID)),
				"SObjectType":                 storage.StringValue(objectName),
				"PermissionsRead":             storage.BooleanValue(permission.AllowRead),
				"PermissionsCreate":           storage.BooleanValue(permission.AllowCreate),
				"PermissionsEdit":             storage.BooleanValue(permission.AllowEdit),
				"PermissionsDelete":           storage.BooleanValue(permission.AllowDelete),
				"PermissionsViewAllRecords":   storage.BooleanValue(permission.ViewAllRecords),
				"PermissionsModifyAllRecords": storage.BooleanValue(permission.ModifyAllRecords),
			},
		}
	}
	for _, permission := range metadata.FieldPermissions {
		fieldName := strings.TrimSpace(permission.Field)
		if fieldName == "" {
			continue
		}
		objectName := fieldPermissionObjectName(fieldName)
		key := fieldPermissionRecordKey(parentID, objectName, fieldName)
		if objectName == "" || fieldPermissionKeys[key] {
			continue
		}
		ensurePermissionReferencedObjectField(org, objectName, fieldName)
		id, err := generator.Next("FieldPermissions")
		if err != nil {
			continue
		}
		fieldState.Records[id] = storage.Record{
			ID:     id,
			Object: "FieldPermissions",
			Fields: map[string]storage.Value{
				"ParentId":        storage.IDValue(storage.ID(parentID)),
				"SObjectType":     storage.StringValue(objectName),
				"Field":           storage.StringValue(fieldName),
				"PermissionsRead": storage.BooleanValue(permission.Readable),
				"PermissionsEdit": storage.BooleanValue(permission.Editable),
			},
		}
		fieldPermissionKeys[key] = true
	}
	for _, visibility := range metadata.RecordTypeVisibilities {
		if !visibility.Visible {
			continue
		}
		objectName, developerName, ok := strings.Cut(strings.TrimSpace(visibility.RecordType), ".")
		if !ok || objectName == "" || developerName == "" {
			continue
		}
		ensurePermissionReferencedObject(org, objectName)
		canonicalObject, ok := profileRecordTypeObjectName(*org, objectName)
		if !ok {
			continue
		}
		developerName = stripRecordTypeNamespaceToken(developerName)
		state := org.Objects[canonicalObject]
		if profileRecordTypeExists(state.Definition.RecordTypes, developerName) {
			continue
		}
		state.Definition.RecordTypes = append(state.Definition.RecordTypes, storage.RecordTypeInfo{
			DeveloperName: developerName,
			Name:          recordTypeLabelFromDeveloperName(developerName),
			Active:        true,
			Available:     true,
			Default:       visibility.Default,
		})
		org.Objects[canonicalObject] = state
	}
	org.Objects["ObjectPermissions"] = objectState
	org.Objects["FieldPermissions"] = fieldState
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
	ensureProjectDataReferencedObjectField(org, objectName, qualifiedFieldName, "")
}

func ensureProjectDataReferencedObjectField(org *storage.OrgState, objectName, qualifiedFieldName string, hintedType storage.FieldType) {
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
	field := inferredReferencedField(*org, resolvedObjectName, fieldName, hintedType)
	if existingName, ok := storage.ResolveFieldName(state.Definition, org.Namespace, fieldName); ok {
		existing := state.Definition.Fields[existingName]
		if !permissionReferencedFieldCanUpgrade(existing, field) {
			if permissionReferencedFieldCanMergeChildRelationship(existing, field) {
				if existing.ChildRelationshipName == "" {
					existing.ChildRelationshipName = field.ChildRelationshipName
				}
				state.Definition.Fields[existingName] = existing
				state.Definition.Relations = mergePermissionReferencedFieldRelation(state.Definition.Relations, storage.Relationship{
					Field:              existing.APIName,
					ParentObjects:      append([]string(nil), existing.ReferenceTo...),
					ParentRelationship: existing.RelationshipName,
					ChildRelationship:  existing.ChildRelationshipName,
					Polymorphic:        len(existing.ReferenceTo) > 1,
				})
				org.Objects[resolvedObjectName] = state
			}
			return
		}
		delete(state.Definition.Fields, existingName)
		state.Definition.Relations = removePermissionReferencedFieldRelations(state.Definition.Relations, existing.APIName)
	}
	state.Definition.Fields[fieldName] = field
	if field.Type == storage.FieldReference && field.RelationshipName != "" && len(field.ReferenceTo) > 0 {
		state.Definition.Relations = mergePermissionReferencedFieldRelation(state.Definition.Relations, storage.Relationship{
			Field:              field.APIName,
			ParentObjects:      append([]string(nil), field.ReferenceTo...),
			ParentRelationship: field.RelationshipName,
			ChildRelationship:  field.ChildRelationshipName,
			Polymorphic:        len(field.ReferenceTo) > 1,
		})
	}
	org.Objects[resolvedObjectName] = state
}

func permissionReferencedFieldCanUpgrade(existing, replacement storage.Field) bool {
	if replacement.Type == storage.FieldReference && existing.Type == storage.FieldReference && len(replacement.ReferenceTo) > 0 {
		return false
	}
	if replacement.Type != storage.FieldReference || existing.Type == storage.FieldReference || len(existing.ReferenceTo) > 0 || existing.RelationshipName != "" {
		return false
	}
	for _, hintedType := range []storage.FieldType{"", storage.FieldBoolean, storage.FieldDecimal} {
		if inferredStandardFieldShapeEqual(existing, inferredReferencedStandardField(existing.APIName, hintedType)) {
			return true
		}
	}
	return false
}

func projectReferencedFieldCanUpgrade(existing, replacement storage.Field) bool {
	if replacement.Type == storage.FieldString || existing.Type == replacement.Type {
		return false
	}
	for _, hintedType := range []storage.FieldType{"", storage.FieldBoolean, storage.FieldDecimal} {
		if inferredStandardFieldShapeEqual(existing, inferredReferencedStandardField(existing.APIName, hintedType)) {
			return true
		}
	}
	return false
}

func permissionReferencedFieldCanMergeChildRelationship(existing, replacement storage.Field) bool {
	if existing.Type != storage.FieldReference || replacement.Type != storage.FieldReference || replacement.ChildRelationshipName == "" {
		return false
	}
	if !strings.EqualFold(existing.RelationshipName, replacement.RelationshipName) || len(existing.ReferenceTo) == 0 || len(replacement.ReferenceTo) == 0 {
		return false
	}
	for _, existingTarget := range existing.ReferenceTo {
		for _, replacementTarget := range replacement.ReferenceTo {
			if strings.EqualFold(existingTarget, replacementTarget) {
				return true
			}
		}
	}
	return false
}

func mergePermissionReferencedFieldRelation(relations []storage.Relationship, replacement storage.Relationship) []storage.Relationship {
	if replacement.Field == "" {
		return relations
	}
	for i := range relations {
		if !strings.EqualFold(relations[i].Field, replacement.Field) {
			continue
		}
		if relations[i].ChildRelationship == "" {
			relations[i].ChildRelationship = replacement.ChildRelationship
		}
		if len(relations[i].ParentObjects) == 0 {
			relations[i].ParentObjects = append([]string(nil), replacement.ParentObjects...)
		}
		if relations[i].ParentRelationship == "" {
			relations[i].ParentRelationship = replacement.ParentRelationship
		}
		return relations
	}
	return append(relations, replacement)
}

func removePermissionReferencedFieldRelations(relations []storage.Relationship, fieldName string) []storage.Relationship {
	if strings.TrimSpace(fieldName) == "" || len(relations) == 0 {
		return relations
	}
	out := relations[:0]
	for _, relation := range relations {
		if strings.EqualFold(relation.Field, fieldName) {
			continue
		}
		out = append(out, relation)
	}
	return out
}

func inferredStandardFieldShapeEqual(existing, inferred storage.Field) bool {
	return existing.APIName == inferred.APIName &&
		existing.Label == inferred.Label &&
		existing.Type == inferred.Type &&
		existing.DisplayType == inferred.DisplayType &&
		existing.DefaultValue == inferred.DefaultValue &&
		existing.Formula == "" &&
		existing.CompoundFieldName == "" &&
		len(existing.PicklistValues) == 0
}

func permissionFieldLocalName(fieldName string) string {
	if idx := strings.IndexByte(fieldName, '.'); idx >= 0 && idx < len(fieldName)-1 {
		return strings.TrimSpace(fieldName[idx+1:])
	}
	return strings.TrimSpace(fieldName)
}

func applyProjectPermissionSetGroupRecords(org *storage.OrgState, p project.Project) {
	if org == nil || len(p.PermissionSetGroupFiles) == 0 {
		return
	}
	storage.EnsureStandardObject(org, "PermissionSetGroup")
	state := org.Objects["PermissionSetGroup"]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	generator := storage.NewIDGenerator(storage.StandardKeyPrefixes())
	generator.Sequences = org.IDSequences
	for _, file := range p.PermissionSetGroupFiles {
		name := metadataNameFromPath(file, ".permissionsetgroup-meta.xml", ".permissionsetgroup")
		if name == "" || recordFieldExists(state, "DeveloperName", name) {
			continue
		}
		id, err := generator.Next("PermissionSetGroup")
		if err != nil {
			continue
		}
		state.Records[id] = storage.Record{
			ID:     id,
			Object: "PermissionSetGroup",
			Fields: map[string]storage.Value{
				"DeveloperName": storage.StringValue(name),
				"MasterLabel":   storage.StringValue(strings.ReplaceAll(name, "_", " ")),
				"Status":        storage.StringValue("Updated"),
			},
		}
	}
	org.IDSequences = generator.Sequences
	org.Objects["PermissionSetGroup"] = state
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

func objectPermissionRecordExists(state storage.ObjectState, parentID, objectName string) bool {
	for _, record := range state.Records {
		parent, hasParent := record.GetField("ParentId")
		objectValue, hasObject := record.GetField("SObjectType")
		if hasParent && hasObject && storageIDValueEqualsText(parent, parentID) && storageStringValueEqualsText(objectValue, objectName) {
			return true
		}
	}
	return false
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
	if org == nil {
		return
	}
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	prefixes := storage.AssignDeterministicPrefixes(names, nil)
	for name, prefix := range prefixes {
		state, ok := org.Objects[name]
		if !ok || prefix == "" {
			continue
		}
		state.Definition.KeyPrefix = prefix
		org.Objects[name] = state
	}
}

func applyApexClassRecords(org *storage.OrgState, index typesys.Index, caches ...sourceCache) {
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
		if typ.Kind != apexast.DeclarationClass || strings.Contains(typ.Name, ".") {
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

func applyProjectSetupSingletonRecords(org *storage.OrgState) {
	if org == nil {
		return
	}
	for objectName, state := range org.Objects {
		if !strings.EqualFold(objectName, "Setup_Data__c") || len(state.Records) > 0 {
			continue
		}
		if state.Records == nil {
			state.Records = make(map[storage.ID]storage.Record)
		}
		id := storage.ID("aZZZZZZZZZZZZZZ")
		fields := map[string]storage.Value{
			"Name": storage.StringValue("Default"),
		}
		state.Records[id] = storage.Record{
			ID:     id,
			Object: state.Definition.APIName,
			Fields: fields,
		}
		org.Objects[objectName] = state
	}
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

func compileProjectMethod(className, methodName, returnType string, modifiers []string, file string, r diagnostic.Range, source string) (vm.Method, error) {
	methodSource, err := extractMethodSource(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	params, err := parseParams(methodSource)
	if err != nil {
		return vm.Method{}, err
	}
	body, err := extractMethodBody(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return vm.Method{}, err
	}
	return vm.Method{
		Name:       className + "." + methodName,
		ReturnType: returnType,
		Params:     params,
		Program:    program,
		ClassName:  className,
		IsStatic:   hasModifier(modifiers, "static"),
		Access:     accessModifier(modifiers),
		Modifiers:  modifiers,
		File:       file,
		Line:       r.Start.Line,
		Column:     r.Start.Column,
	}, nil
}

func compileProjectConstructor(className, file string, r diagnostic.Range, source string) (vm.Method, error) {
	methodSource, err := extractMethodSource(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	params, err := parseParams(methodSource)
	if err != nil {
		return vm.Method{}, err
	}
	body, err := extractMethodBody(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return vm.Method{}, err
	}
	return vm.Method{
		Name:          className + ".<init>",
		ReturnType:    "void",
		Params:        params,
		Program:       program,
		ClassName:     className,
		IsConstructor: true,
		File:          file,
		Line:          r.Start.Line,
		Column:        r.Start.Column,
	}, nil
}

func compileProjectInitializer(className, file string, r diagnostic.Range, source string, static bool) (vm.Method, error) {
	body, err := extractMethodBody(source, r)
	if err != nil {
		return vm.Method{}, err
	}
	program, err := vm.CompileAnonymous(body)
	if err != nil {
		return vm.Method{}, err
	}
	name := className + ".<init_block>"
	if static {
		name = className + ".<static_init>"
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
	}, nil
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
	program, err := vm.CompileAnonymous(typeName + " __field = " + expr + ";")
	if err != nil {
		return vm.Value{}, false
	}
	machine := vm.New(nil)
	result, err := machine.Execute(program)
	if err != nil {
		return vm.Value{}, false
	}
	value, ok := result.Vars["__field"]
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
