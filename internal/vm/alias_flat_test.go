package vm

import (
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestAliasFlatObjectRootsMatchDetachedLegacy(t *testing.T) {
	target := aliasFlatTestObject("Account", map[string]Value{"Name": String("before")})
	other := aliasFlatTestObject("Account", map[string]Value{"Name": String("other"), "Empty": Null})
	scalarSameRef := String("scalar")
	scalarSameRef.Ref = target.Ref
	previous := snapshotAlias(target)
	updated := cloneValuePreserveRefs(target)
	updated.Fields["Name"] = String("after")

	setRoot := Set()
	setRoot.Type = "Set<Object>"
	// Construct duplicate refs explicitly: the alias walk must handle every
	// backing slot even where a Set's normal insertion path would deduplicate.
	setRoot.Set = []Value{target, cloneValuePreserveRefs(target), other, scalarSameRef}
	targetWithRelationship := cloneValuePreserveRefs(target)
	targetWithRelationship.Fields["Parent"] = other
	cases := []struct {
		name    string
		root    Value
		changed bool
	}{
		{"list duplicates detached", List(target, cloneValuePreserveRefs(target), other), true},
		{"scalar same ref", List(scalarSameRef, Null, Int(3), target), true},
		{"target relationship stops traversal", List(targetWithRelationship, other), true},
		{"set duplicate slots", setRoot, true},
		{"absent terminal", List(other, scalarSameRef, Null), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fast := cloneValuePreserveRefs(tc.root)
			legacy := cloneValuePreserveRefs(tc.root)
			before := cloneValuePreserveRefs(tc.root)
			if fast.Kind == ValueList && len(fast.List) > 0 && sameSliceBacking(fast.List, legacy.List) {
				t.Fatal("test requires detached list backings")
			}
			var probe scopeAliasProbe
			handled, changed := tryFlatObjectAliasRoot(fast, previous, updated, &probe)
			if !handled || changed != tc.changed {
				t.Fatalf("handled=%v changed=%v, want handled=true changed=%v", handled, changed, tc.changed)
			}
			want, legacyChanged := replaceValueAliasRef(legacy, previous, updated, make(map[uint64]bool))
			if legacyChanged != changed || !reflect.DeepEqual(fast, want) {
				t.Fatalf("shortcut differs from legacy: changed=%v legacyChanged=%v", changed, legacyChanged)
			}
			if !reflect.DeepEqual(tc.root, before) {
				t.Fatal("detached propagation changed the original fixture")
			}
			if probe.shortcutVisits == 0 {
				t.Fatal("expected work to be counted")
			}
		})
	}
}

func TestAliasFlatObjectFallbackDoesNotPartiallyWrite(t *testing.T) {
	target := aliasFlatTestObject("Account", map[string]Value{"Name": String("before")})
	updated := cloneValuePreserveRefs(target)
	updated.Fields["Name"] = String("after")
	holder := aliasFlatTestObject("Holder", map[string]Value{"record": cloneValuePreserveRefs(target)})
	cycle := aliasFlatTestObject("Cycle", map[string]Value{})
	cycle.Fields["self"] = cycle
	mapWithNestedKey := Map()
	mapWithNestedKey.Map["key"] = target
	mapWithNestedKey.MapKeys["key"] = holder
	cases := []struct {
		name string
		root Value
	}{
		{"nested object", List(target, holder)},
		{"nested list", List(target, List(target))},
		{"nested set", List(target, Set(target))},
		{"map key holder", mapWithNestedKey},
		{"cyclic object", List(target, cycle)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fast := cloneValuePreserveRefs(tc.root)
			before := cloneValuePreserveRefs(fast)
			handled, changed := tryFlatObjectAliasRoot(fast, snapshotAlias(target), updated, nil)
			if handled || changed {
				t.Fatalf("unsafe root accepted: handled=%v changed=%v", handled, changed)
			}
			if !reflect.DeepEqual(fast, before) {
				t.Fatal("fallback changed a slot before completing proof")
			}
			got, gotChanged := replaceValueAliasRef(fast, snapshotAlias(target), updated, make(map[uint64]bool))
			want, wantChanged := replaceValueAliasRef(cloneValuePreserveRefs(tc.root), snapshotAlias(target), updated, make(map[uint64]bool))
			if gotChanged != wantChanged || !reflect.DeepEqual(got, want) {
				t.Fatal("fallback differs from unmodified legacy propagation")
			}
		})
	}
}

