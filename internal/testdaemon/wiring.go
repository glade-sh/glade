package testdaemon

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
	"github.com/glade-sh/glade/internal/watch"
)

const MaxClassShardCountV1 = 1024

func NewRunRequestV1(
	options apextest.Options,
	changedSince string,
	shardCount int,
	shardIndex int,
	returnClassShards bool,
) RunRequestV1 {
	return runRequestV1FromOptions(options, changedSince, shardCount, shardIndex, returnClassShards)
}

func runRequestV1FromOptions(
	options apextest.Options,
	changedSince string,
	shardCount int,
	shardIndex int,
	returnClassShards bool,
) RunRequestV1 {
	return RunRequestV1{
		Filter:              options.Filter,
		ChangedSince:        changedSince,
		SelectedClasses:     append([]string(nil), options.SelectedClasses...),
		SelectedMethod:      options.SelectedMethod,
		LimitMode:           string(options.LimitMode),
		LimitCaps:           limitCapsV1FromVM(options.LimitCaps),
		LimitCapsSet:        options.LimitCapsSet,
		TraceBlocked:        options.TraceBlocked,
		TraceAll:            options.TraceAll,
		SlowTestThresholdMS: options.SlowTestThresholdMS,
		TimeoutMS:           options.TimeoutMS,
		Parallelism:         options.Parallelism,
		ParallelMethods:     options.ParallelMethods,
		NoDiskCache:         options.NoDiskCache,
		ClassDurationMS:     cloneInt64Map(options.ClassDurationMS),
		MethodDurationMS:    cloneInt64Map(options.MethodDurationMS),
		PerfCounters:        options.PerfCounters,
		ShardCount:          shardCount,
		ShardIndex:          shardIndex,
		ReturnClassShards:   returnClassShards,
	}
}

func apexOptionsFromRunRequestV1(request RunRequestV1) apextest.Options {
	return apextest.Options{
		Filter:              request.Filter,
		SelectedClasses:     append([]string(nil), request.SelectedClasses...),
		SelectedMethod:      request.SelectedMethod,
		LimitMode:           vm.LimitMode(request.LimitMode),
		LimitCaps:           vmLimitCapsFromV1(request.LimitCaps),
		LimitCapsSet:        request.LimitCapsSet,
		TraceBlocked:        request.TraceBlocked,
		TraceAll:            request.TraceAll,
		SlowTestThresholdMS: request.SlowTestThresholdMS,
		TimeoutMS:           request.TimeoutMS,
		Parallelism:         request.Parallelism,
		ParallelMethods:     request.ParallelMethods,
		NoDiskCache:         request.NoDiskCache,
		ClassDurationMS:     cloneInt64Map(request.ClassDurationMS),
		MethodDurationMS:    cloneInt64Map(request.MethodDurationMS),
		PerfCounters:        request.PerfCounters,
	}
}

func validateRunRequestV1(request RunRequestV1) error {
	switch request.LimitMode {
	case "", string(vm.LimitModePermissive), string(vm.LimitModeStrict):
	default:
		return fmt.Errorf("unsupported test limit mode %q", request.LimitMode)
	}
	if request.TimeoutMS < 0 || request.SlowTestThresholdMS < 0 {
		return fmt.Errorf("test timing values cannot be negative")
	}
	if request.Parallelism < 1 {
		return fmt.Errorf("test parallelism must be at least 1")
	}
	for name, duration := range request.ClassDurationMS {
		if duration < 0 {
			return fmt.Errorf("class duration %q cannot be negative", name)
		}
	}
	for name, duration := range request.MethodDurationMS {
		if duration < 0 {
			return fmt.Errorf("method duration %q cannot be negative", name)
		}
	}
	if request.ShardCount < 0 || request.ShardIndex < 0 {
		return fmt.Errorf("test shard values cannot be negative")
	}
	if request.ReturnClassShards && request.ShardCount == 0 {
		return fmt.Errorf("test shard plan requires a positive shard count")
	}
	if request.ShardCount > MaxClassShardCountV1 {
		return fmt.Errorf("test shard count cannot exceed %d", MaxClassShardCountV1)
	}
	if request.ShardCount == 0 && request.ShardIndex != 0 {
		return fmt.Errorf("test shard index requires a positive shard count")
	}
	if request.ShardCount > 0 && request.ShardIndex >= request.ShardCount {
		return fmt.Errorf("test shard index must be less than shard count")
	}
	return nil
}

