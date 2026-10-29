package vm

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func candidatePlanObject(name string) storage.ObjectState {
	return storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: name,
		Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
	}, Records: map[storage.ID]storage.Record{}}
}

func candidatePlanChild(name, parent, relationship string, extraFields int) storage.ObjectState {
	fields := map[string]storage.Field{
		"Name": {APIName: "Name", Type: storage.FieldString},
		"Parent__c": {
			APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{parent},
			RelationshipName: "Parent__r", ChildRelationshipName: relationship,
		},
	}
	for i := 0; i < extraFields; i++ {
		key := fmt.Sprintf("Extra%02d__c", i)
		fields[key] = storage.Field{APIName: key, Type: storage.FieldString}
	}
	return storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: name,
		Fields:  fields,
		Relations: []storage.Relationship{{
			Field: "Parent__c", ParentObjects: []string{parent}, ParentRelationship: "Parent__r", ChildRelationship: relationship,
		}},
	}, Records: map[storage.ID]storage.Record{}}
}

func candidatePlanOrg(unrelated int) storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["Parent__c"] = candidatePlanObject("Parent__c")
	org.Objects["Child__c"] = candidatePlanChild("Child__c", "Parent__c", "Children__r", 0)
	for i := 0; i < unrelated; i++ {
		name := fmt.Sprintf("Island%03d__c", i)
		org.Objects[name] = candidatePlanObject(name)
	}
	return org
}

func candidatePlanMachine(org *storage.OrgState) *VM {
	machine := New(nil)
	machine.SetOrg(org)
	return machine
}

func TestLoadedChildRelationshipVisitsIgnoreUnrelatedObjects(t *testing.T) {
	measure := func(unrelated int) uint64 {
		t.Helper()
		org := candidatePlanOrg(unrelated)
		machine := candidatePlanMachine(&org)
		before := childRelationshipScanVisits.Load()
		loaded := machine.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Children__r")
		if len(loaded.ChildRelationshipNames) != 1 || loaded.ChildRelationshipNames[0] != "Children__r" {
			t.Fatalf("loaded lookup = %+v", loaded)
		}
		// Cold misses and a repeated name. The repeat is cached.
		for _, name := range []string{"MissingA__r", "MissingB__r", "MissingA__r"} {
			if len(machine.loadedChildRelationshipLookup("Parent__c", "Parent__c", name).ChildRelationshipNames) != 0 {
				t.Fatalf("unexpected loaded relationship %s", name)
			}
		}
		return childRelationshipScanVisits.Load() - before
	}
	base := measure(0)
	for _, unrelated := range []int{40, 400} {
		if got := measure(unrelated); got != base {
			t.Fatalf("loaded visits with %d unrelated objects = %d, want %d", unrelated, got, base)
		}
	}
	if base == 0 {
		t.Fatal("loaded lookup did not record candidate visits")
	}
}

func TestChildRelationshipParentPlanSkipsObjectStateCopies(t *testing.T) {
	org := candidatePlanOrg(40)
	machine := candidatePlanMachine(&org)
	beforeCopies := childRelationshipObjectStateCopies.Load()
	beforeBuilds := jsonChildRelationshipIndexBuilds.Load()
	for _, name := range []string{"Children__r", "Children", "Missing__r", "Other__r"} {
		_, _ = machine.jsonSObjectChildRelationshipType("Parent__c", name)
	}
	machine.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Children__r")
	if got := childRelationshipObjectStateCopies.Load() - beforeCopies; got != 0 {
		t.Fatalf("parent plan copied %d object states", got)
	}
	if got := jsonChildRelationshipIndexBuilds.Load() - beforeBuilds; got != 1 {
		t.Fatalf("parent plan builds = %d, want 1", got)
	}
}