func TestAliasFlatObjectRechecksSameRefDivergentBackings(t *testing.T) {
	target := aliasFlatTestObject("Account", map[string]Value{"Name": String("before")})
	other := aliasFlatTestObject("Account", map[string]Value{"Name": String("other")})
	first := List(other)
	second := cloneValuePreserveRefs(first)
	second.List[0] = cloneValuePreserveRefs(target)
	if first.Ref != second.Ref || sameSliceBacking(first.List, second.List) {
		t.Fatal("test needs matching refs and different backing")
	}
	updated := cloneValuePreserveRefs(target)
	updated.Fields["Name"] = String("after")
	for _, root := range []Value{first, second} {
		want := cloneValuePreserveRefs(root)
		want, wantChanged := replaceValueAliasRef(want, snapshotAlias(target), updated, make(map[uint64]bool))
		handled, changed := tryFlatObjectAliasRoot(root, snapshotAlias(target), updated, nil)
		if !handled || changed != wantChanged || !reflect.DeepEqual(root, want) {
			t.Fatal("a previous same-ref backing affected this root's result")
		}
	}
	if first.List[0].Ref != other.Ref || second.List[0].Fields["Name"].Text != "after" {
		t.Fatal("wrong divergent backing was changed")
	}
}

func TestAliasFlatObjectCountsPreflightAndReplacement(t *testing.T) {
	target := aliasFlatTestObject("Account", map[string]Value{"Name": String("before")})
	other := aliasFlatTestObject("Account", map[string]Value{"Name": String("other")})
	root := List(other, target, cloneValuePreserveRefs(target))
	var probe scopeAliasProbe
	handled, changed := tryFlatObjectAliasRoot(root, snapshotAlias(target), target, &probe)
	// One root + three preflight slots + one non-target field + three
	// replacement slot inspections. Target fields are never visited.
	if !handled || !changed || probe.shortcutVisits != 8 {
		t.Fatalf("handled=%v changed=%v visits=%d, want true/true/8", handled, changed, probe.shortcutVisits)
	}
}

