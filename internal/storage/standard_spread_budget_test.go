package storage

import (
	"reflect"
	"sync"
	"testing"
)

var spreadBudgetShapes = [][]string{
	nil,
	{"MultiCurrency"},
	{"PersonAccounts"},
	{"MultiCurrency", "PersonAccounts"},
	{"Communities"},
	{"Communities", "MultiCurrency"},
}

// setSpreadBudget sets the template field budget for one test.
func setSpreadBudget(t *testing.T, fields int) {
	t.Helper()
	was := SetSpreadTemplateFieldBudget(fields)
	t.Cleanup(func() { SetSpreadTemplateFieldBudget(was) })
}

// liveTemplateFields counts the fields held by cells that are built and not
// evicted. groups counts the groups with at least one such cell.
func liveTemplateFields() (fields, groups int) {
	seen := map[[4]any]bool{}
	spreadTemplates.Range(func(k, v any) bool {
		cell := v.(*spreadTemplateCell)
		if cell.evicted || !cell.done.Load() || !cell.ok {
			return true
		}
		key := k.(spreadTemplateKey)
		seen[[4]any{key.phase, key.signature, key.multiCurr, key.recordType}] = true
		fields += len(cell.state.Definition.Fields)
		return true
	})
	return fields, len(seen)
}

// evictedCells counts tombstones.
func evictedCells() (n int) {
	spreadTemplates.Range(func(_, v any) bool {
		if v.(*spreadTemplateCell).evicted {
			n++
		}
		return true
	})
	return n
}

func buildSpreadShape(t *testing.T, features []string) {
	t.Helper()
	org := sourceSpreadLazyOrg(features)
	if !MaterializeStandardObjectSpread(&org) {
		t.Fatalf("materialize %v reported no change", features)
	}
}

// groupCellNames returns the object names of the cells of one shape.
func groupCellNames(probe *OrgState) []string {
	_, hasRecordType := probe.Objects["RecordType"]
	want := spreadTemplateKey{
		phase:      probe.StandardSpread,
		signature:  canonicalFeatureSignature(probe.Features),
		multiCurr:  probe.HasFeature("MultiCurrency"),
		recordType: hasRecordType,
	}
	var names []string
	spreadTemplates.Range(func(k, _ any) bool {
		key := k.(spreadTemplateKey)
		if key.phase == want.phase && key.signature == want.signature && key.multiCurr == want.multiCurr && key.recordType == want.recordType {
			names = append(names, key.object)
		}
		return true
	})
	return names
}

func oneShapeFields(t *testing.T) int {
	t.Helper()
	setSpreadBudget(t, 0)
	buildSpreadShape(t, spreadBudgetShapes[0])
	fields, _ := liveTemplateFields()
	if fields == 0 {
		t.Fatal("a completed spread left no template fields")
	}
	spreadTemplates.Clear()
	return fields
}

func TestSpreadTemplateBudgetDefault(t *testing.T) {
	was := SetSpreadTemplateFieldBudget(1)
	defer SetSpreadTemplateFieldBudget(was)
	if was != 200_000 {
		t.Fatalf("default spread template field budget = %d, want 200000", was)
	}
	if got := SetSpreadTemplateFieldBudget(was); got != 1 {
		t.Fatalf("setter returned %d, want the previous value 1", got)
	}
}

// Six completed shapes must not pile up: live template fields stay within the
// budget of two spreads. Fails when eviction is removed.
func TestSpreadTemplateBudgetBoundsLiveFields(t *testing.T) {
	resetSpreadMemos(t)
	one := oneShapeFields(t)
	budget := 2*one + one/2
	setSpreadBudget(t, budget)
	for i, features := range spreadBudgetShapes {
		buildSpreadShape(t, features)
		fields, _ := liveTemplateFields()
		if fields > budget {
			t.Fatalf("after shape %d: %d live template fields, budget %d", i, fields, budget)
		}
	}
	if evictedCells() == 0 {
		t.Fatal("six shapes under a two-spread budget evicted nothing")
	}
}

// A budget of zero turns eviction off.
func TestSpreadTemplateBudgetZeroKeepsEveryGroup(t *testing.T) {
	resetSpreadMemos(t)
	setSpreadBudget(t, 0)
	for _, features := range spreadBudgetShapes[:4] {
		buildSpreadShape(t, features)
	}
	if n := evictedCells(); n != 0 {
		t.Fatalf("%d cells evicted with the budget off", n)
	}
	if _, groups := liveTemplateFields(); groups < 4 {
		t.Fatalf("%d live groups, want 4", groups)
	}
}

// A template is a pure function of its key: evicting a group and building it
// again gives equal states.
func TestSpreadTemplatePurityAcrossEviction(t *testing.T) {
	resetSpreadMemos(t)
	one := oneShapeFields(t)
	setSpreadBudget(t, one+one/2)
	for _, features := range spreadBudgetShapes[:3] {
		probe := sourceSpreadLazyOrg(features)
		buildSpreadShape(t, features)
		names := groupCellNames(&probe)
		if len(names) == 0 {
			t.Fatalf("shape %v built no cells", features)
		}
		before := map[string]ObjectState{}
		for _, name := range names {
			state, ok := spreadTemplate(&probe, name)
			if ok {
				before[name] = state
			}
		}
		// Build another shape: the budget evicts this one.
		other := spreadBudgetShapes[(len(features)+3)%len(spreadBudgetShapes)]
		buildSpreadShape(t, other)
		buildSpreadShape(t, spreadBudgetShapes[5])
		if evictedCells() == 0 {
			t.Fatalf("shape %v: nothing evicted", features)
		}
		for name, was := range before {
			got, ok := spreadTemplate(&probe, name)
			if !ok {
				t.Fatalf("%v %s: template missing after eviction", features, name)
			}
			if !reflect.DeepEqual(was.Definition, got.Definition) {
				t.Errorf("%v %s: template differs after eviction", features, name)
			}
		}
	}
}

