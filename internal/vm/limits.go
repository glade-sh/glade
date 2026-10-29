package vm

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

type LimitMode string

const (
	LimitModePermissive LimitMode = "permissive"
	LimitModeStrict     LimitMode = "strict"
)

const apexCursorFetchCallLimit = 100
const apexCursorRowLimit = 50_000_000
const syncAggregateQueryLimit = 300

// Limits.getLimitHeapSize() natively: synchronous Apex (anonymous, test
// methods and Scheduled Apex execute) and async Apex (Queueable, @future,
// Batchable execute).
const syncHeapLimit = 10_000_000
const asyncHeapLimit = 25_000_000

// nativeHeapThreshold is where Salesforce enforces the heap under both default
// caps, far above the reported caps, in every context. It accepted about
// 30 MB sync (HEAP3-L30A/T) and 40 MB in a Queueable, Scheduled, Batch and
// @future job (HEAP3-Q40, HEAP4-SCH40/rows). It threw "Apex heap size too
// large: <heap>" on the 100,001-character allocation that crossed it: sync
// 52504652 (HEAP3-L60A/AE/T/TE), Queueable 52504770 (HEAP3-Q60/Q100), Scheduled
// 52504787, Batch 52504864 and @future 52504762 (HEAP4-SCH60/rows), each
// about 100,000 bytes past the previous checkpoint. The threshold lies in
// (52404652, 52504652]; 50 MiB is an in-bracket choice, not a captured byte.
const nativeHeapThreshold = 50 * 1024 * 1024

type Limits struct {
	Queries                  int `json:"queries"`
	AggregateQueries         int `json:"aggregateQueries"`
	QueryRows                int `json:"queryRows"`
	DMLStatements            int `json:"dmlStatements"`
	DMLRows                  int `json:"dmlRows"`
	HeapSize                 int `json:"heapSize"`
	CPUTimeMS                int `json:"cpuTimeMs"`
	Callouts                 int `json:"callouts"`
	AsyncJobs                int `json:"asyncJobs"`
	FutureCalls              int `json:"futureCalls"`
	QueueableJobs            int `json:"queueableJobs"`
	BatchJobs                int `json:"batchJobs"`
	ScheduledJobs            int `json:"scheduledJobs"`
	EmailInvokes             int `json:"emailInvocations"`
	SOSLQueries              int `json:"soslQueries"`
	QueryLocatorRows         int `json:"queryLocatorRows"`
	ApexCursorRows           int `json:"apexCursorRows"`
	ApexCursors              int `json:"apexCursors"`
	FetchCallsOnApexCursor   int `json:"fetchCallsOnApexCursor"`
	ApexPaginationCursors    int `json:"apexPaginationCursors"`
	ApexPaginationCursorRows int `json:"apexPaginationCursorRows"`
	RunAs                    int `json:"runAs"`
	Savepoints               int `json:"savepoints"`
	SavepointRollbacks       int `json:"savepointRollbacks"`
	PublishImmediateDML      int `json:"publishImmediateDml"`
}

type LimitCaps struct {
	Queries             int `json:"queries"`
	QueryRows           int `json:"queryRows"`
	DMLStatements       int `json:"dmlStatements"`
	DMLRows             int `json:"dmlRows"`
	HeapSize            int `json:"heapSize"`
	CPUTimeMS           int `json:"cpuTimeMs"`
	Callouts            int `json:"callouts"`
	AsyncJobs           int `json:"asyncJobs"`
	FutureCalls         int `json:"futureCalls"`
	QueueableJobs       int `json:"queueableJobs"`
	BatchJobs           int `json:"batchJobs"`
	ScheduledJobs       int `json:"scheduledJobs"`
	EmailInvokes        int `json:"emailInvocations"`
	SOSLQueries         int `json:"soslQueries"`
	QueryLocatorRows    int `json:"queryLocatorRows"`
	RunAs               int `json:"runAs"`
	Savepoints          int `json:"savepoints"`
	SavepointRollbacks  int `json:"savepointRollbacks"`
	PublishImmediateDML int `json:"publishImmediateDml"`
	// nativeDefaults marks platform defaults, including their async caps.
	// SetLimitCaps clears it for complete profile, flag or hosted overrides;
	// those caps stay explicit even when their values equal native defaults.
	nativeDefaults bool
}