func (d *Daemon) runRequestV1(
	ctx context.Context,
	request RunRequestV1,
) (testreport.Run, watch.TestSelection, *ClassShardPlanV1, error) {
	return d.runRequestV1WithProgress(ctx, request, nil, nil)
}

func (d *Daemon) runRequestV1WithProgress(
	ctx context.Context,
	request RunRequestV1,
	progress func(apextest.TestProgress),
	setProgressTotal func(int),
) (testreport.Run, watch.TestSelection, *ClassShardPlanV1, error) {
	if err := validateRunRequestV1(request); err != nil {
		return testreport.Run{}, watch.TestSelection{}, nil, err
	}
	if err := ctx.Err(); err != nil {
		return testreport.Run{}, watch.TestSelection{}, nil, err
	}
	options := apexOptionsFromRunRequestV1(request)
	options.Progress = progress

	d.mu.RLock()
	index := d.index
	graph := d.graph
	d.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return testreport.Run{}, watch.TestSelection{}, nil, err
	}
	selectorRun, selectorFailed := exactSelectorFailureV1(index, options)
	if err := ctx.Err(); err != nil {
		return testreport.Run{}, watch.TestSelection{}, nil, err
	}
	if selectorFailed {
		selectorRun.Dependencies = append([]typesys.DependencyInfo(nil), index.Dependencies...)
		return selectorRun, watch.TestSelection{}, nil, nil
	}

	var selection watch.TestSelection
	if changedSince := strings.TrimSpace(request.ChangedSince); changedSince != "" {
		if err := ctx.Err(); err != nil {
			return testreport.Run{}, watch.TestSelection{}, nil, err
		}
		changes, err := watch.GitChangesSince(d.root, changedSince)
		if err != nil {
			return testreport.Run{}, watch.TestSelection{}, nil, err
		}
		if err := ctx.Err(); err != nil {
			return testreport.Run{}, watch.TestSelection{}, nil, err
		}
		selection = watch.SelectAffectedTestsWithRefGraph(index, changes, graph)
		if err := ctx.Err(); err != nil {
			return testreport.Run{}, watch.TestSelection{}, nil, err
		}
		selectedOptions, ok := optionsForSelection(options, selection)
		if err := ctx.Err(); err != nil {
			return testreport.Run{}, watch.TestSelection{}, nil, err
		}
		if !ok {
			run := testreport.Run{Name: "glade test", Dependencies: append([]typesys.DependencyInfo(nil), index.Dependencies...)}
			if request.ReturnClassShards {
				if err := ctx.Err(); err != nil {
					return testreport.Run{}, watch.TestSelection{}, nil, err
				}
				plan := PlanClassShards(nil, request.ClassDurationMS, request.ShardCount)
				if err := ctx.Err(); err != nil {
					return testreport.Run{}, watch.TestSelection{}, nil, err
				}
				return run, selection, &plan, nil
			}
			return run, selection, nil, nil
		}
		options = selectedOptions
	}

	if err := ctx.Err(); err != nil {
		return testreport.Run{}, watch.TestSelection{}, nil, err
	}
	cases := apextest.Discover(index, options)
	if err := ctx.Err(); err != nil {
		return testreport.Run{}, watch.TestSelection{}, nil, err
	}
	if request.ReturnClassShards {
		if err := ctx.Err(); err != nil {
			return testreport.Run{}, watch.TestSelection{}, nil, err
		}
		plan := PlanClassShards(cases, request.ClassDurationMS, request.ShardCount)
		if err := ctx.Err(); err != nil {
			return testreport.Run{}, watch.TestSelection{}, nil, err
		}
		return testreport.Run{
			Name:         "glade test",
			Dependencies: append([]typesys.DependencyInfo(nil), index.Dependencies...),
		}, selection, &plan, nil
	}
	if request.ShardCount > 0 {
		if err := ctx.Err(); err != nil {
			return testreport.Run{}, watch.TestSelection{}, nil, err
		}
		plan := PlanClassShards(cases, request.ClassDurationMS, request.ShardCount)
		if err := ctx.Err(); err != nil {
			return testreport.Run{}, watch.TestSelection{}, nil, err
		}
		cases = selectClassShard(cases, plan, request.ShardIndex)
		if err := ctx.Err(); err != nil {
			return testreport.Run{}, watch.TestSelection{}, nil, err
		}
	}
	if setProgressTotal != nil {
		setProgressTotal(len(cases))
	}
	run := apextest.RunCasesContext(ctx, index, options, cases)
	run.Dependencies = append(run.Dependencies, index.Dependencies...)
	return run, selection, nil, nil
}