// SpreadObjectUntouched answers the same before and after an eviction.
func TestSpreadObjectUntouchedAcrossEviction(t *testing.T) {
	resetSpreadMemos(t)
	one := oneShapeFields(t)
	setSpreadBudget(t, one+one/2)
	features := spreadBudgetShapes[1]
	org := sourceSpreadLazyOrg(features)
	names := owedNames(&org, 5)
	for _, name := range names[:4] {
		if _, ok := installSpreadObject(&org, name); !ok {
			t.Fatalf("install %s", name)
		}
	}
	withRecord := org.Objects[names[1]]
	withRecord.Records = map[ID]Record{"001000000000001AAA": {}}
	org.Objects[names[1]] = withRecord
	org.Objects["Evict_Custom__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Evict_Custom__c"}}
	clone := org.Clone()

	answer := func() map[string]bool {
		out := map[string]bool{}
		for _, name := range append(append([]string{}, names...), "Evict_Custom__c") {
			out[name] = org.SpreadObjectUntouched(name)
			out["clone:"+name] = clone.SpreadObjectUntouched(name)
		}
		return out
	}
	before := answer()
	if !before[names[0]] || !before["clone:"+names[0]] {
		t.Fatalf("installed untouched object not untouched before eviction: %v", before)
	}
	if before[names[1]] || before[names[4]] || before["Evict_Custom__c"] {
		t.Fatalf("record-bearing, uninstalled or custom object reported untouched: %v", before)
	}
	buildSpreadShape(t, spreadBudgetShapes[2])
	buildSpreadShape(t, spreadBudgetShapes[3])
	if evictedCells() == 0 {
		t.Fatal("nothing evicted")
	}
	if after := answer(); !reflect.DeepEqual(before, after) {
		t.Fatalf("untouched answers changed across eviction:\nbefore %v\nafter  %v", before, after)
	}
}

// Goroutines that meet a tombstone build one replacement and share it.
func TestSpreadTemplateTombstoneRebuildsOnce(t *testing.T) {
	resetSpreadMemos(t)
	one := oneShapeFields(t)
	setSpreadBudget(t, one+one/2)
	probe := sourceSpreadLazyOrg(spreadBudgetShapes[1])
	buildSpreadShape(t, spreadBudgetShapes[1])
	name := owedNames(&probe, 1)[0]
	buildSpreadShape(t, spreadBudgetShapes[2])
	buildSpreadShape(t, spreadBudgetShapes[3])
	if evictedCells() == 0 {
		t.Fatal("nothing evicted")
	}
	states := make([]ObjectState, 8)
	var wg sync.WaitGroup
	for i := range states {
		wg.Add(1)
		go func() {
			defer wg.Done()
			states[i], _ = spreadTemplate(&probe, name)
		}()
	}
	wg.Wait()
	first := reflect.ValueOf(states[0].Definition.Fields).Pointer()
	for i := range states {
		if reflect.ValueOf(states[i].Definition.Fields).Pointer() != first {
			t.Fatalf("goroutine %d built its own replacement", i)
		}
	}
}

// Orgs of six shapes complete on eight goroutines under a one-spread budget
// while another goroutine releases the catalog. Each final org equals an eager
// org of its shape.
func TestSpreadBudgetRaceMatchesEager(t *testing.T) {
	resetSpreadMemos(t)
	one := oneShapeFields(t)
	eager := make([]OrgState, len(spreadBudgetShapes))
	for i, features := range spreadBudgetShapes {
		eager[i] = sourceSpreadEagerOrg(features)
	}
	resetSpreadMemos(t)
	setSpreadBudget(t, one)
	names := KnownStandardObjectNames()
	orgs := make([]OrgState, 8)
	for i := range orgs {
		orgs[i] = sourceSpreadLazyOrg(spreadBudgetShapes[i%len(spreadBudgetShapes)])
	}
	stop := make(chan struct{})
	var releaser sync.WaitGroup
	releaser.Add(1)
	go func() {
		defer releaser.Done()
		for {
			select {
			case <-stop:
				return
			default:
				ReleaseStandardCatalogCaches()
			}
		}
	}()
	var wg sync.WaitGroup
	for i := range orgs {
		wg.Add(1)
		go func(org *OrgState, offset int) {
			defer wg.Done()
			for j := range names {
				installSpreadObject(org, names[(j+offset*97)%len(names)])
				org.SpreadObjectUntouched(names[j])
			}
			MaterializeStandardObjectSpread(org)
		}(&orgs[i], i)
	}
	wg.Wait()
	close(stop)
	releaser.Wait()
	for i := range orgs {
		compareCompleteOrgs(t, eager[i%len(spreadBudgetShapes)], orgs[i])
	}
}