// A completed parent plan already answers every spelling and every miss. Its
// retained size must depend on schema names, not on names supplied by callers.
func TestChildRelationshipParentPlanBoundsQueryRetention(t *testing.T) {
	for _, namespace := range []string{"", "pkg"} {
		for _, frozen := range []bool{false, true} {
			t.Run(fmt.Sprintf("namespace=%s/frozen=%v", namespace, frozen), func(t *testing.T) {
				org := candidatePlanOrg(0)
				org.Namespace = namespace
				machine := candidatePlanMachine(&org)
				if frozen {
					template := NewRestoredRuntimeTemplate(org, New(nil))
					machine = template.CloneMachine(nil)
					clone := template.CloneOrg()
					machine.SetRuntimeTemplateOrg(&clone)
				}
				for i := 0; i < 400; i++ {
					parent := []string{"Parent__c", "parent__c", "PARENT__C"}[i%3]
					if _, ok := machine.jsonSObjectChildRelationshipType(parent, fmt.Sprintf("Missing%d__r", i)); ok {
						t.Fatal("invented relationship")
					}
					if got, ok := machine.jsonSObjectChildRelationshipType(parent, []string{"Children", "Children__r", "CHILDREN__R"}[i%3]); !ok || got != "List<Child__c>" {
						t.Fatalf("positive lookup = %q, %v", got, ok)
					}
				}
				cache := machine.jsonChildRelTypeCache
				cache.mu.RLock()
				defer cache.mu.RUnlock()
				if len(cache.parentIndexes) != 1 || len(cache.entries) != 0 {
					t.Fatalf("retained %d parent plans and %d redundant query entries", len(cache.parentIndexes), len(cache.entries))
				}
			})
		}
	}
}

// Repeated reference fields can describe the same relationship. Building the
// immutable result should not allocate an alias list and match slice per field.
func TestChildRelationshipParentPlanBoundsDuplicateAllocations(t *testing.T) {
	for _, namespace := range []string{"", "pkg"} {
		t.Run(namespace, func(t *testing.T) {
			measure := func(copies int) float64 {
				org := candidatePlanOrg(0)
				org.Namespace = namespace
				child := org.Objects["Child__c"]
				for i := 0; i < copies; i++ {
					field := child.Definition.Fields["Parent__c"]
					field.APIName = fmt.Sprintf("Lookup%d__c", i)
					child.Definition.Fields[field.APIName] = field
					child.Definition.Relations = append(child.Definition.Relations, child.Definition.Relations[0])
				}
				org.Objects["Child__c"] = child
				machine := candidatePlanMachine(&org)
				machine.childRelationshipCandidateNames("Parent__c")
				return testing.AllocsPerRun(3, func() {
					index := machine.buildJSONSObjectChildRelationshipTypeIndex("Parent__c")
					if got := index["children__r"].Type; got != "List<Child__c>" {
						t.Fatalf("duplicate lookup = %q", got)
					}
				})
			}
			baseline := measure(0)
			for _, copies := range []int{40, 400} {
				if got := measure(copies); got > baseline+16 {
					t.Errorf("%d duplicate references: %.0f allocations, baseline %.0f (limit +16)", copies, got, baseline)
				}
			}
		})
	}
}

func TestChildRelationshipCandidatesPreserveEqualFoldTargets(t *testing.T) {
	for _, tc := range []struct{ target, parent string }{
		{"ſParent__c", "SParent__c"},
		{"ΣParent__c", "ςParent__c"},
		{"pkg__ſParent__c", "other__SParent__c"},
		{"pkg__ΣParent__c", "other__ςParent__c"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			for _, frozen := range []bool{false, true} {
				org := storage.NewOrgState()
				org.Objects[tc.parent] = candidatePlanObject(tc.parent)
				org.Objects["Child__c"] = candidatePlanChild("Child__c", tc.target, "Children__r", 0)
				machine := candidatePlanMachine(&org)
				if frozen {
					template := NewRestoredRuntimeTemplate(org, New(nil))
					machine = template.CloneMachine(nil)
					clone := template.CloneOrg()
					machine.SetRuntimeTemplateOrg(&clone)
				}
				if !foldedRelationshipReference(machine.Org.Objects["Child__c"].Definition.Relations[0], tc.parent) {
					t.Fatal("fixture does not match the unindexed predicate")
				}
				loaded := machine.loadedChildRelationshipLookup(tc.parent, tc.parent, "Children__r")
				if len(loaded.ChildRelationshipNames) != 1 || loaded.ChildRelationshipNames[0] != "Children__r" {
					t.Errorf("frozen=%v loaded lookup = %+v", frozen, loaded)
				}
				if got := machine.loadedChildRelationshipLookup(tc.parent, tc.parent, "Missing__r"); len(got.ChildRelationshipNames) != 0 {
					t.Error("invented a missing relationship")
				}
			}
		})
	}
}

