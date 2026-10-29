package vm

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// Ownership controls preserve the local read contract anchored by
// testdata/describe_sharing/org.tsv:2,4,5,7 and describe_read_workload/org.tsv.
// Coercion representation and allocation checks are implementation controls,
// not new claims about Salesforce mutation behavior.
func TestFieldMapCOWStorageAndReadAllocation(t *testing.T) {
	var allocations []float64
	for _, count := range []int{8, 512} {
		machine := New(nil)
		org := describeOverlayOrg(count)
		machine.SetOrg(&org)
		wrapper := fieldMapWrapper(t, machine, "pkg__Widget__c")
		first := fieldMapGet(t, machine, wrapper)
		second := fieldMapGet(t, machine, wrapper)
		source := wrapper.Fields["map"]
		if !first.mapBackingShared || !second.mapBackingShared || !source.mapBackingShared ||
			!sameMapBacking(first.Map, source.Map) || !sameMapBacking(second.MapKeys, source.MapKeys) ||
			&first.MapOrder[0] != &source.MapOrder[0] {
			t.Errorf("%d fields: independent holders did not share frozen storage", count)
		}
		if first.Ref == second.Ref || first.Ref == source.Ref || second.Ref == source.Ref {
			t.Fatal("separate getMap calls must allocate independent identities")
		}
		allocations = append(allocations, testing.AllocsPerRun(10, func() {
			got := fieldMapGet(t, machine, wrapper)
			if len(got.Map) != len(source.Map) {
				panic("field map lost entries")
			}
		}))
	}
	t.Logf("getMap allocations/read: 8 fields %.0f; 512 fields %.0f", allocations[0], allocations[1])
	if allocations[1] > allocations[0]+2 || allocations[1] > 24 {
		t.Fatal("read allocations scale with all fields rather than the returned holder")
	}
}

// Keep aliases outside VM scopes: argument evaluation can retain such Values
// while a later argument changes the representation of the same holder.
func TestFieldMapCOWRetainedArgumentObservations(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		for _, primed := range []bool{false, true} {
			for _, target := range []string{"Map<String,Object>", "Map<String,Schema.DescribeFieldResult>"} {
				t.Run(fmt.Sprintf("namespace=%v/primed=%v/%s", namespaced, primed, target), func(t *testing.T) {
					machine := describeShareUnprimedFor(namespaced)
					if primed {
						base, template := describeSharePrimedBaseFor(t, namespaced)
						machine = describeShareClone(base, template)
					}
					wrapper := fieldMapWrapper(t, machine, "Account")
					first := fieldMapGet(t, machine, wrapper)
					second := fieldMapGet(t, machine, wrapper)
					independentBefore, _ := json.Marshal(second)
					eager := privateDescribeCollection(first)
					_, err := machine.coerceAssignable(target, eager)
					if err != nil {
						t.Fatal(err)
					}
					converted, err := machine.coerceAssignable(target, first)
					if err != nil || converted.Ref != first.Ref {
						t.Fatalf("coercion lost holder identity: %v", err)
					}
					for _, method := range []string{"get", "values", "keySet", "size", "clone"} {
						var args []Value
						if method == "get" {
							args = []Value{String("Name")}
						}
						actual, _, err := machine.callValueMember("", first, method, args, &Result{})
						want, _, eagerErr := machine.callValueMember("", eager, method, args, &Result{})
						actualJSON, _ := json.Marshal(actual)
						wantJSON, _ := json.Marshal(want)
						if fmt.Sprint(err) != fmt.Sprint(eagerErr) || string(actualJSON) != string(wantJSON) {
							t.Fatalf("retained argument %s differs from eager coercion: %v / %v", method, err, eagerErr)
						}
					}
					actualJSON, err := json.Marshal(first)
					wantJSON, eagerErr := json.Marshal(eager)
					if err != nil || eagerErr != nil || string(actualJSON) != string(wantJSON) || first.String() != eager.String() || approxValueSize(first) != approxValueSize(eager) {
						t.Fatal("retained argument serialization, display, or logical heap differs")
					}
					independentAfter, _ := json.Marshal(second)
					if string(independentBefore) != string(independentAfter) || !reflect.DeepEqual(second.Map, wrapper.Fields["map"].Map) {
						t.Fatal("coercion changed an independent holder or source")
					}
				})
			}
		}
	}
}