func TestAliasFlatObjectScalarMarkerMaps(t *testing.T) {
	for _, mode := range []string{"scalar map", "scalar list", "scalar set", "typed object value", "typed object key", "generic object value", "generic object key", "nested collection", "cyclic map"} {
		t.Run(mode, func(t *testing.T) {
			target := aliasFlatTestObject("Account", map[string]Value{"Name": String("before")})
			updated := cloneValuePreserveRefs(target)
			updated.Fields["Name"] = String("after")
			marker := Map()
			// Match legacy field pruning even for malformed scalar-typed maps.
			marker.Type = "Map<String,Boolean>"
			marker.Map["marker"] = Bool(true)
			marker.MapKeys["marker"] = String("marker")
			accepted := false
			switch mode {
			case "scalar map":
				accepted = true
			case "scalar list":
				marker = List(Bool(true), Null, String("scalar"))
				accepted = true
			case "scalar set":
				marker = Set(Bool(true), Null, String("scalar"))
				accepted = true
			case "typed object value":
				marker.Map["marker"] = cloneValuePreserveRefs(target)
				accepted = valueCannotContainAliasRef(marker, target.Ref, ValueObject)
			case "typed object key":
				marker.MapKeys["marker"] = cloneValuePreserveRefs(target)
				accepted = valueCannotContainAliasRef(marker, target.Ref, ValueObject)
			case "generic object value":
				marker.Type = "Map<String,Object>"
				marker.Map["marker"] = cloneValuePreserveRefs(target)
			case "generic object key":
				marker.Type = "Map<Object,Object>"
				marker.MapKeys["marker"] = cloneValuePreserveRefs(target)
			case "nested collection":
				marker.Type = "Map<String,Object>"
				marker.Map["marker"] = List(Bool(true))
			case "cyclic map":
				marker.Type = "Map<String,Object>"
				marker.Map["marker"] = marker
			}
			holder := aliasFlatTestObject("Account", map[string]Value{"arbitrary": marker})
			root := List(cloneValuePreserveRefs(target), holder)
			before := cloneValuePreserveRefs(root)
			handled, changed := tryFlatObjectAliasRoot(root, snapshotAlias(target), updated, nil)
			if handled != accepted || changed != accepted {
				t.Fatalf("handled=%v changed=%v, want both %v", handled, changed, accepted)
			}
			if !accepted {
				if !reflect.DeepEqual(root, before) {
					t.Fatal("rejected marker changed a direct target before proof completed")
				}
				return
			}
			want, legacyChanged := replaceValueAliasRef(before, snapshotAlias(target), updated, make(map[uint64]bool))
			if !legacyChanged || !reflect.DeepEqual(root, want) {
				t.Fatal("accepted scalar marker differs from legacy replacement")
			}
			if mode == "typed object value" && root.List[1].Fields["arbitrary"].Map["marker"].Fields["Name"].Text != "before" {
				t.Fatal("legacy-pruned value alias was changed")
			}
			if mode == "typed object key" && root.List[1].Fields["arbitrary"].MapKeys["marker"].Fields["Name"].Text != "before" {
				t.Fatal("legacy-pruned key alias was changed")
			}
		})
	}
}

func TestAliasFlatObjectPreservesLegacyFieldTypePruning(t *testing.T) {
	target := aliasFlatTestObject("Account", map[string]Value{"Name": String("before")})
	updated := cloneValuePreserveRefs(target)
	updated.Fields["Name"] = String("after")
	list := List(cloneValuePreserveRefs(target))
	list.Type = "List<String>"
	set := Set(cloneValuePreserveRefs(target))
	set.Type = "Set<Integer>"
	mapped := Map()
	mapped.Type = "Map<String,Boolean>"
	mapped.Map["target"] = cloneValuePreserveRefs(target)
	mapped.MapKeys["target"] = cloneValuePreserveRefs(target)
	for _, field := range []Value{list, set, mapped} {
		t.Run(field.Type, func(t *testing.T) {
			if !valueCannotContainAliasRef(field, target.Ref, ValueObject) {
				t.Fatal("test field must exercise legacy declared-type pruning")
			}
			holder := aliasFlatTestObject("Holder", map[string]Value{"field": cloneValuePreserveRefs(field)})
			root := List(cloneValuePreserveRefs(target), holder)
			legacy := cloneValuePreserveRefs(root)
			beforeField := cloneValuePreserveRefs(root.List[1].Fields["field"])
			var probe scopeAliasProbe
			handled, changed := tryFlatObjectAliasRoot(root, snapshotAlias(target), updated, &probe)
			want, legacyChanged := replaceValueAliasRef(legacy, snapshotAlias(target), updated, make(map[uint64]bool))
			if !handled || !changed || !legacyChanged || !reflect.DeepEqual(root, want) {
				t.Fatal("field-pruning shortcut differs from detached legacy replacement")
			}
			if root.List[0].Fields["Name"].Text != "after" || !reflect.DeepEqual(root.List[1].Fields["field"], beforeField) {
				t.Fatal("direct update or preserved hidden field contents differ from legacy")
			}
			// Root, two preflight slots, one pruned field, two replacement
			// slots. No malformed hidden field contents were inspected.
			if probe.shortcutVisits != 6 {
				t.Fatalf("shortcutVisits=%d, want 6", probe.shortcutVisits)
			}
		})
	}
}

