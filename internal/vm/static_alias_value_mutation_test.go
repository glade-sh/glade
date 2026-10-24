package vm

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// The same owned observations are exercised through the real anonymous,
// @IsTest, runner and LSP routes by TestStaticRecordAliasLoopNativeRoutes.
func TestStaticRecordAliasValueMutationNativeTwins(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/static_alias_loop/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 25 {
		t.Fatalf("native rows=%d, want 24 plus header", len(rows)-1)
	}
	for line, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1]+"/"+row[2], func(t *testing.T) {
			t.Logf("native twin: internal/apextest/testdata/conformance/static_alias_loop/org.tsv:%d", line+2)
			count := 3
			if row[2] == "empty" {
				count = 0
			}
			machine, list := staticRecordAliasFixture(t, count)
			org := testDataOrg()
			org.APIVersion = row[0]
			machine.SetOrg(&org)
			for i := range list.List {
				list.List[i].Fields["Name"] = String(fmt.Sprintf("Synthetic %d", i/2))
			}
			observed := map[string]any{"size": len(list.List)}
			if count != 0 {
				first := list.List[0]
				if row[2] == "nested" {
					machine.setGraphFieldValue(&list.List[2], "Parent", first)
				}
				if row[2] == "move" {
					// Reorder an already indexed backing, just as remove/add does.
					warmAliasMutationIndex(t, machine, list)
					machine.invalidateFlatObjectAliasIndexes(list.Ref)
					copy(list.List, list.List[1:])
					list.List[len(list.List)-1] = first
				}
				if row[2] == "rejected" {
					_, handled, err := machine.callSObjectMember(first, "put", []Value{String("Absent__c"), String("rejected")})
					if !handled || err == nil {
						t.Fatalf("invalid field: handled=%v err=%v", handled, err)
					}
					var thrown *apexThrowError
					if !errors.As(err, &thrown) {
						t.Fatalf("invalid field did not throw an Apex exception: %v", err)
					}
					observed["exception"] = "System." + exceptionTypeName(thrown.value.Type)
					observed["unchanged"] = list.List[1].Fields["Name"].Text == "Synthetic 0"
				} else {
					recorder := NewPerfRecorder()
					machine.SetPerfRecorder(recorder)
					for i := range list.List {
						previous := list.List[i]
						updated := previous
						updated.Fields = cloneAliasOccurrenceFields(previous.Fields)
						updated.Fields["Description"] = String(fmt.Sprintf("step-%d", i))
						// Intentionally call the value entry directly. Snapshot propagation can
						// otherwise repair an omitted update and hide this path's regressions.
						machine.propagateValueMutationToStatics(previous, updated)
					}
					descriptions := []string{}
					for _, item := range list.List {
						descriptions = append(descriptions, item.Fields["Description"].Text)
						if item.Ref == first.Ref {
							observed["first"] = item.Fields["Description"].Text
						}
					}
					observed["descriptions"] = descriptions
					if row[2] == "nested" {
						observed["parent"] = list.List[2].Fields["Parent"].Fields["Description"].Text
					} else {
						// Functional equality alone also passes with the quadratic fallback.
						raw, _ := json.Marshal(recorder.Snapshot().StaticAlias)
						counters := map[string]uint64{}
						if err := json.Unmarshal(raw, &counters); err != nil {
							t.Fatal(err)
						}
						if counters["replacementVisits"] == 0 || len(machine.flatAliasIndex) == 0 {
							t.Error("value mutation did not use the static occurrence index")
						}
					}
				}
			}
			raw, _ := json.Marshal(observed)
			var got, want any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s value-mutation=%s", row[3], raw)
			}
		})
	}
}