// Only loaded lookup scanned the full org on the baseline. JSON, DML, and lazy
// lookup already used lowercase candidate keys, so preserve their misses even
// when the final relationship predicate would accept Unicode simple folding.
func TestChildRelationshipCandidateRoutesPreserveBaselineUnicodeMisses(t *testing.T) {
	for _, tc := range []struct{ target, parent string }{
		{"ſParent__c", "SParent__c"},
		{"SParent__c", "ſParent__c"},
		{"ΣParent__c", "ςParent__c"},
		{"pkg__ſParent__c", "other__SParent__c"},
	} {
		for _, frozen := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/frozen=%v", tc.target, tc.parent, frozen), func(t *testing.T) {
				org := storage.NewOrgState()
				org.Objects[tc.parent] = candidatePlanObject(tc.parent)
				org.Objects["Child__c"] = candidatePlanChild("Child__c", tc.target, "Children__r", 0)
				machine := candidatePlanMachine(&org)
				if frozen {
					template := NewRestoredRuntimeTemplate(org, New(nil))
					machine = template.CloneMachine(nil)
					clone := template.CloneOrg()
					machine.SetRuntimeTemplateOrg(&clone)
				}
				if got := machine.loadedChildRelationshipLookup(tc.parent, tc.parent, "Children__r"); len(got.ChildRelationshipNames) != 1 {
					t.Errorf("loaded lookup lost its original full-org match: %+v", got)
				}
				if got, ok := machine.jsonSObjectChildRelationshipType(tc.parent, "Children__r"); ok || got != "" {
					t.Errorf("JSON broadened baseline miss: %q, %v", got, ok)
				}
				if got := machine.childRelationshipLookup(tc.parent, "Children__r"); got.ChildType != "" {
					t.Errorf("DML broadened baseline miss: %+v", got)
				}
				if got := machine.lazyChildRelationshipLookup(tc.parent, "Children__r"); got.OK {
					t.Errorf("lazy lookup broadened baseline miss: %+v", got)
				}
			})
		}
	}
}

func TestChildRelationshipParentPlanKeepsRelationBeforeFieldPrecedence(t *testing.T) {
	for _, names := range [][2]string{{"Child__c", "CHILD__C"}, {"pkg__Child__c", "PKG__CHILD__C"}, {"Item__c", "ITEM__C"}} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reverse=%v", names[0], reverse), func(t *testing.T) {
				org := storage.NewOrgState()
				org.Objects["Parent__c"] = candidatePlanObject("Parent__c")
				fieldOnly := candidatePlanChild(names[0], "Parent__c", "Children__r", 0)
				fieldOnly.Definition.Relations = nil
				relationOnly := candidatePlanChild(names[1], "Parent__c", "Children__r", 0)
				relationOnly.Definition.Fields = nil
				org.Objects[names[0]], org.Objects[names[1]] = fieldOnly, relationOnly
				machine := candidatePlanMachine(&org)
				// Pin both candidate visitation orders. The unindexed JSON builder
				// visits every relation before any reference field in either order.
				table := machine.childRelationshipCandidateTable()
				order := []string{names[0], names[1]}
				if reverse {
					order[0], order[1] = order[1], order[0]
				}
				table["parent__c"] = order
				for _, alias := range []string{"Children__r", "Children"} {
					if got, ok := machine.jsonSObjectChildRelationshipType("Parent__c", alias); !ok || got != "List<"+names[1]+">" {
						t.Errorf("%s = %q, %v; want relation spelling %s", alias, got, ok, names[1])
					}
				}
			})
		}
	}
}

func TestChildRelationshipScanCandidateAllocationBound(t *testing.T) {
	for _, children := range []int{40, 400} {
		t.Run(fmt.Sprint(children), func(t *testing.T) {
			org := candidatePlanOrg(0)
			for i := 0; i < children; i++ {
				name := fmt.Sprintf("Item%03d__c", i)
				org.Objects[name] = candidatePlanChild(name, "Parent__c", "Items__r", 0)
			}
			machine := candidatePlanMachine(&org)
			want := make(map[string]storage.ObjectState)
			for _, name := range machine.childRelationshipCandidateNames("Parent__c") {
				want[name] = org.Objects[name]
			}
			got := make(map[string]storage.ObjectState)
			for name, state := range machine.childRelationshipScanCandidates("Parent__c") {
				got[name] = state
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatal("candidate names or definitions differ from the snapshot reference")
			}
			visits := 0
			allocations := testing.AllocsPerRun(3, func() {
				visits = 0
				for _, state := range machine.childRelationshipScanCandidates("Parent__c") {
					if state.Definition.APIName != "" {
						visits++
					}
				}
			})
			if visits != len(want) {
				t.Fatalf("visits = %d, want %d", visits, len(want))
			}
			// The name set can allocate map groups; it must not allocate an
			// ObjectState value for every candidate as the former map did.
			if limit := float64(len(want))/4 + 16; allocations > limit {
				t.Fatalf("%d candidates allocated %.0f times, limit %.0f", len(want), allocations, limit)
			}
		})
	}
}