func TestAliasFlatObjectPreservesLegacyRootPruning(t *testing.T) {
	target := aliasFlatTestObject("Account", map[string]Value{"Name": String("before")})
	updated := cloneValuePreserveRefs(target)
	updated.Fields["Name"] = String("after")
	list := List(cloneValuePreserveRefs(target))
	list.Type = "List<String>"
	set := Set(cloneValuePreserveRefs(target))
	set.Type = "Set<Integer>"
	mapped := Map()
	mapped.Type = "Map<String,Boolean>"
	mapped.Map["target"] = cloneValuePreserveRefs(target)
	mapped.MapKeys["target"] = String("target")
	for _, root := range []Value{list, set, mapped, List(), Map()} {
		before := cloneValuePreserveRefs(root)
		if !valueCannotContainAliasRef(root, target.Ref, ValueObject) {
			t.Fatalf("test root %q must exercise existing root pruning", root.Type)
		}
		handled, changed := tryFlatObjectAliasRoot(root, snapshotAlias(target), updated, nil)
		if handled || changed || !reflect.DeepEqual(root, before) {
			t.Fatalf("root %q overrode legacy pruning: handled=%v changed=%v", root.Type, handled, changed)
		}
	}
}

func TestAliasFlatObjectPreservesLegacyMapKeyPruning(t *testing.T) {
	target := aliasFlatTestObject("Account", map[string]Value{"Name": String("before")})
	updated := cloneValuePreserveRefs(target)
	updated.Fields["Name"] = String("after")
	root := Map()
	root.Type = "Map<String,Object>"
	root.Map["target"] = cloneValuePreserveRefs(target)
	root.MapKeys["target"] = cloneValuePreserveRefs(target)
	before := cloneValuePreserveRefs(root)
	if valueCannotContainAliasRef(root, target.Ref, ValueObject) || !mapKeyTypeCannotContainAlias(root.Type, snapshotAlias(target)) {
		t.Fatal("test needs eligible root with legacy key pruning")
	}
	handled, changed := tryFlatObjectAliasRoot(root, snapshotAlias(target), updated, nil)
	if handled || changed || !reflect.DeepEqual(root, before) {
		t.Fatal("malformed nonprimitive key overrode pruning or partially wrote")
	}
	got, gotChanged := replaceValueAliasRef(root, snapshotAlias(target), updated, make(map[uint64]bool))
	want, wantChanged := replaceValueAliasRef(before, snapshotAlias(target), updated, make(map[uint64]bool))
	if !gotChanged || !wantChanged || !reflect.DeepEqual(got, want) {
		t.Fatal("fallback differs from legacy replacement")
	}
	if got.Map["target"].Fields["Name"].Text != "after" || got.MapKeys["target"].Fields["Name"].Text != "before" {
		t.Fatal("legacy value replacement and key pruning were not preserved")
	}
}

func aliasFlatTestObject(typeName string, fields map[string]Value) Value {
	value := Object(typeName)
	value.Fields = fields
	return value
}

func TestAliasFlatObjectScopeMapRetainsLegacy(t *testing.T) {
	machine := New(nil)
	target := aliasFlatTestObject("Account", map[string]Value{"Name": String("before")})
	root := Map()
	root.Type = "Map<String,Object>"
	root.Map["target"] = cloneValuePreserveRefs(target)
	root.MapKeys["target"] = String("target")
	scope := map[string]Value{"map": root}
	updated := cloneValuePreserveRefs(target)
	updated.Fields["Name"] = String("after")
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(target), updated)
	stats := recorder.Snapshot().ScopeAlias
	if stats.RecursiveVisits == 0 || stats.ShortcutVisits != 0 {
		t.Fatalf("map did not retain legacy traversal: %#v", stats)
	}
	if got := scope["map"].Map["target"].Fields["Name"]; got.Text != "after" {
		t.Fatalf("map alias stale: %v", got)
	}
}

