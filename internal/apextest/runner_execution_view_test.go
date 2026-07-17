package apextest

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRuntimeExecutionViewOmitsCompiledPayloadAndCopiesTriggerErrors(t *testing.T) {
	triggerErrors := []error{errors.New("first trigger error"), errors.New("second trigger error")}
	entry := runtimeCacheEntry{
		Methods:       map[string]vm.Method{"Example.run": {Name: "run"}},
		Classes:       []vm.Class{{Name: "Example"}},
		Triggers:      []vm.Trigger{{Name: "ExampleTrigger"}},
		TriggerErrors: triggerErrors,
		PageNames:     []string{"ExamplePage"},
		BaseErr:       errors.New("base error"),
		restored:      vm.NewRestoredRuntimeTemplate(storage.NewOrgState(), vm.New(nil)),
		patchAuthority: &runtimePatchAuthority{
			sourceReferences: map[string]string{"Example.cls": "source"},
		},
	}

	view, ok := runtimeExecutionViewFromEntry(entry)
	if !ok {
		t.Fatal("valid restored runtime did not produce an execution view")
	}
	viewType := reflect.TypeOf(view)
	for _, forbidden := range []string{"Methods", "Classes", "Triggers", "PageNames", "patchAuthority"} {
		if _, present := viewType.FieldByName(forbidden); present {
			t.Fatalf("execution view retains unused compiled payload field %s", forbidden)
		}
	}
	if !view.restored.Valid() || view.BaseErr != entry.BaseErr {
		t.Fatalf("execution view lost required restored runtime state: %#v", view)
	}
	if got, want := len(view.TriggerErrors), 2; got != want {
		t.Fatalf("trigger error count = %d, want %d", got, want)
	}
	triggerErrors[0] = errors.New("mutated source slice")
	if got := view.TriggerErrors[0].Error(); got != "first trigger error" {
		t.Fatalf("execution view aliases cache-owned trigger error slice: %q", got)
	}
}

func TestRuntimeExecutionProjectionRejectsUncloneableAuthoritylessPayload(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	cycle := ir.Expr{Kind: ir.ExprUnary, Operator: "-"}
	cycle.Left = &cycle
	entry := runtimeCacheEntry{
		Methods: map[string]vm.Method{
			"Example.run": {Name: "run", Program: ir.Program{Instructions: []ir.Instruction{{Op: ir.OpReturn, Expr: cycle}}}},
		},
		restored: vm.NewRestoredRuntimeTemplate(storage.NewOrgState(), vm.New(nil)),
	}
	if entry.patchAuthority != nil {
		t.Fatal("fixture unexpectedly has patch authority")
	}
	if _, ok := cloneRuntimeCacheEntryChecked(entry); ok {
		t.Fatal("fixture payload unexpectedly supports a full deep clone")
	}
	if _, ok := runtimeExecutionProjection(entry); ok {
		t.Fatal("execution projection accepted an authority-less payload rejected by the full clone boundary")
	}
	key := runtimeCacheKey("unvalidated-execution-payload")
	runtimeCacheMu.Lock()
	runtimeCache[key] = entry
	runtimeCacheMu.Unlock()
	if _, ok := validMemoryRuntimeExecution(key); ok {
		t.Fatal("memory hit trusted an entry without recorded execution validation")
	}
	runtimeCacheMu.RLock()
	_, retained := runtimeCache[key]
	runtimeCacheMu.RUnlock()
	if retained {
		t.Fatal("memory hit retained an entry without recorded execution validation")
	}
}