// RunRequestV1 applies the versioned daemon request semantics in-process.
func (d *Daemon) RunRequestV1(
	ctx context.Context,
	request RunRequestV1,
	progress func(apextest.TestProgress),
	setProgressTotal func(int),
) (testreport.Run, watch.TestSelection, *ClassShardPlanV1, error) {
	return d.runRequestV1WithProgress(ctx, request, progress, setProgressTotal)
}

func exactSelectorFailureV1(index typesys.Index, options apextest.Options) (testreport.Run, bool) {
	if len(options.SelectedClasses) != 1 {
		return testreport.Run{}, false
	}
	className := strings.TrimSpace(options.SelectedClasses[0])
	if className == "" {
		return testreport.Run{}, false
	}
	selectorOptions := options
	selectorOptions.Filter = ""
	if len(apextest.Discover(index, selectorOptions)) > 0 {
		return testreport.Run{}, false
	}
	methodName := strings.TrimSpace(options.SelectedMethod)
	if methodName != "" {
		classOptions := selectorOptions
		classOptions.SelectedMethod = ""
		if len(apextest.Discover(index, classOptions)) > 0 {
			return selectorFailureRunV1(
				"missing test method",
				fmt.Sprintf("no test method matched --class %q --method %q", className, methodName),
				fmt.Sprintf("Glade found test class %q, but no exact test method named %q.", className, methodName),
			), true
		}
	}
	return selectorFailureRunV1(
		"missing test class",
		fmt.Sprintf("no test class matched --class %q", className),
		fmt.Sprintf("Glade did not discover an exact test class named %q.", className),
	), true
}