// These deliberately separate all backing maps, including duplicate elements.
// Ref equality, rather than shared Go maps or read-time refresh, must propagate.
func TestAliasFlatDetachedShapes(t *testing.T) {
	sizes := []int{6}
	if os.Getenv("GLADE_ALIAS_MEASURE") == "1" {
		sizes = []int{200, 1000, 4000}
	}
	for _, shape := range []string{"A", "B", "C", "D", "E", "F", "G"} {
		for _, n := range sizes {
			t.Run(fmt.Sprintf("%s/%d", shape, n), func(t *testing.T) {
				machine := New(nil)
				org := testDataOrg()
				machine.SetOrg(&org)
				rows := make([]Value, n)
				for i := range rows {
					rows[i] = Object("Account")
					rows[i].Fields["Name"] = String("before")
				}
				if shape == "F" {
					for i := 1; i < n; i += 2 {
						rows[i] = rows[i-1]
					}
				}
				list := List(rows...)
				list.Type = "List<Account>"
				scope := map[string]Value{"es": cloneValuePreserveRefs(list)}
				// A second snapshot with the same list Ref must be checked separately;
				// logical collection identity does not prove shared physical backing.
				scope["copy"] = cloneValuePreserveRefs(list)
				if shape == "D" {
					others := make([]Value, n)
					for i := range others {
						others[i] = Object("Account")
						others[i].Fields["Name"] = String("other")
					}
					scope["other"] = List(others...)
				}
				if shape == "E" {
					m := Value{Kind: ValueMap, Type: "Map<Integer,Account>", Ref: newValueRef(), Map: map[string]Value{}, MapKeys: map[string]Value{}}
					for i, v := range rows {
						k := mapKey(Int(int64(i)))
						m.Map[k] = cloneValuePreserveRefs(v)
						m.MapKeys[k] = Int(int64(i))
					}
					scope["m"] = m
				}
				if shape == "G" {
					parent := Object("Account")
					parent.Fields["Name"] = String("before")
					for i := range rows {
						rows[i].Fields["Parent"] = cloneValuePreserveRefs(parent)
					}
					list.List = rows
					scope["es"] = cloneValuePreserveRefs(list)
					scope["copy"] = cloneValuePreserveRefs(list)
					for i := range rows {
						rows[i] = parent
					}
				}
				if sameSliceBacking(scope["es"].List, scope["copy"].List) ||
					sameMapBacking(scope["es"].List[0].Fields, scope["copy"].List[0].Fields) {
					t.Fatal("shape requires detached root and field backings")
				}
				recorder := NewPerfRecorder()
				machine.SetPerfRecorder(recorder)
				expected := make(map[uint64]string)
				for j := 0; j < n; j++ {
					i := j
					if shape == "C" {
						i = n - j - 1
					}
					original := rows[i]
					updated := cloneValuePreserveRefs(original)
					expected[original.Ref] = fmt.Sprintf("x%d", j)
					updated.Fields["Name"] = String(expected[original.Ref])
					machine.propagateAliasSnapshotToScope(scope, snapshotAlias(original), updated)
					// Check every occurrence immediately. In F this catches a dropped
					// duplicate before its own later iteration could conceal the loss.
					for _, name := range []string{"es", "copy"} {
						for slot, v := range scope[name].List {
							if shape == "G" {
								v = v.Fields["Parent"]
							}
							if v.Ref == original.Ref && v.Fields["Name"].Text != expected[original.Ref] {
								t.Fatalf("iteration %d: %s[%d] matching ref not propagated", j, name, slot)
							}
						}
					}
					if shape == "E" {
						for key, v := range scope["m"].Map {
							if v.Ref == original.Ref && v.Fields["Name"].Text != expected[original.Ref] {
								t.Fatalf("iteration %d: map[%s] matching ref not propagated", j, key)
							}
						}
					}
				}
				for _, name := range []string{"es", "copy"} {
					for i, v := range scope[name].List {
						if shape == "G" {
							v = v.Fields["Parent"]
						}
						if v.Fields["Name"].Text != expected[v.Ref] {
							t.Fatalf("%s[%d] not propagated: %#v", name, i, v)
						}
					}
				}
				if shape == "D" {
					for _, v := range scope["other"].List {
						if v.Fields["Name"].Text != "other" {
							t.Fatal("unrelated changed")
						}
					}
				}
				if shape == "E" {
					for _, v := range scope["m"].Map {
						if v.Fields["Name"].Text != expected[v.Ref] {
							t.Fatal("map alias stale")
						}
					}
				}
				stats := recorder.Snapshot().ScopeAlias
				if shape == "G" {
					if stats.RecursiveVisits == 0 {
						t.Fatal("nested holders must fall back to legacy recursion")
					}
				} else if stats.ShortcutVisits == 0 {
					t.Fatal("flat list roots must count shortcut work")
				}
				if shape == "E" && stats.RecursiveVisits == 0 {
					t.Fatal("map root must retain legacy recursion")
				}
				if shape != "E" && shape != "G" && stats.RecursiveVisits != 0 {
					t.Fatalf("verified flat roots used legacy recursion: %#v", stats)
				}
				t.Logf("shape=%s n=%d RecursiveVisits=%d ShortcutVisits=%d", shape, n, stats.RecursiveVisits, stats.ShortcutVisits)
			})
		}
	}
}