func TestStaticRecordAliasValueMutationBranches(t *testing.T) {
	for _, measured := range []bool{false, true} {
		t.Run(fmt.Sprintf("recorder=%v", measured), func(t *testing.T) {
			t.Run("handled_changed", func(t *testing.T) {
				machine, list := staticRecordAliasFixture(t, 2)
				if measured {
					machine.SetPerfRecorder(NewPerfRecorder())
				}
				previous := list.List[0]
				updated := cloneValuePreserveRefs(previous)
				updated.Fields["Description"] = String("changed")
				machine.propagateValueMutationToStatics(previous, updated)
				if len(machine.flatAliasIndex) == 0 {
					t.Fatal("static value path bypassed occurrence index")
				}
				for _, name := range []string{"StaticRows", "sample.StaticRows"} {
					for _, item := range machine.Classes[name].StaticFields["Rows"].Value.List[:2] {
						if item.Ref != previous.Ref || item.Fields["Description"].Text != "changed" {
							t.Fatal("duplicate alias lost mutation")
						}
					}
				}
			})
			t.Run("handled_miss_forgets_location", func(t *testing.T) {
				machine, list := staticRecordAliasFixture(t, 1)
				if measured {
					machine.SetPerfRecorder(NewPerfRecorder())
				}
				previous := Object("Account")
				updated := cloneValuePreserveRefs(previous)
				updated.Fields["Description"] = String("absent")
				location := machine.canonicalStaticFieldLocation(staticFieldRef{ClassName: "StaticRows", FieldName: "Rows"})
				machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()
				machine.rememberStaticValueRefsInField(previous, location)
				if !machine.staticValueRefFields[previous.Ref].contains(location) {
					t.Fatal("fixture did not publish the canonical stale location")
				}
				machine.propagateValueMutationToStatics(previous, updated)
				if machine.staticValueRefFields[previous.Ref].contains(location) {
					t.Fatal("indexed miss retained stale location")
				}
				if len(machine.flatAliasIndex) == 0 {
					t.Fatal("miss did not take handled branch")
				}
				if _, ok := list.List[0].Fields["Description"]; ok {
					t.Fatal("miss changed unrelated record")
				}
			})
			t.Run("nested_fallback_after_warm_invalidation", func(t *testing.T) {
				machine, list := staticRecordAliasFixture(t, 2)
				if measured {
					machine.SetPerfRecorder(NewPerfRecorder())
				}
				warmAliasMutationIndex(t, machine, list)
				// Retain the list backing while publishing a nested alias. Calling with
				// the same value checks preparation even before the no-change return.
				list.List[2].Fields["Parent"] = list.List[0]
				machine.propagateValueMutationToStatics(list, list)
				if len(machine.flatAliasIndex) != 0 {
					t.Fatal("value mutation retained an unsafe warmed index")
				}
				previous := list.List[0]
				updated := cloneValuePreserveRefs(previous)
				updated.Fields["Description"] = String("nested")
				machine.propagateValueMutationToStatics(previous, updated)
				if got := list.List[2].Fields["Parent"].Fields["Description"].Text; got != "nested" {
					t.Fatalf("nested fallback=%q", got)
				}
			})
		})
	}
}

func TestStaticRecordAliasValueMutationMovedBetweenLists(t *testing.T) {
	machine, source := staticRecordAliasFixture(t, 1)
	warmAliasMutationIndex(t, machine, source)
	first := source.List[0]
	destination := List(first, cloneValuePreserveRefs(first))
	destination.Type = source.Type
	class := machine.Classes["StaticRows"]
	class.StaticFields["Destination"] = Field{Name: "Destination", Type: destination.Type, Static: true, Value: destination}
	machine.storeMutableClassAtAlias("StaticRows", class)
	// Exercise the collection wrapper with a changed header, then the value
	// entry with the moved record. The project route captures cross-list movement
	// in testdata/conformance/static_alias_move/org.tsv in the apextest package.
	emptied := source
	emptied.List = nil
	machine.propagateCollectionMutationToStatics(source, emptied)
	machine.invalidateStaticValueRefs()
	updated := cloneValuePreserveRefs(first)
	updated.Fields["Description"] = String("moved")
	machine.propagateValueMutationToStatics(first, updated)
	if got := machine.Classes["StaticRows"].StaticFields["Rows"].Value; len(got.List) != 0 {
		t.Fatal("source list retained moved records")
	}
	for _, item := range destination.List {
		if item.Fields["Description"].Text != "moved" {
			t.Fatal("destination alias missed mutation")
		}
	}
}

