package vm

import (
	"fmt"
	"maps"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
)

type dmlScopeAliasReasonObserver struct{ reason string }

func (o *dmlScopeAliasReasonObserver) scopeAliasRootVisited()                {}
func (o *dmlScopeAliasReasonObserver) scopeAliasBatchFallback(reason string) { o.reason = reason }

// Unlike a shallow copy of Value, these updates have genuinely new Fields.
func dmlIndependentRows(n int) ([]Value, []dmlScopeAliasMutation) {
	rows := make([]Value, n)
	mutations := make([]dmlScopeAliasMutation, n)
	for i := range rows {
		rows[i] = Object("Account")
		rows[i].Fields["Name"] = String(fmt.Sprint(i))
		updated := rows[i]
		updated.Fields = map[string]Value{"Name": rows[i].Fields["Name"], "Id": String(fmt.Sprintf("id-%d", i))}
		mutations[i] = dmlScopeAliasMutation{previous: snapshotAlias(rows[i]), updated: updated}
	}
	return rows, mutations
}

func TestDMLScopeAliasIndependentUpdates(t *testing.T) {
	for _, n := range []int{1, 63, 64, 65, 199, 200, 201, 400, 1000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			rows, mutations := dmlIndependentRows(n)
			list := List(rows...)
			machine := New(nil)
			machine.Globals["rows"] = list
			machine.Globals["alias"] = list
			referenceRows := make([]Value, n)
			for i, row := range rows {
				referenceRows[i] = row
				referenceRows[i].Fields = maps.Clone(row.Fields)
			}
			referenceList := List(referenceRows...)
			referenceList.Ref = list.Ref
			reference := New(nil)
			reference.Globals["rows"], reference.Globals["alias"] = referenceList, referenceList
			for _, mutation := range mutations {
				updated := mutation.updated
				updated.Fields = maps.Clone(updated.Fields)
				reference.propagateAliasSnapshotToScope(reference.Globals, mutation.previous, updated)
				reference.propagateAliasSnapshotToStatics(mutation.previous, updated)
			}
			for _, row := range rows {
				if _, exists := row.Fields["Id"]; exists {
					t.Fatal("old row already has Id")
				}
			}
			machine.propagateSuccessfulDMLTargetAliases(mutations)
			if !reflect.DeepEqual(machine.Globals, reference.Globals) {
				t.Fatal("prepared publication differs from independent legacy propagation")
			}
			for _, root := range []string{"rows", "alias"} {
				for i, row := range machine.Globals[root].List {
					if row.Ref != mutations[i].updated.Ref || row.Fields["Id"].Text != fmt.Sprintf("id-%d", i) {
						t.Fatalf("%s[%d] did not receive exact update", root, i)
					}
				}
			}
		})
	}
}

func TestDMLScopeAliasRejectsUnsafeBackingBeforePublication(t *testing.T) {
	for _, shape := range []string{"different-ref-map", "overlapping-slices", "same-ref-view", "cycle", "cross-target", "empty-cross-target", "duplicate", "external", "cache", "static"} {
		t.Run(shape, func(t *testing.T) {
			rows, mutations := dmlIndependentRows(2)
			root := List(rows...)
			machine := New(nil)
			machine.Globals["rows"] = root
			switch shape {
			case "different-ref-map":
				a := Object("Envelope")
				a.Fields["row"] = rows[0]
				b := Object("Envelope")
				b.Fields = a.Fields
				machine.Globals["a"], machine.Globals["b"] = a, b
			case "overlapping-slices":
				other := List(rows[1:]...)
				machine.Globals["overlap"] = other
			case "same-ref-view":
				other := root
				other.List = rows[1:]
				machine.Globals["view"] = other
			case "cycle":
				cycle := Object("Envelope")
				cycle.Fields["self"] = cycle
				machine.Globals["cycle"] = cycle
			case "cross-target":
				mutations[0].updated.Fields["child"] = rows[1]
			case "empty-cross-target":
				emptyView := rows[1]
				emptyView.Fields = map[string]Value{}
				mutations[0].updated.Fields["child"] = emptyView
			case "duplicate":
				mutations[1] = mutations[0]
			case "external":
				machine.scopeStack = append(machine.scopeStack, map[string]Value{"rows": root})
			case "static":
				machine.Classes["Holder"] = Class{Name: "Holder", StaticFields: map[string]Field{"rows": {Name: "rows", Value: root}}}
			case "cache":
				machine.platformCache = map[string]map[string]cacheEntry{"owned": {"rows": {Value: root}}}
			}
			observer := &dmlScopeAliasReasonObserver{}
			machine.scopeAliasTraversalObserver = observer
			before := cloneValuePreserveRefs(machine.Globals["rows"])
			if machine.tryPropagateDMLScopeAliasSnapshotMutationsToScope(machine.Globals, mutations) {
				t.Fatal("unsafe graph admitted")
			}
			wantReason := map[string]string{
				"different-ref-map": "unsafe caller graph", "overlapping-slices": "overlapping caller slices",
				"same-ref-view": "unsafe caller graph", "cycle": "unsafe caller graph",
				"cross-target": "non-flat target or incomplete provenance", "empty-cross-target": "non-flat target or incomplete provenance",
				"duplicate": "duplicate mutation target", "external": "external caller sharing",
				"cache": "external cache sharing", "static": "external static sharing",
			}[shape]
			if observer.reason != wantReason {
				t.Fatalf("reason = %q, want %q", observer.reason, wantReason)
			}
			if !reflect.DeepEqual(before, machine.Globals["rows"]) {
				t.Fatal("rejection changed caller graph")
			}
			for _, row := range rows {
				if _, exists := row.Fields["Id"]; exists {
					t.Fatal("rejected preparation published an update")
				}
			}
		})
	}
}