type LimitViolation struct {
	Name  string `json:"name"`
	Used  int    `json:"used"`
	Limit int    `json:"limit"`
}

func defaultLimitCaps() LimitCaps {
	return LimitCaps{
		Queries:             100,
		QueryRows:           50000,
		DMLStatements:       150,
		DMLRows:             10000,
		HeapSize:            syncHeapLimit,
		CPUTimeMS:           10000,
		Callouts:            100,
		AsyncJobs:           50,
		FutureCalls:         50,
		QueueableJobs:       50,
		BatchJobs:           5,
		ScheduledJobs:       100,
		EmailInvokes:        10,
		SOSLQueries:         20,
		QueryLocatorRows:    10000,
		RunAs:               100,
		Savepoints:          5,
		SavepointRollbacks:  100,
		PublishImmediateDML: 150,
		nativeDefaults:      true,
	}
}

func LimitCapsForProfile(name string) (LimitCaps, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "default", "strict-sync", "custom":
		return defaultLimitCaps(), true
	case "strict-async":
		caps := defaultLimitCaps()
		caps.Queries = 200
		caps.HeapSize = asyncHeapLimit
		caps.CPUTimeMS = 60000
		return caps, true
	default:
		return LimitCaps{}, false
	}
}

// withScheduledLimitWindow runs Scheduled Apex execute as its own async
// transaction, but with the caller's heap and CPU caps: natively a
// Schedulable reports the synchronous heap and CPU limits.
func (vm *VM) withScheduledLimitWindow(run func() (Value, error)) (Value, error) {
	heapSize, cpuTimeMS := vm.limitCaps.HeapSize, vm.limitCaps.CPUTimeMS
	return vm.withAsyncLimitWindow(func() (Value, error) {
		vm.limitCaps.HeapSize, vm.limitCaps.CPUTimeMS = heapSize, cpuTimeMS
		return run()
	})
}

// heapLimitError is strict mode's heap LimitException, or nil. A profile, flag
// or hosted cap is local and throws as soon as it is passed. Under a native
// default cap, passing the reported cap only records a violation; Salesforce
// throws once the heap first exceeds nativeHeapThreshold, with glade's own
// heap count in the native message.
func (vm *VM) heapLimitError(used, cap int) error {
	if !vm.limitCaps.nativeDefaults {
		return &RuntimeError{Type: "System.LimitException", Message: fmt.Sprintf("Too many heapSize: %d out of %d", used, cap), Stack: vm.stackFrames()}
	}
	if used <= nativeHeapThreshold {
		return nil
	}
	return &RuntimeError{Type: "System.LimitException", Message: fmt.Sprintf("Apex heap size too large: %d", used), Stack: vm.stackFrames()}
}

func (vm *VM) SetLimitMode(mode LimitMode) {
	vm.limitMode = mode
}

// SetLimitCaps installs explicit caps (a profile, flags or a hosted limit).
// Async entry preserves them, and strict mode enforces their local heap cap.
func (vm *VM) SetLimitCaps(caps LimitCaps) {
	caps.nativeDefaults = false
	vm.limitCaps = caps
}

func (vm *VM) ResetLimits() {
	vm.limits = Limits{}
	vm.limitViolations = nil
	vm.cpuBudgetUsed = 0
}