func TestChildRelationshipScanCandidateBoundaries(t *testing.T) {
	for _, machine := range []*VM{nil, New(nil)} {
		for range machine.childRelationshipScanCandidates("Parent__c") {
			t.Fatal("nil org yielded a candidate")
		}
	}
	org := candidatePlanOrg(0)
	machine := candidatePlanMachine(&org)
	table := machine.childRelationshipCandidateTable()
	table["parent__c"] = append(table["parent__c"], "Vanished__c")
	scan := machine.childRelationshipScanCandidates("Parent__c")
	next := candidatePlanOrg(0)
	next.Objects["Child__c"] = candidatePlanObject("Replacement__c")
	machine.SetOrg(&next)
	visits := 0
	for name, state := range scan {
		visits++
		if name != "Child__c" || state.Definition.APIName != "Child__c" {
			t.Fatalf("scan escaped its original org: %q / %q", name, state.Definition.APIName)
		}
	}
	if visits != 1 {
		t.Fatalf("visits = %d, want one existing child", visits)
	}
	visits = 0
	for range machine.childRelationshipScanCandidates("Parent__c") {
		visits++
		break
	}
	// The replacement has no reference field, so use the original org to
	// exercise early termination over an actual candidate.
	machine.SetOrg(&org)
	for range machine.childRelationshipScanCandidates("Parent__c") {
		visits++
		break
	}
	if visits != 1 {
		t.Fatalf("early termination visits = %d, want 1", visits)
	}
}

func TestChildRelationshipScanCandidateEarlyBreakAndDistinctNames(t *testing.T) {
	org := candidatePlanOrg(0)
	org.Namespace = "pkg"
	for _, name := range []string{"CHILD__C", "pkg__Child__c"} {
		org.Objects[name] = candidatePlanChild(name, "Parent__c", "Children__r", 0)
	}
	machine := candidatePlanMachine(&org)
	visits := 0
	for range machine.childRelationshipScanCandidates("pkg__Parent__c") {
		visits++
		break
	}
	if visits != 1 {
		t.Fatalf("early termination visits = %d, want 1", visits)
	}
	names := make(map[string]bool)
	for name := range machine.childRelationshipScanCandidates("pkg__Parent__c") {
		names[name] = true
	}
	if len(names) != 3 || !names["Child__c"] || !names["CHILD__C"] || !names["pkg__Child__c"] {
		t.Fatalf("distinct candidate spellings were collapsed: %v", names)
	}
}