func TestDMLScopeAliasCalleeCountersRemainSeparate(t *testing.T) {
	rows, mutations := dmlIndependentRows(65)
	machine := New(nil)
	machine.Globals["rows"] = List(rows...)
	machine.scopeStack = append(machine.scopeStack, map[string]Value{"scalar": Int(1)})
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)
	machine.propagateSuccessfulDMLTargetAliases(mutations)
	stats := recorder.Snapshot()
	if stats.ScopeAlias.Calls != 0 || stats.DML.ScopeAliasBatchCalls != 1 || stats.DML.ScopeAliasBatchRows != 65 {
		t.Fatalf("callee DML changed caller-return telemetry: scope=%+v dml=%+v", stats.ScopeAlias, stats.DML)
	}
}

func TestDMLScopeAliasPrimitiveRecordBookkeeping(t *testing.T) {
	rows, mutations := dmlIndependentRows(200)
	for i := range rows {
		markExplicitSObjectField(&rows[i], "Name")
		markExplicitSObjectField(&mutations[i].updated, "Name")
		markDMLAccessibleFields(&rows[i])
		markDMLAccessibleFields(&mutations[i].updated)
	}
	machine := New(nil)
	observer := &dmlScopeAliasReasonObserver{}
	machine.scopeAliasTraversalObserver = observer
	machine.Globals["rows"] = List(rows...)
	if !machine.tryPropagateDMLScopeAliasSnapshotMutationsToScope(machine.Globals, mutations) {
		t.Fatalf("ordinary primitive record bookkeeping disabled the bulk path: %s", observer.reason)
	}
}

func TestDMLScopeAliasUnrelatedReferenceFreeExternalRoot(t *testing.T) {
	rows, mutations := dmlIndependentRows(2)
	machine := New(nil)
	machine.Globals["rows"] = List(rows...)
	machine.scopeStack = append(machine.scopeStack, map[string]Value{"empty": {Kind: ValueObject, Type: "Token"}})
	observer := &dmlScopeAliasReasonObserver{}
	machine.scopeAliasTraversalObserver = observer
	if !machine.tryPropagateDMLScopeAliasSnapshotMutationsToScope(machine.Globals, mutations) {
		t.Fatalf("unrelated reference-free external root disabled the batch: %s", observer.reason)
	}
}

func TestDMLScopeAliasScalarObjectFields(t *testing.T) {
	rows, mutations := dmlIndependentRows(2)
	for i := range rows {
		rows[i].Fields["Id"] = platformScalar("Id", fmt.Sprintf("001000000000%03dAAA", i))
		mutations[i].updated.Fields["Id"] = rows[i].Fields["Id"]
		mutations[i].updated.Fields["CreatedDate"] = platformScalar("Datetime", "2026-01-01T00:00:00Z")
	}
	machine := New(nil)
	machine.Globals["rows"] = List(rows...)
	if !machine.tryPropagateDMLScopeAliasSnapshotMutationsToScope(machine.Globals, mutations) {
		t.Fatal("scalar Id/Datetime fields disabled batching")
	}
	rows, mutations = dmlIndependentRows(2)
	forged := platformScalar("Id", "owned")
	forged.Ref = rows[1].Ref
	mutations[0].updated.Fields["Id"] = forged
	machine = New(nil)
	machine.Globals["rows"] = List(rows...)
	if machine.tryPropagateDMLScopeAliasSnapshotMutationsToScope(machine.Globals, mutations) {
		t.Fatal("scalar-looking cross-target view was admitted")
	}
}