func (vm *VM) incrementLimit(name string, delta int) error {
	switch name {
	case "queries":
		vm.limits.Queries += delta
		return vm.checkLimit(name, vm.limits.Queries, vm.limitCaps.Queries)
	case "aggregateQueries":
		vm.limits.AggregateQueries += delta
		if vm.currentAsyncKind == "" {
			return vm.checkLimit(name, vm.limits.AggregateQueries, syncAggregateQueryLimit)
		}
		return nil
	case "queryRows":
		vm.limits.QueryRows += delta
		return vm.checkLimit(name, vm.limits.QueryRows, vm.limitCaps.QueryRows)
	case "dmlStatements":
		if delta > 0 {
			vm.calloutPendingWork.pending = true
		}
		vm.limits.DMLStatements += delta
		return vm.checkLimit(name, vm.limits.DMLStatements, vm.limitCaps.DMLStatements)
	case "dmlRows":
		vm.limits.DMLRows += delta
		return vm.checkLimit(name, vm.limits.DMLRows, vm.limitCaps.DMLRows)
	case "heapSize":
		vm.limits.HeapSize += delta
		return vm.checkLimit(name, vm.limits.HeapSize, vm.limitCaps.HeapSize)
	case "cpuTime":
		return vm.incrementCPUBudget(delta)
	case "callouts":
		vm.limits.Callouts += delta
		return vm.checkLimit(name, vm.limits.Callouts, vm.limitCaps.Callouts)
	case "asyncJobs":
		vm.limits.AsyncJobs += delta
		return vm.checkLimit(name, vm.limits.AsyncJobs, vm.limitCaps.AsyncJobs)
	case "futureCalls":
		vm.limits.FutureCalls += delta
		return vm.checkLimit(name, vm.limits.FutureCalls, vm.limitCaps.FutureCalls)
	case "queueableJobs":
		vm.limits.QueueableJobs += delta
		return vm.checkLimit(name, vm.limits.QueueableJobs, vm.limitCaps.QueueableJobs)
	case "batchJobs":
		vm.limits.BatchJobs += delta
		return vm.checkLimit(name, vm.limits.BatchJobs, vm.limitCaps.BatchJobs)
	case "scheduledJobs":
		vm.limits.ScheduledJobs += delta
		return vm.checkLimit(name, vm.limits.ScheduledJobs, vm.limitCaps.ScheduledJobs)
	case "emailInvocations":
		vm.limits.EmailInvokes += delta
		return vm.checkLimit(name, vm.limits.EmailInvokes, vm.limitCaps.EmailInvokes)
	case "soslQueries":
		vm.limits.SOSLQueries += delta
		return vm.checkLimit(name, vm.limits.SOSLQueries, vm.limitCaps.SOSLQueries)
	case "queryLocatorRows":
		vm.limits.QueryLocatorRows += delta
		return vm.checkLimit(name, vm.limits.QueryLocatorRows, vm.limitCaps.QueryLocatorRows)
	case "apexPaginationCursors":
		vm.limits.ApexPaginationCursors += delta
		return vm.checkLimit(name, vm.limits.ApexPaginationCursors, 50)
	case "apexPaginationCursorRows":
		vm.limits.ApexPaginationCursorRows += delta
		return vm.checkLimit(name, vm.limits.ApexPaginationCursorRows, 100000)
	case "apexCursorRows":
		vm.limits.ApexCursorRows += delta
		return vm.checkLimit(name, vm.limits.ApexCursorRows, apexCursorRowLimit)
	case "apexCursors":
		vm.limits.ApexCursors += delta
		return vm.checkLimit(name, vm.limits.ApexCursors, 50)
	case "fetchCallsOnApexCursor":
		vm.limits.FetchCallsOnApexCursor += delta
		return vm.checkLimit(name, vm.limits.FetchCallsOnApexCursor, apexCursorFetchCallLimit)
	case "runAs":
		vm.limits.RunAs += delta
		return vm.checkLimit(name, vm.limits.RunAs, vm.limitCaps.RunAs)
	case "savepoints":
		vm.limits.Savepoints += delta
		return vm.checkLimit(name, vm.limits.Savepoints, vm.limitCaps.Savepoints)
	case "savepointRollbacks":
		vm.limits.SavepointRollbacks += delta
		return vm.checkLimit(name, vm.limits.SavepointRollbacks, vm.limitCaps.SavepointRollbacks)
	case "publishImmediateDml":
		vm.limits.PublishImmediateDML += delta
		return vm.checkLimit(name, vm.limits.PublishImmediateDML, vm.limitCaps.PublishImmediateDML)
	default:
		return fmt.Errorf("unknown limit %q", name)
	}
}

// CPU samples use a deterministic estimate of Apex work, rather than host
// elapsed or interpreter CPU time. Each instruction, loop iteration and
// existing SOQL/DML row charge contributes one microsecond. This preserves
// work-based progress without charging host waits, unrelated goroutines or
// differences in interpreter speed. Native CPU milliseconds are not emulated.
// Sampling and governor enforcement both use the resulting milliseconds.
const cpuBudgetUnitsPerMillisecond = 1000

func (vm *VM) sampleCPUTimeMS() int {
	ms := vm.cpuBudgetUsed / cpuBudgetUnitsPerMillisecond
	if ms == 0 && vm.cpuBudgetUsed > 0 {
		ms = 1
	}
	vm.limits.CPUTimeMS = ms
	return ms
}