func TestAliasFlatMapRootsAlwaysFallBack(t *testing.T) {
	target := aliasFlatTestObject("Account", map[string]Value{"Name": String("before")})
	updated := cloneValuePreserveRefs(target)
	updated.Fields["Name"] = String("after")
	for _, position := range []string{"value", "key", "both"} {
		t.Run(position, func(t *testing.T) {
			root := Map()
			root.Type = "Map<Object,Object>"
			root.Map["entry"] = String("value")
			root.MapKeys["entry"] = String("key")
			if position == "value" || position == "both" {
				root.Map["entry"] = cloneValuePreserveRefs(target)
			}
			if position == "key" || position == "both" {
				root.MapKeys["entry"] = cloneValuePreserveRefs(target)
			}
			before := cloneValuePreserveRefs(root)
			var probe scopeAliasProbe
			handled, changed := tryFlatObjectAliasRoot(root, snapshotAlias(target), updated, &probe)
			if handled || changed || probe.shortcutVisits != 0 || !reflect.DeepEqual(root, before) {
				t.Fatalf("Map root entered shortcut: handled=%v changed=%v visits=%d", handled, changed, probe.shortcutVisits)
			}
			got, changed := replaceValueAliasRef(root, snapshotAlias(target), updated, make(map[uint64]bool))
			if !changed {
				t.Fatal("legacy fallback missed map occurrence")
			}
			if position != "key" && got.Map["entry"].Fields["Name"].Text != "after" {
				t.Fatal("legacy fallback missed value")
			}
			if position != "value" && got.MapKeys["entry"].Fields["Name"].Text != "after" {
				t.Fatal("legacy fallback missed key")
			}
		})
	}
}

func TestAliasFlatSetScopeUsesShortcut(t *testing.T) {
	target := aliasFlatTestObject("Account", map[string]Value{"Name": String("before")})
	other := aliasFlatTestObject("Account", map[string]Value{"Name": String("other")})
	root := Set(target, other)
	root.Type = "Set<Account>"
	first := cloneValuePreserveRefs(root)
	second := cloneValuePreserveRefs(root)
	if sameSliceBacking(first.Set, second.Set) || sameMapBacking(first.Set[0].Fields, second.Set[0].Fields) {
		t.Fatal("test requires separate set and field backing")
	}
	scope := map[string]Value{"first": first, "second": second}
	updated := cloneValuePreserveRefs(target)
	updated.Fields["Name"] = String("after")
	machine := New(nil)
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)
	machine.propagateAliasSnapshotToScope(scope, snapshotAlias(target), updated)
	for name, value := range scope {
		want, changed := replaceValueAliasRef(cloneValuePreserveRefs(root), snapshotAlias(target), updated, make(map[uint64]bool))
		if !changed || !reflect.DeepEqual(value, want) {
			t.Fatalf("%s differs from detached legacy Set propagation", name)
		}
	}
	stats := recorder.Snapshot().ScopeAlias
	if stats.RecursiveVisits != 0 || stats.ShortcutVisits == 0 || stats.ReplacedRoots != 2 {
		t.Fatalf("Set roots did not use shortcut: %#v", stats)
	}
}