func TestDMLScopeAliasIneligiblePreparationDoesNotAllocate(t *testing.T) {
	for _, shape := range []string{"missing-ref", "changed-ref", "changed-type", "nested-record", "oversized"} {
		t.Run(shape, func(t *testing.T) {
			_, mutations := dmlIndependentRows(2)
			switch shape {
			case "missing-ref":
				mutations[0].previous.ref = 0
			case "changed-ref":
				mutations[0].updated.Ref++
			case "changed-type":
				mutations[0].updated.Type = "Contact"
			case "nested-record":
				mutations[0].updated.Fields["child"] = Object("Contact")
			case "oversized":
				_, mutations = dmlIndependentRows(dmlScopeAliasBatchMaxTargets + 1)
			}
			machine := New(nil)
			recorder := NewPerfRecorder()
			machine.SetPerfRecorder(recorder)
			allocations := testing.AllocsPerRun(100, func() {
				if machine.prepareDMLScopeAliasPlan(machine.Globals, mutations) != nil {
					t.Fatal("ineligible targets admitted")
				}
			})
			if allocations != 0 {
				t.Fatalf("ineligible preparation allocated %.0f objects; want zero", allocations)
			}
			if got := recorder.Snapshot().DML; got.ScopeAliasBatchCalls != 0 || got.ScopeAliasBatchFallback != 101 {
				t.Fatalf("unexpected fallback telemetry: %+v", got)
			}
		})
	}
}

func TestDMLScopeAliasIneligibleProductionDoesNotAllocate(t *testing.T) {
	for _, shape := range []string{"missing-ref", "nested-record", "mixed-types", "local-only", "single-success", "no-success", "oversized"} {
		t.Run(shape, func(t *testing.T) {
			rows, _ := dmlIndependentRows(2)
			machine := New(nil)
			results := []dml.Result{{Success: true}, {Success: true}}
			switch shape {
			case "missing-ref":
				rows[0].Ref = 0
			case "nested-record":
				rows[0].Fields["child"] = Object("Contact")
			case "mixed-types":
				rows[1].Type = "Contact"
			case "local-only":
				machine.localOnlyObjectRefs = map[uint64]bool{rows[0].Ref: true}
			case "single-success":
				results[1].Success = false
			case "no-success":
				results[0].Success, results[1].Success = false, false
			case "oversized":
				rows, _ = dmlIndependentRows(dmlScopeAliasBatchMaxTargets + 1)
				results = make([]dml.Result, len(rows))
				for i := range results {
					results[i].Success = true
				}
			}
			targets := make([]*Value, len(rows))
			for i := range rows {
				targets[i] = &rows[i]
			}
			allocations := testing.AllocsPerRun(100, func() {
				if machine.tryPublishDMLScopeAliasBatch(targets, results, "insert") {
					t.Fatal("ineligible publication admitted")
				}
			})
			if allocations != 0 {
				t.Fatalf("ineligible production path allocated %.0f objects; want zero", allocations)
			}
		})
	}
}

// Each construction owns all backing independently, but uses matching logical
// identities. Both the old and updated Fields are distinct before publication.
func dmlContainerFixture(shape string, n int) (*VM, []dmlScopeAliasMutation, Value) {
	rows, mutations := dmlIndependentRows(n)
	for i := range rows {
		rows[i].Ref = uint64(i + 1)
		mutations[i].previous = snapshotAlias(rows[i])
		mutations[i].updated.Ref = rows[i].Ref
	}
	root := Value{Ref: 10000}
	switch shape {
	case "object":
		root.Kind, root.Type, root.Fields = ValueObject, "Envelope", map[string]Value{}
		for i, row := range rows {
			root.Fields[fmt.Sprint(i)] = row
		}
	case "list":
		root.Kind, root.List = ValueList, rows
	case "set":
		root.Kind, root.Set = ValueSet, rows
	case "map-values", "map-keys":
		root.Kind, root.Type = ValueMap, "Map<Account,Account>"
		root.Map, root.MapKeys = map[string]Value{}, map[string]Value{}
		for i, row := range rows {
			key := fmt.Sprint(i)
			root.MapOrder = append(root.MapOrder, key)
			if shape == "map-values" {
				root.Map[key], root.MapKeys[key] = row, String(key)
			} else {
				root.Map[key], root.MapKeys[key] = String(key), row
			}
		}
	case "nested":
		root.Kind, root.Type, root.Fields = ValueObject, "Envelope", map[string]Value{}
		for i, row := range rows {
			leaf := Value{Kind: ValueSet, Ref: uint64(20000 + i), Set: []Value{row, row}}
			m := Value{Kind: ValueMap, Type: "Map<Account,Object>", Ref: uint64(30000 + i), Map: map[string]Value{"k": leaf}, MapKeys: map[string]Value{"k": row}, MapOrder: []string{"k"}}
			root.Fields[fmt.Sprint(i)] = Value{Kind: ValueList, Ref: uint64(40000 + i), List: []Value{m, m}}
		}
	}
	machine := New(nil)
	machine.Globals["root"], machine.Globals["alias"] = root, root
	return machine, mutations, root
}