func (vm *VM) incrementCPUBudget(delta int) error {
	if delta <= 0 {
		return nil
	}
	vm.cpuBudgetUsed += delta
	return vm.checkLimit("cpuTime", vm.sampleCPUTimeMS(), vm.limitCaps.CPUTimeMS)
}

func (vm *VM) checkLimit(name string, used, cap int) error {
	if cap < 0 || used <= cap {
		return nil
	}
	violation := LimitViolation{Name: name, Used: used, Limit: cap}
	recorded := false
	for i := range vm.limitViolations {
		if vm.limitViolations[i].Name == name {
			vm.limitViolations[i] = violation
			recorded = true
			break
		}
	}
	if !recorded {
		vm.limitViolations = append(vm.limitViolations, violation)
	}
	if vm.limitMode == LimitModeStrict && name == "heapSize" {
		return vm.heapLimitError(used, cap)
	}
	if vm.limitMode == LimitModeStrict {
		message := fmt.Sprintf("Too many %s: %d out of %d", name, used, cap)
		if vm.limitCaps.nativeDefaults && vm.currentAsyncKind == "" {
			switch name {
			case "queries":
				message = fmt.Sprintf("Too many SOQL queries: %d", used)
			case "aggregateQueries":
				message = fmt.Sprintf("Too many SOQL aggregate queries: %d", used)
			case "dmlStatements":
				message = fmt.Sprintf("Too many DML statements: %d", used)
			}
		}
		return &RuntimeError{
			Type:    "System.LimitException",
			Message: message,
			Stack:   vm.stackFrames(),
		}
	}
	return nil
}

func (vm *VM) incrementQueryLocatorRows(value Value) error {
	if value.Kind != ValueList {
		return nil
	}
	return vm.incrementLimit("queryLocatorRows", len(value.List))
}

