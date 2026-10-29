package vm

import (
	"encoding/csv"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDescribeReadWorkloadNativeConformance(t *testing.T) {
	source, err := os.ReadFile("testdata/describe_read_workload/probe.apex")
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open("testdata/describe_read_workload/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 19 || !reflect.DeepEqual(rows[0], []string{"source_api", "row", "observation"}) {
		t.Fatal("incomplete native read matrix")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var want []string
			for _, row := range rows[1:] {
				if row[0] == api {
					want = append(want, "ROW\t"+row[1]+"\t"+row[2])
				}
			}
			if len(want) != 9 {
				t.Fatal("missing native read observations")
			}
			program, err := CompileAnonymousWithOptions(string(source), CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			for _, name := range []string{"Account", "Contact", "Opportunity", "Task", "User"} {
				storage.EnsureStandardObject(&org, name)
			}
			template := storage.NewRuntimeTemplate(org)
			PrimeRuntimeTemplateSchema(&template)
			base := New(nil)
			base.PrimeMetadataSchema(&template.Org)
			for range 2 {
				machine := base.CloneRuntime(nil)
				clone := template.CloneRuntimeOrg()
				machine.SetRuntimeTemplateOrg(&clone)
				result, err := machine.Execute(program)
				if err != nil || !reflect.DeepEqual(result.Debug, want) {
					t.Fatalf("native=%v; local=%v; error=%v", want, result.Debug, err)
				}
			}
			t.Run("allocation", testTypedGlobalDescribeAllocation)
		})
	}
}

func testTypedGlobalDescribeAllocation(t *testing.T) {
	for _, count := range []int{1, 128} {
		t.Run(fmt.Sprintf("objects=%d", count), func(t *testing.T) {
			org := storage.NewOrgState()
			for i := range count {
				name := fmt.Sprintf("Parcel%03d__c", i)
				org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name}}
			}
			machine := New(nil)
			machine.SetOrg(&org)
			raw := machine.schemaGlobalDescribe()
			keys := orderedValueMapKeys(raw)
			bind := func() {
				view, err := machine.coerceAssignable("Map<String,Schema.SObjectType>", machine.globalDescribeMapValue())
				if err != nil || len(view.Map) != len(raw.Map) || !isGlobalDescribeMap(view) {
					t.Fatalf("typed describe changed shape: type=%s size=%d error=%v", view.Type, len(view.Map), err)
				}
				if view.Ref == raw.Ref {
					t.Fatal("typed describe exposed cached map storage")
				}
				if !reflect.DeepEqual(view.MapOrder, keys) || len(view.MapKeys) != len(keys) {
					t.Fatal("typed describe changed key representation")
				}
			}
			allocations := testing.AllocsPerRun(10, bind)
			measurement := testing.Benchmark(func(b *testing.B) {
				for range b.N {
					bind()
				}
			})
			// A typed binding keeps its private identity without rebuilding the
			// schema entries. Mutation isolation is pinned separately.
			byteLimit := int64(4096)
			bytes := measurement.AllocedBytesPerOp()
			t.Logf("aliases=%d allocations/binding=%.0f bytes/binding=%d limit=%d", len(keys), allocations, bytes, byteLimit)
			if bytes > byteLimit {
				t.Fatalf("typed global describe rebuilt entries: %d bytes, budget %d", bytes, byteLimit)
			}
		})
	}
}

func TestMapCoercionReadOnlyRepairPreservesPayloadAndEmptyMap(t *testing.T) {
	t.Run("nested alias", func(t *testing.T) {
		machine := New(nil)
		child := List(String("before"))
		child.Type = "List<String>"
		key := mapKey(String("items"))
		value := Map()
		value.Type = "Map<Object,Object>"
		value.Map[key] = child
		value.MapKeys = nil
		coerced, err := machine.coerceAssignable("Map<String,List<String>>", value)
		if err != nil {
			t.Fatal(err)
		}
		if coerced.Ref == value.Ref || sameMapBacking(coerced.Map, value.Map) {
			t.Fatal("repair retained the noncanonical parent representation")
		}
		got := coerced.Map[key]
		if got.Ref != child.Ref || !sameSliceBacking(got.List, child.List) {
			t.Fatal("repair detached the unchanged nested list")
		}
		got.List[0] = String("after")
		if child.List[0].Text != "after" || value.MapKeys != nil || value.MapOrder != nil {
			t.Fatal("repair lost the child alias or changed the original metadata")
		}
	})
	t.Run("nil map", func(t *testing.T) {
		value := Value{Kind: ValueMap, Type: "Map<Object,Object>", Ref: newValueRef()}
		coerced, err := New(nil).coerceAssignable("Map<String,Object>", value)
		if err != nil {
			t.Fatal(err)
		}
		if coerced.Ref == value.Ref || coerced.Map == nil || coerced.MapKeys == nil || coerced.MapOrder == nil {
			t.Fatal("repair must detach and materialize an empty map")
		}
		key := mapKey(String("added"))
		coerced.Map[key] = String("usable")
	})
}
