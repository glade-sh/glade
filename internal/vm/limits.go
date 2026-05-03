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
	case "getDmlRows":
		return Int(int64(vm.limits.DMLRows)), true
	case "getLimitDmlRows":
		return Int(int64(vm.limitCaps.DMLRows)), true
	case "getHeapSize":
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
	case "getAsyncJobs":
		return Int(int64(vm.limits.AsyncJobs)), true
	case "getLimitAsyncJobs":
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
	default:
		return Null, false
	}
}