func selectorFailureRunV1(name, message, detail string) testreport.Run {
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

func PlanClassShards(cases []apextest.TestCase, durations map[string]int64, count int) ClassShardPlanV1 {
	if count <= 0 || count > MaxClassShardCountV1 {
		return ClassShardPlanV1{}
	}
	shards := make([]ClassShardV1, count)
	for index := range shards {
		shards[index].Index = index
	}
	weights := classShardWeights(cases, durations)
	if len(durations) == 0 {
		sort.Slice(weights, func(i, j int) bool { return weights[i].class < weights[j].class })
		for index, weight := range weights {
			target := index % count
			shards[target].Classes = append(shards[target].Classes, weight.class)
			shards[target].TotalDurationMS += weight.durationMS
		}
		return ClassShardPlanV1{Shards: shards}
	}
	for _, weight := range weights {
		target := 0
		for index := 1; index < len(shards); index++ {
			if shards[index].TotalDurationMS < shards[target].TotalDurationMS {
				target = index
			}
		}
		shards[target].Classes = append(shards[target].Classes, weight.class)
		shards[target].TotalDurationMS += weight.durationMS
	}
	for index := range shards {
		sort.Strings(shards[index].Classes)
	}
	return ClassShardPlanV1{Shards: shards}
}

type classShardWeight struct {
	class      string
	durationMS int64
}

func classShardWeights(cases []apextest.TestCase, durations map[string]int64) []classShardWeight {
	methodCounts := map[string]int64{}
	for _, testCase := range cases {
		if testCase.ClassName != "" {
			methodCounts[testCase.ClassName]++
		}
	}
	weights := make([]classShardWeight, 0, len(methodCounts))
	for className, methodCount := range methodCounts {
		duration := durations[className]
		if duration <= 0 {
			duration = methodCount
		}
		weights = append(weights, classShardWeight{class: className, durationMS: duration})
	}
	sort.Slice(weights, func(i, j int) bool {
		if weights[i].durationMS == weights[j].durationMS {
			return weights[i].class < weights[j].class
		}
		return weights[i].durationMS > weights[j].durationMS
	})
	return weights
}

func selectClassShard(cases []apextest.TestCase, plan ClassShardPlanV1, index int) []apextest.TestCase {
	if len(plan.Shards) == 0 {
		return cases
	}
	selected := make(map[string]bool, len(plan.Shards[index].Classes))
	for _, className := range plan.Shards[index].Classes {
		selected[className] = true
	}
	out := make([]apextest.TestCase, 0, len(cases))
	for _, testCase := range cases {
		if selected[testCase.ClassName] {
			out = append(out, testCase)
		}
	}
	return out
}

func cloneInt64Map(values map[string]int64) map[string]int64 {
	if values == nil {
		return nil
	}
	cloned := make(map[string]int64, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func limitCapsV1FromVM(caps vm.LimitCaps) LimitCapsV1 {
	return LimitCapsV1{
		Queries: caps.Queries, QueryRows: caps.QueryRows,
		DMLStatements: caps.DMLStatements, DMLRows: caps.DMLRows,
		HeapSize: caps.HeapSize, CPUTimeMS: caps.CPUTimeMS,
		Callouts: caps.Callouts, AsyncJobs: caps.AsyncJobs,
		FutureCalls: caps.FutureCalls, QueueableJobs: caps.QueueableJobs,
		BatchJobs: caps.BatchJobs, ScheduledJobs: caps.ScheduledJobs,
		EmailInvokes: caps.EmailInvokes, SOSLQueries: caps.SOSLQueries,
		QueryLocatorRows: caps.QueryLocatorRows, RunAs: caps.RunAs,
		Savepoints: caps.Savepoints, SavepointRollbacks: caps.SavepointRollbacks,
		PublishImmediateDML: caps.PublishImmediateDML,
	}
}

func vmLimitCapsFromV1(caps LimitCapsV1) vm.LimitCaps {
	return vm.LimitCaps{
		Queries: caps.Queries, QueryRows: caps.QueryRows,
		DMLStatements: caps.DMLStatements, DMLRows: caps.DMLRows,
		HeapSize: caps.HeapSize, CPUTimeMS: caps.CPUTimeMS,
		Callouts: caps.Callouts, AsyncJobs: caps.AsyncJobs,
		FutureCalls: caps.FutureCalls, QueueableJobs: caps.QueueableJobs,
		BatchJobs: caps.BatchJobs, ScheduledJobs: caps.ScheduledJobs,
		EmailInvokes: caps.EmailInvokes, SOSLQueries: caps.SOSLQueries,
		QueryLocatorRows: caps.QueryLocatorRows, RunAs: caps.RunAs,
		Savepoints: caps.Savepoints, SavepointRollbacks: caps.SavepointRollbacks,
		PublishImmediateDML: caps.PublishImmediateDML,
	}
}
