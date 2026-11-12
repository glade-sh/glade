package vm

// WorkCounts counts the operations that dominate Salesforce CPU time. The
// counters are advisory: nothing in the runtime reads them to change behavior.
type WorkCounts struct {
	Instructions   int `json:"instructions"`
	LoopIterations int `json:"loopIterations"`
	SOQLQueries    int `json:"soqlQueries"`
	SOQLRows       int `json:"soqlRows"`
	DMLStatements  int `json:"dmlStatements"`
	DMLRows        int `json:"dmlRows"`
	TriggerRuns    int `json:"triggerRuns"`
	FlowRuns       int `json:"flowRuns"`
	Callouts       int `json:"callouts"`
}

// Sub returns the counts accumulated since an earlier snapshot.
func (w WorkCounts) Sub(o WorkCounts) WorkCounts {
	return WorkCounts{
		Instructions: w.Instructions - o.Instructions, LoopIterations: w.LoopIterations - o.LoopIterations,
		SOQLQueries: w.SOQLQueries - o.SOQLQueries, SOQLRows: w.SOQLRows - o.SOQLRows,
		DMLStatements: w.DMLStatements - o.DMLStatements, DMLRows: w.DMLRows - o.DMLRows,
		TriggerRuns: w.TriggerRuns - o.TriggerRuns, FlowRuns: w.FlowRuns - o.FlowRuns,
		Callouts: w.Callouts - o.Callouts,
	}
}

// Add returns the sum of two counts.
func (w WorkCounts) Add(o WorkCounts) WorkCounts {
	return WorkCounts{
		Instructions: w.Instructions + o.Instructions, LoopIterations: w.LoopIterations + o.LoopIterations,
		SOQLQueries: w.SOQLQueries + o.SOQLQueries, SOQLRows: w.SOQLRows + o.SOQLRows,
		DMLStatements: w.DMLStatements + o.DMLStatements, DMLRows: w.DMLRows + o.DMLRows,
		TriggerRuns: w.TriggerRuns + o.TriggerRuns, FlowRuns: w.FlowRuns + o.FlowRuns,
		Callouts: w.Callouts + o.Callouts,
	}
}

// MethodWork is the work done inside one Apex method, callees included. A
// recursive method counts each nested call, so its totals overlap.
type MethodWork struct {
	Method string     `json:"method"`
	Calls  int        `json:"calls"`
	Work   WorkCounts `json:"work"`
}

type workState struct {
	instructions   int
	loopIterations int
	triggerRuns    int
	flowRuns       int
	tracking       bool
	methods        map[string]*MethodWork
}

func (w *workState) reset() {
	tracking := w.tracking
	*w = workState{tracking: tracking}
}

func (w *workState) recordMethod(name string, start, end WorkCounts) {
	if w.methods == nil {
		w.methods = make(map[string]*MethodWork)
	}
	entry := w.methods[name]
	if entry == nil {
		entry = &MethodWork{Method: name}
		w.methods[name] = entry
	}
	entry.Calls++
	entry.Work = entry.Work.Add(end.Sub(start))
}

// SetWorkTracking turns on per-method attribution and flow counting. The
// scalar totals are always counted.
func (vm *VM) SetWorkTracking(on bool) { vm.work.tracking = on }

// WorkCounts returns the operations counted since the last ResetLimits.
func (vm *VM) WorkCounts() WorkCounts {
	return WorkCounts{
		Instructions: vm.work.instructions, LoopIterations: vm.work.loopIterations,
		SOQLQueries: vm.limits.Queries, SOQLRows: vm.limits.QueryRows,
		DMLStatements: vm.limits.DMLStatements, DMLRows: vm.limits.DMLRows,
		TriggerRuns: vm.work.triggerRuns, FlowRuns: vm.work.flowRuns,
		Callouts: vm.limits.Callouts,
	}
}

// MethodWork returns per-method work, enabled by SetWorkTracking.
func (vm *VM) MethodWork() []MethodWork {
	out := make([]MethodWork, 0, len(vm.work.methods))
	for _, entry := range vm.work.methods {
		out = append(out, *entry)
	}
	return out
}