func TestAliasFlatRechecksRootAfterNestedHolderInsertion(t *testing.T) {
	for _, kind := range []ValueKind{ValueList, ValueSet} {
		t.Run(string(kind), func(t *testing.T) {
			target := aliasFlatTestObject("Account", map[string]Value{"Name": String("before")})
			other := aliasFlatTestObject("Account", map[string]Value{"Name": String("other")})
			root := List(cloneValuePreserveRefs(target), other)
			if kind == ValueSet {
				root = Set(cloneValuePreserveRefs(target), other)
			}
			rootRef := root.Ref
			scope := map[string]Value{"root": root}
			machine := New(nil)
			recorder := NewPerfRecorder()
			machine.SetPerfRecorder(recorder)
			firstUpdate := cloneValuePreserveRefs(target)
			firstUpdate.Fields["Name"] = String("first")
			machine.propagateAliasSnapshotToScope(scope, snapshotAlias(target), firstUpdate)
			firstStats := recorder.Snapshot().ScopeAlias
			if firstStats.ShortcutVisits == 0 || firstStats.RecursiveVisits != 0 || firstStats.ReplacedRoots != 1 {
				t.Fatalf("first flat root did not use shortcut: %#v", firstStats)
			}

			// Retain the root identity and backing, but replace its direct target
			// with a holder. The target now exists only inside a different object.
			// Its fields are detached from firstUpdate so shared maps cannot hide
			// a dropped propagation or an unsafe retained eligibility decision.
			holder := aliasFlatTestObject("Holder", map[string]Value{"record": cloneValuePreserveRefs(firstUpdate)})
			if sameMapBacking(holder.Fields["record"].Fields, firstUpdate.Fields) {
				t.Fatal("holder must contain a detached target snapshot")
			}
			root = scope["root"]
			if kind == ValueList {
				root.List[0] = holder
			} else {
				root.Set[0] = holder
			}
			scope["root"] = root
			if root.Ref != rootRef {
				t.Fatal("test must reuse the flat root's ref")
			}
			secondUpdate := cloneValuePreserveRefs(firstUpdate)
			secondUpdate.Fields["Name"] = String("second")
			before := cloneValuePreserveRefs(root)
			handled, changed := tryFlatObjectAliasRoot(root, snapshotAlias(firstUpdate), secondUpdate, nil)
			if handled || changed || !reflect.DeepEqual(root, before) {
				t.Fatalf("changed root reused flat eligibility or wrote before fallback: handled=%v changed=%v", handled, changed)
			}
			machine.propagateAliasSnapshotToScope(scope, snapshotAlias(firstUpdate), secondUpdate)
			secondStats := recorder.Snapshot().ScopeAlias
			if secondStats.RecursiveVisits <= firstStats.RecursiveVisits || secondStats.ReplacedRoots != 2 {
				t.Fatalf("nested root did not fall back to legacy propagation: %#v", secondStats)
			}
			got := scope["root"].List
			if kind == ValueSet {
				got = scope["root"].Set
			}
			if got[0].Fields["record"].Fields["Name"].Text != "second" || got[1].Fields["Name"].Text != "other" {
				t.Fatal("nested target or unrelated sibling has the wrong value")
			}
			if firstUpdate.Fields["Name"].Text != "first" || target.Fields["Name"].Text != "before" {
				t.Fatal("detached source snapshots were changed")
			}
		})
	}
}