func TestChildRelationshipPlanLifetimes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := candidatePlanOrg(0)
			machine := candidatePlanMachine(&org)
			before := jsonChildRelationshipIndexBuilds.Load()
			if got, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); !ok || got != "List<Child__c>" {
				t.Fatalf("positive = %q, %v", got, ok)
			}
			if got := jsonChildRelationshipIndexBuilds.Load() - before; got != 1 {
				t.Fatalf("builds = %d, want 1", got)
			}
			// Same-count in-place field change without a generation bump keeps the plan.
			child := org.Objects["Child__c"]
			field := child.Definition.Fields["Parent__c"]
			field.ChildRelationshipName = "Next__r"
			child.Definition.Fields["Parent__c"] = field
			child.Definition.Relations[0].ChildRelationship = "Next__r"
			org.Objects["Child__c"] = child
			if got, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); !ok || got != "List<Child__c>" {
				t.Fatalf("stale plan = %q, %v", got, ok)
			}
			if _, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Next__r"); ok {
				t.Fatal("in-place rename was visible before the generation bump")
			}
			if got := jsonChildRelationshipIndexBuilds.Load() - before; got != 1 {
				t.Fatalf("builds after in-place edit = %d, want 1", got)
			}

			org.ClearRuntimeSchemaStamp()
			if _, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); ok {
				t.Fatal("old relationship survived the generation bump")
			}
			if got, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Next__r"); !ok || got != "List<Child__c>" {
				t.Fatalf("renamed = %q, %v", got, ok)
			}
			loaded := machine.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Next__r")
			if len(loaded.ChildRelationshipNames) != 1 {
				t.Fatalf("loaded renamed = %+v", loaded)
			}
			if len(machine.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Children__r").ChildRelationshipNames) != 0 {
				t.Fatal("loaded lookup kept the old relationship")
			}

			// Positive again, still the same object count.
			child = org.Objects["Child__c"]
			field = child.Definition.Fields["Parent__c"]
			field.ChildRelationshipName = "Children__r"
			field.ReferenceTo = []string{"Parent__c"}
			child.Definition.Fields["Parent__c"] = field
			child.Definition.Relations[0].ChildRelationship = "Children__r"
			child.Definition.Relations[0].ParentObjects = []string{"Parent__c"}
			org.Objects["Child__c"] = child
			org.ClearRuntimeSchemaStamp()
			if got, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); !ok || got != "List<Child__c>" {
				t.Fatalf("restored = %q, %v", got, ok)
			}
			if _, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Next__r"); ok {
				t.Fatal("renamed relationship survived restoration")
			}

			source := `Child__c child = new Child__c(Name='owned'); Parent__c row = new Parent__c(Name='p'); row.Parcels__r = new List<Child__c>{child}; System.assertEquals(1, row.getSObjects('Parcels__r').size());`
			// The fixture relationship is Children__r. Exercise the anonymous and
			// test-context routes through that name.
			source = strings.ReplaceAll(source, "Parcels__r", "Children__r")
			for _, testContext := range []bool{false, true} {
				readerOrg := candidatePlanOrg(0)
				reader := candidatePlanMachine(&readerOrg)
				if testContext {
					reader.EnableTestContext()
				}
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := reader.Execute(program); err != nil {
					t.Fatalf("testContext=%v: %v", testContext, err)
				}
				// Warm route: the second parent-plan read builds nothing.
				if _, ok := reader.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); !ok {
					t.Fatal("warm read missed")
				}
				built := jsonChildRelationshipIndexBuilds.Load()
				if _, ok := reader.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); !ok {
					t.Fatal("second warm read missed")
				}
				if jsonChildRelationshipIndexBuilds.Load() != built {
					t.Fatal("warm read rebuilt the parent plan")
				}
			}
		})
	}
}

func TestChildRelationshipPlanFrozenSiblingsAndUnstampedHosts(t *testing.T) {
	seed := candidatePlanOrg(8)
	template := storage.NewRuntimeTemplate(seed)
	PrimeRuntimeTemplateSchema(&template)
	before := jsonChildRelationshipIndexBuilds.Load()
	var visits []uint64
	for i := 0; i < 2; i++ {
		machine := New(nil)
		machine.PrimeMetadataSchema(&template.Org)
		clone := template.CloneRuntimeOrg()
		machine.SetRuntimeTemplateOrg(&clone)
		start := childRelationshipScanVisits.Load()
		if got, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); !ok || got != "List<Child__c>" {
			t.Fatalf("sibling %d JSON = %q, %v", i, got, ok)
		}
		if _, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Missing__r"); ok {
			t.Fatal("unexpected missing relationship")
		}
		loaded := machine.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Children__r")
		if len(loaded.ChildRelationshipNames) != 1 {
			t.Fatalf("sibling %d loaded = %+v", i, loaded)
		}
		visits = append(visits, childRelationshipScanVisits.Load()-start)
	}
	if got := jsonChildRelationshipIndexBuilds.Load() - before; got != 1 {
		t.Fatalf("frozen sibling builds = %d, want 1", got)
	}
	if visits[1] != 0 {
		t.Fatalf("second frozen sibling rescanned: %v", visits)
	}
	if visits[0] > 2 {
		t.Fatalf("frozen loaded scan visited %d objects, want at most the parent and child", visits[0])
	}

	unstamped := candidatePlanOrg(0)
	if unstamped.RuntimeSchemaStamp != "" {
		t.Fatal("fixture org is stamped")
	}
	first := candidatePlanMachine(&unstamped)
	secondOrg := candidatePlanOrg(0)
	second := candidatePlanMachine(&secondOrg)
	before = jsonChildRelationshipIndexBuilds.Load()
	if _, ok := first.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); !ok {
		t.Fatal("unstamped host missed")
	}
	if _, ok := second.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); !ok {
		t.Fatal("second unstamped host missed")
	}
	if got := jsonChildRelationshipIndexBuilds.Load() - before; got != 2 {
		t.Fatalf("unstamped hosts shared a parent plan: builds = %d, want 2", got)
	}
}

