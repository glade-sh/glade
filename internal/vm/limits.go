package vm

import "fmt"

type LimitMode string

const (
	LimitModePermissive LimitMode = "permissive"
	LimitModeStrict     LimitMode = "strict"
)

type Limits struct {
	Queries       int `json:"queries"`
	QueryRows     int `json:"queryRows"`
	DMLStatements int `json:"dmlStatements"`
	DMLRows       int `json:"dmlRows"`
	HeapSize      int `json:"heapSize"`
	CPUTimeMS     int `json:"cpuTimeMs"`
	Callouts      int `json:"callouts"`
	AsyncJobs     int `json:"asyncJobs"`
	FutureCalls   int `json:"futureCalls"`
	QueueableJobs int `json:"queueableJobs"`
	BatchJobs     int `json:"batchJobs"`
	ScheduledJobs int `json:"scheduledJobs"`
	EmailInvokes  int `json:"emailInvocations"`
}

type LimitCaps struct {
	Queries       int `json:"queries"`
	QueryRows     int `json:"queryRows"`
	DMLStatements int `json:"dmlStatements"`
	DMLRows       int `json:"dmlRows"`
	HeapSize      int `json:"heapSize"`
	CPUTimeMS     int `json:"cpuTimeMs"`
	Callouts      int `json:"callouts"`
	AsyncJobs     int `json:"asyncJobs"`
	FutureCalls   int `json:"futureCalls"`
	QueueableJobs int `json:"queueableJobs"`
	BatchJobs     int `json:"batchJobs"`
	ScheduledJobs int `json:"scheduledJobs"`
	EmailInvokes  int `json:"emailInvocations"`
}

type LimitViolation struct {
	Name  string `json:"name"`
	Used  int    `json:"used"`
	Limit int    `json:"limit"`
}

func defaultLimitCaps() LimitCaps {
	return LimitCaps{
		Queries:       100,
		QueryRows:     50000,
		DMLStatements: 150,
		DMLRows:       10000,
		HeapSize:      6 * 1024 * 1024,
		CPUTimeMS:     10000,
		Callouts:      100,
		AsyncJobs:     50,
		FutureCalls:   50,
		QueueableJobs: 50,
		BatchJobs:     5,
		ScheduledJobs: 100,
		EmailInvokes:  10,
	}
}

func (vm *VM) SetLimitMode(mode LimitMode) {
	vm.limitMode = mode
}

func (vm *VM) SetLimitCaps(caps LimitCaps) {
	vm.limitCaps = caps
}

func (vm *VM) ResetLimits() {
	vm.limits = Limits{}
	vm.limitViolations = nil
}

func (vm *VM) incrementLimit(name string, delta int) error {
	switch name {
	case "queries":
		vm.limits.Queries += delta
		return vm.checkLimit(name, vm.limits.Queries, vm.limitCaps.Queries)
	case "queryRows":
		vm.limits.QueryRows += delta
		return vm.checkLimit(name, vm.limits.QueryRows, vm.limitCaps.QueryRows)
	case "dmlStatements":
		vm.limits.DMLStatements += delta
		return vm.checkLimit(name, vm.limits.DMLStatements, vm.limitCaps.DMLStatements)
	case "dmlRows":
		vm.limits.DMLRows += delta
		return vm.checkLimit(name, vm.limits.DMLRows, vm.limitCaps.DMLRows)
	case "heapSize":
		vm.limits.HeapSize += delta
		return vm.checkLimit(name, vm.limits.HeapSize, vm.limitCaps.HeapSize)
	case "cpuTime":
		vm.limits.CPUTimeMS += delta
		return vm.checkLimit(name, vm.limits.CPUTimeMS, vm.limitCaps.CPUTimeMS)
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
	default:
		return fmt.Errorf("unknown limit %q", name)
	}
}

func (vm *VM) checkLimit(name string, used, cap int) error {
	if cap < 0 || used <= cap {
		return nil
	}
	violation := LimitViolation{Name: name, Used: used, Limit: cap}
	vm.limitViolations = append(vm.limitViolations, violation)
	if vm.limitMode == LimitModeStrict {
		return &RuntimeError{
			Type:    "System.LimitException",
			Message: fmt.Sprintf("Too many %s: %d out of %d", name, used, cap),
			Stack:   vm.stackFrames(),
		}
	}
	return nil
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
		return Int(int64(vm.limits.CPUTimeMS)), true
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
	case "getPublishImmediateDML":
		return Int(0), true
	case "getLimitPublishImmediateDML":
		return Int(150), true
	case "getAggregateQueries", "getApexCursorRows", "getApexCursors", "getApexPaginationCursorRows",
		"getApexPaginationCursors", "getChildRelationshipsDescribes", "getDatabaseTime",
		"getFetchCallsOnApexCursor", "getFieldSetsDescribes", "getFieldsDescribes",
		"getFindSimilarCalls", "getMobilePushApexCalls", "getPicklistDescribes",
		"getQueryLocatorRows", "getRecordTypesDescribes", "getRunAs", "getSavepointRollbacks",
		"getSavepoints", "getScriptStatements", "getSoslQueries":
		return Int(0), true
	case "getLimitAggregateQueries":
		return Int(300), true
	case "getLimitApexCursorRows", "getLimitApexPaginationCursorRows":
		return Int(10000), true
	case "getLimitApexCursors", "getLimitApexPaginationCursors":
		return Int(50), true
	case "getLimitChildRelationshipsDescribes", "getLimitFieldSetsDescribes",
		"getLimitFieldsDescribes", "getLimitPicklistDescribes", "getLimitRecordTypesDescribes":
		return Int(100), true
	case "getLimitDatabaseTime":
		return Int(0), true
	case "getLimitFetchCallsOnApexCursor":
		return Int(10), true
	case "getLimitFindSimilarCalls":
		return Int(10), true
	case "getLimitMobilePushApexCalls":
		return Int(10), true
	case "getLimitQueryLocatorRows":
		return Int(10000), true
	case "getLimitRunAs":
		return Int(100), true
	case "getLimitSavepointRollbacks":
		return Int(100), true
	case "getLimitSavepoints":
		return Int(5), true
	case "getLimitScriptStatements":
		return Int(200000), true
	case "getLimitSoslQueries":
		return Int(20), true
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