func applyDMLLegacy(machine *VM, mutations []dmlScopeAliasMutation) {
	for _, mutation := range mutations {
		machine.propagateAliasSnapshotToScope(machine.Globals, mutation.previous, mutation.updated)
		machine.propagateAliasSnapshotToStatics(mutation.previous, mutation.updated)
	}
}

func TestDMLScopeAliasEveryContainerMatchesIndependentLegacy(t *testing.T) {
	for _, shape := range []string{"object", "list", "set", "map-values", "map-keys", "nested"} {
		for _, n := range []int{2, 63, 64, 65, 199, 200} {
			t.Run(fmt.Sprintf("%s/%d", shape, n), func(t *testing.T) {
				machine, mutations, retained := dmlContainerFixture(shape, n)
				reference, referenceMutations, referenceRoot := dmlContainerFixture(shape, n)
				if !reflect.DeepEqual(machine.Globals, reference.Globals) {
					t.Fatal("unequal initial graphs")
				}
				if sameMethodReturnAliasBatchBacking(retained, referenceRoot) {
					t.Fatal("differential graphs share backing")
				}
				for i, m := range mutations {
					var oldFields map[string]Value
					var find func(Value)
					find = func(v Value) {
						if v.Ref == m.previous.ref && v.Kind == ValueObject {
							oldFields = v.Fields
							return
						}
						for _, dictionary := range dmlScopeAliasMaps(v) {
							for _, child := range dictionary {
								find(child)
							}
						}
						for _, child := range dmlScopeAliasSequence(v) {
							find(child)
						}
					}
					find(retained)
					if oldFields == nil || sameMapBacking(oldFields, m.updated.Fields) {
						t.Fatalf("update %d lacks independent old Fields", i)
					}
					if _, ok := oldFields["Id"]; ok {
						t.Fatal("old record already contains publication")
					}
				}
				if !machine.tryPropagateDMLScopeAliasSnapshotMutationsToScope(machine.Globals, mutations) {
					t.Fatal("eligible container rejected")
				}
				applyDMLLegacy(reference, referenceMutations)
				if !reflect.DeepEqual(machine.Globals, reference.Globals) {
					t.Fatal("container publication differs from independent legacy")
				}
				if !sameMethodReturnAliasBatchBacking(retained, machine.Globals["root"]) || !sameSliceHeaderBacking(dmlScopeAliasSequence(retained), dmlScopeAliasSequence(machine.Globals["root"])) {
					t.Fatal("publication replaced retained backing")
				}
				occurrences := dmlAssertPublishedGraph(t, retained, mutations)
				want := n
				if shape == "nested" {
					want = 6 * n
				}
				if occurrences != want {
					t.Fatalf("target occurrences=%d, want %d", occurrences, want)
				}
				mutations[0].updated.Fields["Future"] = String("visible")
				if !reflect.DeepEqual(machine.Globals["root"], machine.Globals["alias"]) {
					t.Fatal("intentional root alias split")
				}
			})
		}
	}
}

func dmlAssertPublishedGraph(t *testing.T, root Value, mutations []dmlScopeAliasMutation) int {
	t.Helper()
	occurrences := 0
	var check func(Value)
	check = func(v Value) {
		if v.Kind == ValueObject && v.Type == "Account" {
			i := int(v.Ref) - 1
			if i < 0 || i >= len(mutations) || !reflect.DeepEqual(v, mutations[i].updated) || !sameMapBacking(v.Fields, mutations[i].updated.Fields) {
				t.Fatalf("incorrect target value/Ref/backing: %+v", v)
			}
			occurrences++
			return
		}
		for _, m := range dmlScopeAliasMaps(v) {
			for _, child := range m {
				check(child)
			}
		}
		for _, child := range dmlScopeAliasSequence(v) {
			check(child)
		}
	}
	check(root)
	return occurrences
}