func TestChildRelationshipCandidateSemantics(t *testing.T) {
	t.Run("field-only", func(t *testing.T) {
		org := storage.NewOrgState()
		org.Objects["Parent__c"] = candidatePlanObject("Parent__c")
		org.Objects["Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
			APIName: "Child__c",
			Fields: map[string]storage.Field{
				"Parent__c": {
					APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"},
					RelationshipName: "Parent__r", ChildRelationshipName: "Children__r",
				},
			},
		}}
		machine := candidatePlanMachine(&org)
		if got, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); !ok || got != "List<Child__c>" {
			t.Fatalf("JSON field-only = %q, %v", got, ok)
		}
		if len(machine.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Children__r").ChildRelationshipNames) != 0 {
			t.Fatal("loaded lookup used field-reference semantics")
		}
	})

	t.Run("duplicate-targets", func(t *testing.T) {
		org := storage.NewOrgState()
		org.Objects["Parent__c"] = candidatePlanObject("Parent__c")
		org.Objects["Alpha__c"] = candidatePlanChild("Alpha__c", "Parent__c", "Children__r", 0)
		org.Objects["Beta__c"] = candidatePlanChild("Beta__c", "Parent__c", "Children__r", 4)
		machine := candidatePlanMachine(&org)
		if got, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); !ok || got != "List<Beta__c>" {
			t.Fatalf("duplicate target = %q, %v", got, ok)
		}
		if len(machine.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Children__r").ChildRelationshipNames) != 1 {
			t.Fatal("loaded lookup duplicated the shared relationship name")
		}
	})

	t.Run("namespace-collision", func(t *testing.T) {
		org := storage.NewOrgState()
		org.Namespace = "pkg"
		org.Objects["pkg__Parent__c"] = candidatePlanObject("pkg__Parent__c")
		org.Objects["pkg__Child__c"] = candidatePlanChild("pkg__Child__c", "Parent__c", "pkg__Children__r", 0)
		other := candidatePlanChild("Other__c", "Parent__c", "Children__r", 8)
		org.Objects["Other__c"] = other
		machine := candidatePlanMachine(&org)
		if got, ok := machine.jsonSObjectChildRelationshipType("pkg__Parent__c", "Children__r"); !ok || got != "List<pkg__Child__c>" {
			t.Fatalf("namespaced target = %q, %v", got, ok)
		}
		before := jsonChildRelationshipIndexBuilds.Load()
		org.Namespace = "other"
		if _, ok := machine.jsonSObjectChildRelationshipType("pkg__Parent__c", "Children__r"); !ok {
			t.Fatal("namespace change dropped the stored relationship name")
		}
		if jsonChildRelationshipIndexBuilds.Load()-before != 1 {
			t.Fatal("namespace change reused the parent plan")
		}
	})

	t.Run("change-event", func(t *testing.T) {
		org := candidatePlanOrg(0)
		org.Objects["Parent__ChangeEvent"] = storage.ObjectState{Definition: storage.ObjectDefinition{
			APIName:  "Parent__ChangeEvent",
			Metadata: map[string]string{"kind": "changeEvent", "parentObject": "Parent__c"},
			Relations: []storage.Relationship{{
				Field: "Parent__c", ParentObjects: []string{"Parent__c"}, ChildRelationship: "Children__r",
			}},
		}}
		machine := candidatePlanMachine(&org)
		if got, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children__r"); !ok || got != "List<Child__c>" {
			t.Fatalf("change event shadowed the child: %q, %v", got, ok)
		}
		if names := machine.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Children__r").ChildRelationshipNames; len(names) != 1 || names[0] != "Children__r" {
			t.Fatalf("loaded change-event result = %v", names)
		}
	})

	t.Run("catalog-only", func(t *testing.T) {
		org := storage.NewOrgState()
		before := len(org.Objects)
		machine := candidatePlanMachine(&org)
		got, ok := machine.jsonSObjectChildRelationshipType("Account", "ChildAccounts")
		if !ok || got != "List<Account>" {
			t.Fatalf("catalog self-parent = %q, %v", got, ok)
		}
		if _, ok := machine.jsonSObjectChildRelationshipType("Account", "NotAChild__r"); ok {
			t.Fatal("catalog lookup invented a relationship")
		}
		if len(machine.Org.Objects) != before {
			t.Fatalf("catalog lookup installed %d objects", len(machine.Org.Objects)-before)
		}
	})

	t.Run("content-fallback", func(t *testing.T) {
		org := storage.NewOrgState()
		org.Objects["Widget__c"] = candidatePlanObject("Widget__c")
		org.Objects["Attachment"] = candidatePlanObject("Attachment")
		machine := candidatePlanMachine(&org)
		loaded := machine.loadedChildRelationshipLookup("Widget__c", "Widget__c", "Attachments")
		if len(loaded.ChildRelationshipNames) != 1 || loaded.ChildRelationshipNames[0] != "Attachments" {
			t.Fatalf("content fallback = %+v", loaded)
		}
		if len(machine.loadedChildRelationshipLookup("Widget__c", "Widget__c", "Nope__r").ChildRelationshipNames) != 0 {
			t.Fatal("content fallback matched an unrelated name")
		}
	})

	t.Run("parent-child-name", func(t *testing.T) {
		org := storage.NewOrgState()
		parent := candidatePlanObject("Parent__c")
		parent.Definition.Fields["Lookup__c"] = storage.Field{
			APIName: "Lookup__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Items__r",
		}
		parent.Definition.Relations = []storage.Relationship{{
			Field: "Lookup__c", ParentObjects: []string{"Account"}, ParentRelationship: "Items__r",
		}}
		org.Objects["Parent__c"] = parent
		org.Objects["Item__c"] = candidatePlanChild("Item__c", "Parent__c", "Items__r", 0)
		machine := candidatePlanMachine(&org)
		loaded := machine.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Items__r")
		if !loaded.ParentRelationshipExists || len(loaded.ChildRelationshipNames) != 1 {
			t.Fatalf("conflict lookup = %+v", loaded)
		}
		row := Object("Parent__c")
		row.Fields["Items__r"] = List(Object("Item__c"))
		if _, value, ok := machine.loadedChildRelationshipValue(row, "Items__r"); !ok || value.Kind != ValueList || len(value.List) != 1 {
			t.Fatalf("projected child = %+v, %v", value, ok)
		}
		row.Fields["Items__r"] = Null
		if _, _, ok := machine.loadedChildRelationshipValue(row, "Items__r"); ok {
			t.Fatal("null child won over the parent relationship")
		}
	})

	t.Run("source-projection", func(t *testing.T) {
		org := candidatePlanOrg(0)
		child := org.Objects["Child__c"]
		child.Records[storage.ID("child")] = storage.Record{ID: "child", Object: "Child__c"}
		org.Objects["Child__c"] = child
		machine := candidatePlanMachine(&org)
		row := Object("Parent__c")
		row.Fields[sobjectQueriedFieldsField] = List(String("Children__r"))
		row.Fields["Children__r"] = List(Object("Child__c"))
		name, value, ok := machine.loadedChildRelationshipValue(row, "Children__r")
		if !ok || name != "Children__r" || value.Kind != ValueList || len(value.List) != 1 {
			t.Fatalf("projected value = %q %+v %v", name, value, ok)
		}
		delete(row.Fields, "Children__r")
		if _, _, ok := machine.loadedChildRelationshipValue(row, "Children__r"); ok {
			t.Fatal("loaded lookup materialized a stored record")
		}
	})
}