func TestStaticRecordAliasValueMutationGuards(t *testing.T) {
	for _, variant := range []string{"zero_ref", "unchanged", "unreferenced", "empty_locations", "missing_class", "missing_field", "empty_list"} {
		t.Run(variant, func(t *testing.T) {
			machine, list := staticRecordAliasFixture(t, 1)
			previous := list.List[0]
			updated := cloneValuePreserveRefs(previous)
			updated.Fields["Description"] = String("ignored")
			machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()
			switch variant {
			case "zero_ref":
				previous.Ref = 0
			case "unchanged":
				updated = previous
			case "unreferenced":
				previous = Object("Account")
			case "empty_locations":
				machine.staticValueRefFields[previous.Ref] = staticFieldRefSet{}
			case "missing_class", "missing_field", "empty_list":
				location := staticFieldRef{ClassName: "StaticRows", FieldName: "Rows"}
				if variant == "missing_class" {
					location.ClassName = "Absent"
				}
				if variant == "missing_field" {
					location.FieldName = "Absent"
				}
				locations := staticFieldRefSet{}
				locations.add(location)
				machine.staticValueRefFields[previous.Ref] = locations
				if variant == "empty_list" {
					class := machine.Classes["StaticRows"]
					field := class.StaticFields["Rows"]
					field.Value = List()
					class.StaticFields["Rows"] = field
					machine.storeMutableClassAtAlias("StaticRows", class)
				}
			}
			machine.propagateValueMutationToStatics(previous, updated)
			for _, item := range list.List {
				if _, ok := item.Fields["Description"]; ok {
					t.Fatal("guard changed unrelated record")
				}
			}
		})
	}
}

func TestStaticRecordAliasValueMutationCallers(t *testing.T) {
	t.Run("collection_scope_and_statics", func(t *testing.T) {
		machine, list := staticRecordAliasFixture(t, 1)
		machine.Globals["alias"] = list
		updated := list
		updated.List = append(append([]Value(nil), list.List...), Object("Account"))
		machine.propagateCollectionMutation(list, updated)
		if len(machine.Globals["alias"].List) != 3 || len(machine.Classes["StaticRows"].StaticFields["Rows"].Value.List) != 3 {
			t.Fatal("collection caller failed to publish the new header")
		}
	})
	t.Run("interpreted_method_return", func(t *testing.T) {
		machine, list := staticRecordAliasFixture(t, 1)
		program, err := CompileAnonymous("first = second; first.Description = 'method';")
		if err != nil {
			t.Fatal(err)
		}
		method := Method{Name: "StaticRows.updateAlias", ClassName: "StaticRows", ReturnType: "void", IsStatic: true,
			Access: "public", Params: []Param{{Name: "first", Type: "Account"}, {Name: "second", Type: "Account"}}, Program: program}
		if _, err := machine.callMethodWithReceiver(method, Null, []Value{Object("Account"), list.List[0]}, &Result{}); err != nil {
			t.Fatal(err)
		}
		for _, item := range machine.Classes["StaticRows"].StaticFields["Rows"].Value.List {
			if item.Fields["Description"].Text != "method" {
				t.Fatal("method return lost static alias")
			}
		}
	})
}

func TestStaticRecordAliasPassiveMethodReturn(t *testing.T) {
	machine, list := staticRecordAliasFixture(t, 1)
	for i := range list.List {
		list.List[i].Fields["Description"] = String("initial")
	}
	warmAliasMutationIndex(t, machine, list)
	// Preserve reference identity but detach the receiver backing. The duplicate
	// static slots can only see the setter through method-return propagation.
	receiver := cloneValuePreserveRefs(list.List[0])
	recorder := NewPerfRecorder()
	machine.SetPerfRecorder(recorder)
	method := Method{Name: "StaticRows.setDescription", ClassName: "StaticRows", ReturnType: "void",
		Access: "public", Modifiers: []string{"passive-generated"}, Params: []Param{{Name: "value", Type: "String"}}}
	if _, err := machine.callMethodWithReceiver(method, receiver, []Value{String("passive")}, &Result{}); err != nil {
		t.Fatal(err)
	}
	stats := recorder.Snapshot().StaticAlias
	raw, err := json.Marshal(stats)
	if err != nil {
		t.Fatal(err)
	}
	var counters map[string]uint64
	if err := json.Unmarshal(raw, &counters); err != nil {
		t.Fatal(err)
	}
	if stats.Calls == 0 || counters["replacementVisits"] == 0 {
		t.Fatalf("passive return never propagated to the static index: %+v", stats)
	}
	t.Logf("passive static calls=%d replacement visits=%d", stats.Calls, counters["replacementVisits"])
	for _, item := range machine.Classes["StaticRows"].StaticFields["Rows"].Value.List {
		if got := item.Fields["Description"].Text; got != "passive" {
			t.Fatalf("passive setter lost static alias: %q", got)
		}
	}
}