func TestRuntimeExecutionCacheHitDoesNotCloneUnusedCompiledPayload(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)

	smallKey := runtimeCacheKey("execution-view-small")
	largeKey := runtimeCacheKey("execution-view-large")
	small := runtimeExecutionAllocationFixture(1)
	large := runtimeExecutionAllocationFixture(4096)
	small.executionProjectionValidated = true
	large.executionProjectionValidated = true
	runtimeCacheMu.Lock()
	runtimeCache[smallKey] = small
	runtimeCache[largeKey] = large
	runtimeCacheMu.Unlock()

	measure := func(key runtimeCacheKey) float64 {
		return testing.AllocsPerRun(100, func() {
			view, ok := validMemoryRuntimeExecution(key)
			if !ok || !view.restored.Valid() || len(view.TriggerErrors) != 1 {
				panic("invalid execution cache hit")
			}
		})
	}
	smallAllocs := measure(smallKey)
	largeAllocs := measure(largeKey)
	t.Logf("validated authority-less cache-hit allocations: 1 method %.1f, 4096 methods %.1f", smallAllocs, largeAllocs)
	if largeAllocs > smallAllocs+1 {
		t.Fatalf("validated authority-less cache-hit allocations scale with compiled payload: small %.1f, large %.1f", smallAllocs, largeAllocs)
	}

	runtimeCacheMu.RLock()
	_, retained := runtimeCache[largeKey]
	runtimeCacheMu.RUnlock()
	if !retained {
		t.Fatal("execution-only cache hit evicted the immutable cache entry")
	}

	full, ok := validMemoryRuntimeEntry(largeKey)
	if !ok {
		t.Fatal("full runtime API rejected a valid deeply cloneable payload")
	}
	full.Methods["caller mutation"] = vm.Method{Name: "caller mutation"}
	runtimeCacheMu.RLock()
	cached := runtimeCache[largeKey]
	runtimeCacheMu.RUnlock()
	if _, mutated := cached.Methods["caller mutation"]; mutated {
		t.Fatal("full runtime API stopped deep-isolating caller-owned methods")
	}
}

func TestRuntimeExecutionColdAndMemoryHitTrustCompilerOriginWithoutSourceDigests(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	index, _ := fixture.fullIndex(t)

	key, cold, err := runtimeFromIndexForExecutionWithSourceDigests(index, nil, newSourceCache(), false)
	if err != nil || !cold.restored.Valid() {
		t.Fatalf("authority-less cold execution view: valid=%t err=%v", cold.restored.Valid(), err)
	}
	runtimeCacheMu.RLock()
	cached := runtimeCache[key]
	runtimeCacheMu.RUnlock()
	if cached.patchAuthority != nil {
		t.Fatal("nil SourceDigests fixture unexpectedly produced patch authority")
	}
	if !cached.executionProjectionValidated {
		t.Fatal("compiler-owned cold entry was published without execution validation")
	}

	warmKey, warm, err := runtimeFromIndexForExecutionWithSourceDigests(index, nil, newSourceCache(), false)
	if err != nil || warmKey != key || !warm.restored.Valid() {
		t.Fatalf("authority-less memory hit: key=%q want=%q valid=%t err=%v", warmKey, key, warm.restored.Valid(), err)
	}
}

func TestRuntimeExecutionViewProjectionAllocationDoesNotScaleWithCompiledPayload(t *testing.T) {
	small := runtimeExecutionAllocationFixture(1)
	large := runtimeExecutionAllocationFixture(4096)
	small.executionProjectionValidated = true
	large.executionProjectionValidated = true
	measure := func(entry runtimeCacheEntry) float64 {
		return testing.AllocsPerRun(100, func() {
			view, ok := runtimeExecutionProjection(entry)
			if !ok || len(view.TriggerErrors) != 1 {
				panic("invalid execution view")
			}
		})
	}
	smallAllocs := measure(small)
	largeAllocs := measure(large)
	t.Logf("execution projection allocations: 1 method %.1f, 4096 methods %.1f", smallAllocs, largeAllocs)
	if largeAllocs > smallAllocs+1 {
		t.Fatalf("execution projection allocations scale with compiled payload: small %.1f, large %.1f", smallAllocs, largeAllocs)
	}
}

func runtimeExecutionAllocationFixture(methods int) runtimeCacheEntry {
	entry := runtimeCacheEntry{
		Methods:       make(map[string]vm.Method, methods),
		Classes:       make([]vm.Class, methods),
		Triggers:      make([]vm.Trigger, methods),
		TriggerErrors: []error{errors.New("trigger error")},
		restored:      vm.NewRestoredRuntimeTemplate(storage.NewOrgState(), vm.New(nil)),
	}
	for i := 0; i < methods; i++ {
		entry.Methods[string(rune(i+1))] = vm.Method{Name: "method"}
		entry.Classes[i] = vm.Class{Name: "Class"}
		entry.Triggers[i] = vm.Trigger{Name: "Trigger"}
	}
	return entry
}