// EqualFold and ToLower do not define the same Unicode equivalence classes.
// Alias suppression belongs to each field, before lookup-key expansion; a
// different field is still allowed to contribute the suppressed spelling.
func TestChildRelationshipParentPlanPreservesPerFieldAliasFolding(t *testing.T) {
	groups := []string{"sſ", "µμ", "\u0345ιι", "βϐ", "εϵ", "θϑ", "κϰ", "πϖ", "ρϱ", "σς", "φϕ", "вᲀ", "дᲁ", "оᲂ", "сᲃ", "тᲄᲅ", "ъᲆ", "ѣᲇ", "ᲈꙋ", "ṡẛ"}
	for _, group := range groups {
		for _, explicit := range group {
			for _, derived := range group {
				if explicit == derived {
					continue
				}
				t.Run(fmt.Sprintf("%U_%U", explicit, derived), func(t *testing.T) {
					lookupKeys := map[string]bool{strings.ToLower(string(explicit)): true, strings.ToLower(string(derived)): true}
					if !strings.EqualFold(string(explicit), string(derived)) || len(lookupKeys) != 2 {
						t.Fatal("fixture must distinguish folding from lowercasing")
					}
					for _, stored := range []bool{false, true} {
						org := candidatePlanOrg(0)
						child := org.Objects["Child__c"]
						child.Definition.Relations = nil // field-only route
						field := child.Definition.Fields["Parent__c"]
						suffix := "s"
						if stored {
							field.ChildRelationshipName = ""
							field.RelationshipName = string(explicit) + suffix
						} else {
							field.ChildRelationshipName = string(explicit) + suffix
						}
						child.Definition.PluralLabel = string(derived) + suffix
						child.Definition.Fields["Parent__c"] = field
						org.Objects["Child__c"] = child
						machine := candidatePlanMachine(&org)
						want := map[string]jsonRelationshipTypeLookup{}
						for _, name := range vmFieldChildRelationshipNames(child.Definition, field) {
							for _, key := range jsonRelationshipNameLookupKeys(org.Namespace, name) {
								want[key] = jsonRelationshipTypeLookup{Type: "List<Child__c>", OK: true}
							}
						}
						got := machine.buildJSONSObjectChildRelationshipTypeIndex("Parent__c")
						if !reflect.DeepEqual(got, want) {
							t.Errorf("stored=%v aliases = %#v, want legacy per-field aliases %#v", stored, got, want)
						}
						// The second field's explicit spelling must not be globally suppressed.
						next := field
						next.APIName = "Second__c"
						next.ChildRelationshipName = string(derived) + suffix
						child.Definition.Fields[next.APIName] = next
						org.Objects["Child__c"] = child
						got = machine.buildJSONSObjectChildRelationshipTypeIndex("Parent__c")
						if v := got[strings.ToLower(string(derived)+suffix)]; !v.OK || v.Type != "List<Child__c>" {
							t.Errorf("stored=%v separate field lost its alias: %#v", stored, got)
						}
					}
				})
			}
		}
	}
}