func (vm *VM) limitValue(name string) (Value, bool) {
	name = canonicalLimitGetterName(name)
	switch name {
	case "getQueries":
		return Int(int64(vm.limits.Queries)), true
	case "getLimitQueries":
		return Int(int64(vm.limitCaps.Queries)), true
	case "getQueryRows":
		return Int(int64(vm.limits.QueryRows)), true
	case "getLimitQueryRows":
		return Int(int64(vm.limitCaps.QueryRows)), true
	case "getDmlStatements":
		return Int(int64(vm.limits.DMLStatements)), true
	case "getLimitDmlStatements":
		return Int(int64(vm.limitCaps.DMLStatements)), true
	case "getDMLStatements":
		return Int(int64(vm.limits.DMLStatements)), true
	case "getLimitDMLStatements":
		return Int(int64(vm.limitCaps.DMLStatements)), true
	case "getDmlRows":
		return Int(int64(vm.limits.DMLRows)), true
	case "getLimitDmlRows":
		return Int(int64(vm.limitCaps.DMLRows)), true
	case "getDMLRows":
		return Int(int64(vm.limits.DMLRows)), true
	case "getLimitDMLRows":
		return Int(int64(vm.limitCaps.DMLRows)), true
	case "getHeapSize":
		vm.limits.HeapSize = vm.currentHeapSize()
		return Int(int64(vm.limits.HeapSize)), true
	case "getLimitHeapSize":
		return Int(int64(vm.limitCaps.HeapSize)), true
	case "getCpuTime":
		return Int(int64(vm.sampleCPUTimeMS())), true
	case "getLimitCpuTime":
		return Int(int64(vm.limitCaps.CPUTimeMS)), true
	case "getCallouts":
		return Int(int64(vm.limits.Callouts)), true
	case "getLimitCallouts":
		return Int(int64(vm.limitCaps.Callouts)), true
	case "getAsyncJobs", "getAsyncCalls":
		return Int(int64(vm.limits.AsyncJobs)), true
	case "getLimitAsyncJobs", "getLimitAsyncCalls":
		return Int(int64(vm.limitCaps.AsyncJobs)), true
	case "getQueueableJobs":
		return Int(int64(vm.limits.QueueableJobs)), true
	case "getLimitQueueableJobs":
		if vm.limitCaps.nativeDefaults && vm.currentAsyncKind == "Queueable" {
			return Int(1), true
		}
		return Int(int64(vm.limitCaps.QueueableJobs)), true
	case "getFutureCalls":
		return Int(int64(vm.limits.FutureCalls)), true
	case "getLimitFutureCalls":
		return Int(int64(vm.limitCaps.FutureCalls)), true
	case "getBatchJobs":
		return Int(int64(vm.limits.BatchJobs)), true
	case "getLimitBatchJobs":
		return Int(int64(vm.limitCaps.BatchJobs)), true
	case "getScheduledJobs":
		return Int(int64(vm.limits.ScheduledJobs)), true
	case "getLimitScheduledJobs":
		return Int(int64(vm.limitCaps.ScheduledJobs)), true
	case "getEmailInvocations":
		return Int(int64(vm.limits.EmailInvokes)), true
	case "getLimitEmailInvocations":
		return Int(int64(vm.limitCaps.EmailInvokes)), true
	case "getSoslQueries":
		return Int(int64(vm.limits.SOSLQueries)), true
	case "getPublishImmediateDML":
		return Int(int64(vm.limits.PublishImmediateDML)), true
	case "getLimitPublishImmediateDML":
		return Int(int64(vm.limitCaps.PublishImmediateDML)), true
	case "getApexPaginationCursorRows":
		return Int(int64(vm.limits.ApexPaginationCursorRows)), true
	case "getApexPaginationCursors":
		return Int(int64(vm.limits.ApexPaginationCursors)), true
	case "getAggregateQueries":
		return Int(int64(vm.limits.AggregateQueries)), true
	case "getDatabaseTime",
		"getFieldSetsDescribes", "getFieldsDescribes",
		"getFindSimilarCalls", "getMobilePushApexCalls", "getPicklistDescribes",
		"getRecordTypesDescribes", "getScriptStatements":
		return Int(0), true
	case "getApexCursorRows":
		return Int(int64(vm.limits.ApexCursorRows)), true
	case "getApexCursors":
		return Int(int64(vm.limits.ApexCursors)), true
	case "getFetchCallsOnApexCursor":
		return Int(int64(vm.limits.FetchCallsOnApexCursor)), true
	case "getLimitAggregateQueries":
		return Int(syncAggregateQueryLimit), true
	case "getLimitApexCursorRows":
		return Int(apexCursorRowLimit), true
	case "getLimitApexPaginationCursorRows":
		return Int(100000), true
	case "getLimitApexCursors", "getLimitApexPaginationCursors":
		return Int(50), true
	case "getLimitChildRelationshipsDescribes", "getLimitFieldSetsDescribes",
		"getLimitFieldsDescribes", "getLimitPicklistDescribes", "getLimitRecordTypesDescribes":
		return Int(100), true
	case "getLimitDatabaseTime":
		return Int(0), true
	case "getLimitFetchCallsOnApexCursor":
		return Int(apexCursorFetchCallLimit), true
	case "getLimitFindSimilarCalls":
		return Int(20), true
	case "getLimitMobilePushApexCalls":
		return Int(10), true
	case "getLimitQueryLocatorRows":
		return Int(int64(vm.limitCaps.QueryLocatorRows)), true
	case "getQueryLocatorRows":
		return Int(int64(vm.limits.QueryLocatorRows)), true
	case "getLimitRunAs":
		return Int(int64(vm.limitCaps.DMLStatements)), true
	case "getRunAs":
		return Int(int64(vm.limits.DMLStatements)), true
	case "getLimitSavepointRollbacks":
		return Int(int64(vm.limitCaps.DMLStatements)), true
	case "getSavepointRollbacks":
		return Int(int64(vm.limits.DMLStatements)), true
	case "getLimitSavepoints":
		return Int(int64(vm.limitCaps.DMLStatements)), true
	case "getSavepoints":
		return Int(int64(vm.limits.DMLStatements)), true
	case "getLimitScriptStatements":
		return Int(200000), true
	case "getLimitSoslQueries":
		return Int(int64(vm.limitCaps.SOSLQueries)), true
	default:
		return Null, false
	}
}