func TestDMLScopeAliasStaticSharingFallbackMatchesIndependentLegacy(t *testing.T) {
	for _, shape := range []string{"object", "list", "set", "map-values", "map-keys", "nested"} {
		t.Run(shape, func(t *testing.T) {
			machine, mutations, root := dmlContainerFixture(shape, 3)
			reference, referenceMutations, referenceRoot := dmlContainerFixture(shape, 3)
			install := func(m *VM, v Value) {
				m.Classes["Holder"] = Class{Name: "Holder", StaticFields: map[string]Field{"root": {Name: "root", Value: v}}}
			}
			install(machine, root)
			install(reference, referenceRoot)
			observer := &dmlScopeAliasReasonObserver{}
			machine.scopeAliasTraversalObserver = observer
			if machine.tryPropagateDMLScopeAliasSnapshotMutationsToScope(machine.Globals, mutations) {
				t.Fatal("shared static admitted")
			}
			if observer.reason != "external static sharing" {
				t.Fatalf("wrong rejection: %s", observer.reason)
			}
			if !reflect.DeepEqual(machine.Globals, reference.Globals) || !reflect.DeepEqual(machine.Classes["Holder"], reference.Classes["Holder"]) {
				t.Fatal("rejection published before fallback")
			}
			machine.propagateSuccessfulDMLTargetAliases(mutations)
			applyDMLLegacy(reference, referenceMutations)
			if !reflect.DeepEqual(machine.Globals, reference.Globals) || !reflect.DeepEqual(machine.Classes["Holder"], reference.Classes["Holder"]) {
				t.Fatal("ordered fallback differs from legacy")
			}
			static := machine.Classes["Holder"].StaticFields["root"].Value
			want := 3
			if shape == "nested" {
				want = 18
			}
			if got := dmlAssertPublishedGraph(t, static, mutations); got != want {
				t.Fatalf("static target occurrences=%d, want %d", got, want)
			}
			if got := dmlAssertPublishedGraph(t, machine.Globals["root"], mutations); got != want {
				t.Fatalf("caller target occurrences=%d, want %d", got, want)
			}
			mutations[0].updated.Fields["Future"] = String("shared")
			dmlAssertPublishedGraph(t, static, mutations)
			dmlAssertPublishedGraph(t, machine.Globals["root"], mutations)
			if !sameMethodReturnAliasBatchBacking(machine.Globals["root"], static) || !sameSliceHeaderBacking(dmlScopeAliasSequence(machine.Globals["root"]), dmlScopeAliasSequence(static)) {
				t.Fatal("static/caller backing split")
			}
		})
	}
}

func TestDMLScopeAliasNestedPreparationBound(t *testing.T) {
	for _, n := range []int{2, 63, 64, 65, 199, 200} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			machine, mutations, _ := dmlContainerFixture("nested", n)
			plan := machine.prepareDMLScopeAliasPlan(machine.Globals, mutations)
			if plan == nil {
				t.Fatal("nested eligible graph rejected")
			}
			// Two root aliases, repeated map aliases and repeated row aliases must not
			// multiply traversal of a node's descendants. Each unique envelope/list/map/
			// set is prepared once, with three physical target slots per record.
			if len(plan.nodes) != 1+3*n {
				t.Fatalf("nodes=%d, want %d", len(plan.nodes), 1+3*n)
			}
			slots := 0
			for _, s := range plan.slots {
				if len(s) != 3 {
					t.Fatalf("slots per target=%d, want 3", len(s))
				}
				slots += len(s)
			}
			if slots != 3*n || len(plan.maps) != 1+2*n || len(plan.spans) != 2*n {
				t.Fatalf("unbounded prepared graph: nodes=%d maps=%d spans=%d slots=%d", len(plan.nodes), len(plan.maps), len(plan.spans), slots)
			}
			allocations := testing.AllocsPerRun(3, func() {
				if machine.prepareDMLScopeAliasPlan(machine.Globals, mutations) == nil {
					t.Fatal("preparation rejected")
				}
			})
			if allocations > float64(30*n+100) {
				t.Fatalf("nested preparation allocated %.0f objects for %d rows", allocations, n)
			}
		})
	}
}