func TestChildRelationshipAliasFoldingJSONExecutionRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, testContext := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/testContext=%v", api, testContext), func(t *testing.T) {
				org := candidatePlanOrg(0)
				child := org.Objects["Child__c"]
				child.Definition.Relations = nil
				child.Definition.PluralLabel = "ſs"
				field := child.Definition.Fields["Parent__c"]
				field.ChildRelationshipName = "Ss"
				child.Definition.Fields["Parent__c"] = field
				org.Objects["Child__c"] = child
				machine := candidatePlanMachine(&org)
				if testContext {
					machine.EnableTestContext()
				}
				var source strings.Builder
				for _, name := range []string{"Ss", "ſs"} {
					payload, _ := json.Marshal(map[string]any{name: map[string]any{"records": []any{map[string]any{"attributes": map[string]any{"type": "Child__c"}, "Name": "kid"}}}})
					for _, call := range []string{"JSON.deserialize(payload, Parent__c.class)", "JSON.deserializeStrict(payload, Parent__c.class)", "JSON.createParser(payload).readValueAs(Parent__c.class)"} {
						expected := "No field name specified on column for sobject of type Parent__c"
						if strings.Contains(call, "deserializeStrict") {
							expected = "No such column 'ſs' on sobject of type Parent__c"
						}
						expected = strings.ReplaceAll(expected, "'", "\\'")
						fmt.Fprintf(&source, `{ String payload = '%s'; Boolean rejected=false; try { Parent__c row = (Parent__c)%s; System.assert(JSON.serialize(row).contains('"Name":"kid"')); } catch (System.JSONException e) { rejected=true; System.assertEquals('%s',e.getMessage()); } System.assertEquals(%v,rejected); }`, payload, call, expected, name == "ſs")
					}
				}
				program, err := CompileAnonymousWithOptions(source.String(), CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				for _, route := range []string{"cold", "warm"} {
					if _, err := machine.Execute(program); err != nil {
						t.Fatalf("%s: %v", route, err)
					}
				}
			})
		}
	}
}