func (vm *VM) orgLimitValues() []Value {
	type orgLimitSpec struct {
		name  string
		used  int
		limit int
	}
	vm.limits.HeapSize = vm.currentHeapSize()
	specs := []orgLimitSpec{
		{name: "DailyApiRequests", used: vm.limits.Callouts, limit: vm.limitCaps.Callouts},
		{name: "DailyAsyncApexExecutions", used: vm.limits.AsyncJobs, limit: vm.limitCaps.AsyncJobs},
		{name: "DailyBulkApiBatches", used: vm.limits.BatchJobs, limit: vm.limitCaps.BatchJobs},
		{name: "DailyDurableGenericStreamingApiEvents", used: 0, limit: 10000},
		{name: "DailyDurableStreamingApiEvents", used: 0, limit: 10000},
		{name: "DailyStreamingApiEvents", used: 0, limit: 10000},
		{name: "DataStorageMB", used: 0, limit: 5},
		{name: "FileStorageMB", used: 0, limit: 20},
		{name: "HourlyAsyncReportRuns", used: 0, limit: 1200},
		{name: "HourlyDashboardRefreshes", used: 0, limit: 200},
		{name: "HourlyDashboardResults", used: 0, limit: 5000},
		{name: "HourlyDashboardStatuses", used: 0, limit: 999999},
		{name: "HourlyODataCallout", used: vm.limits.Callouts, limit: vm.limitCaps.Callouts},
		{name: "HourlySyncReportRuns", used: 0, limit: 500},
		{name: "HourlyTimeBasedWorkflow", used: 0, limit: 1000},
		{name: "MassEmail", used: vm.limits.EmailInvokes, limit: vm.limitCaps.EmailInvokes},
		{name: "SingleEmail", used: vm.limits.EmailInvokes, limit: vm.limitCaps.EmailInvokes},
	}
	if vm != nil && apexversion.AtLeast(vm.currentMethod.APIVersion, 67) {
		specs = append(specs,
			orgLimitSpec{name: "DailyAsyncApexElasticExecutions", used: vm.limits.AsyncJobs, limit: 2 * vm.limitCaps.AsyncJobs},
			orgLimitSpec{name: "DailyAsyncApexProcessed", used: vm.limits.AsyncJobs, limit: vm.limitCaps.AsyncJobs},
		)
	}
	values := make([]Value, 0, len(specs))
	for _, spec := range specs {
		values = append(values, orgLimitValue(spec.name, spec.used, spec.limit))
	}
	return values
}

func orgLimitValue(name string, used, limit int) Value {
	out := Object("OrgLimit")
	out.Fields["name"] = String(name)
	out.Fields["value"] = Int(int64(used))
	out.Fields["limit"] = Int(int64(limit))
	return out
}

func unsupportedLimitGetter(name string) bool {
	name = canonicalLimitGetterName(name)
	switch name {
	default:
		return false
	}
}

func canonicalLimitGetterName(name string) string {
	return canonicalStdlibMemberName(name,
		"getQueries",
		"getLimitQueries",
		"getQueryRows",
		"getLimitQueryRows",
		"getDmlStatements",
		"getLimitDmlStatements",
		"getDMLStatements",
		"getLimitDMLStatements",
		"getDmlRows",
		"getLimitDmlRows",
		"getDMLRows",
		"getLimitDMLRows",
		"getHeapSize",
		"getLimitHeapSize",
		"getCpuTime",
		"getLimitCpuTime",
		"getCallouts",
		"getLimitCallouts",
		"getAsyncJobs",
		"getLimitAsyncJobs",
		"getAsyncCalls",
		"getLimitAsyncCalls",
		"getQueueableJobs",
		"getLimitQueueableJobs",
		"getFutureCalls",
		"getLimitFutureCalls",
		"getBatchJobs",
		"getLimitBatchJobs",
		"getScheduledJobs",
		"getLimitScheduledJobs",
		"getEmailInvocations",
		"getLimitEmailInvocations",
		"getAggregateQueries",
		"getLimitAggregateQueries",
		"getFindSimilarCalls",
		"getLimitFindSimilarCalls",
		"getMobilePushApexCalls",
		"getLimitMobilePushApexCalls",
		"getPublishImmediateDML",
		"getLimitPublishImmediateDML",
		"getQueryLocatorRows",
		"getLimitQueryLocatorRows",
		"getSavepointRollbacks",
		"getLimitSavepointRollbacks",
		"getSoslQueries",
		"getLimitSoslQueries",
	)
}
