package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRestoredChildRelationshipCacheCloneRoutes(t *testing.T) {
	for _, frozen := range []bool{false, true} {
		t.Run(fmt.Sprintf("frozen=%v", frozen), func(t *testing.T) {
			seed := runtimeGenerationIndexTemplate(8)
			child := seed.Org.Objects["Child000__c"]
			child.Definition.Relations = []storage.Relationship{{Field: "Parent000__c", ParentObjects: []string{"Parent__c"}, ParentRelationship: "Parent__r", ChildRelationship: "Children000__r"}}
			seed.Org.Objects["Child000__c"] = child
			seed.Org.ClearRuntimeSchemaStamp()
			source := New(nil)
			template := NewRestoredRuntimeTemplate(seed.Org, source)
			newMachine := func() *VM {
				machine := source.CloneRuntime(nil)
				if frozen {
					machine = template.CloneMachine(nil)
				}
				org := template.CloneOrg()
				generation := org.RuntimeGenerationID()
				machine.SetRuntimeTemplateOrg(&org)
				if org.RuntimeGenerationID() != generation {
					t.Error("installing a restored org clone discarded its template generation")
				}
				return machine
			}
			first := newMachine()
			before := jsonChildRelationshipIndexBuilds.Load()
			if got, ok := first.jsonSObjectChildRelationshipType("Parent__c", "Children000__r"); !ok || got != "List<Child000__c>" {
				t.Fatalf("first JSON lookup = %q, %v", got, ok)
			}
			first.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Children000__r")
			if !first.lazyChildRelationshipLookup("Parent__c", "Children000__r").OK {
				t.Fatal("missing lazy relationship")
			}
			sibling := newMachine()
			if first.jsonChildRelTypeCache != sibling.jsonChildRelTypeCache || first.loadedChildRelCache != sibling.loadedChildRelCache || first.lazyChildRelCache != sibling.lazyChildRelCache {
				t.Error("restored clones did not share JSON, loaded, and lazy relationship caches")
			}
			if got, ok := sibling.jsonSObjectChildRelationshipType("Parent__c", "Children000__r"); !ok || got != "List<Child000__c>" {
				t.Fatalf("sibling JSON lookup = %q, %v", got, ok)
			}
			if builds := jsonChildRelationshipIndexBuilds.Load() - before; builds != 1 {
				t.Errorf("parent index builds across restored clones = %d, want 1", builds)
			}

			// Schema writes detach the org and all three derived lookup caches.
			definition, ok := storage.EnsureMutableObjectDefinition(first.Org, "Child000__c")
			if !ok {
				t.Fatal("missing child definition")
			}
			field := definition.Fields["Parent000__c"]
			field.ChildRelationshipName = "Renamed__r"
			definition.Fields["Parent000__c"] = field
			definition.Relations[0].ChildRelationship = "Renamed__r"
			first.SetRuntimeTemplateOrg(first.Org)
			if first.jsonChildRelTypeCache == sibling.jsonChildRelTypeCache || first.loadedChildRelCache == sibling.loadedChildRelCache || first.lazyChildRelCache == sibling.lazyChildRelCache {
				t.Error("mutated clone retained sibling caches")
			}
			for _, machine := range []*VM{sibling, newMachine()} {
				if got, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children000__r"); !ok || got != "List<Child000__c>" {
					t.Errorf("schema mutation leaked to sibling JSON lookup: %q, %v", got, ok)
				}
				if _, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Renamed__r"); ok {
					t.Error("renamed JSON relationship leaked to sibling")
				}
				if !machine.lazyChildRelationshipLookup("Parent__c", "Children000__r").OK || machine.lazyChildRelationshipLookup("Parent__c", "Renamed__r").OK {
					t.Error("schema mutation leaked to sibling lazy lookup")
				}
				if len(machine.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Children000__r").ChildRelationshipNames) != 1 || len(machine.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Renamed__r").ChildRelationshipNames) != 0 {
					t.Error("schema mutation leaked to sibling loaded lookup")
				}
			}
			if _, ok := first.jsonSObjectChildRelationshipType("Parent__c", "Children000__r"); ok {
				t.Error("mutated clone retained old JSON relationship")
			}
			if _, ok := first.jsonSObjectChildRelationshipType("Parent__c", "Renamed__r"); !ok || !first.lazyChildRelationshipLookup("Parent__c", "Renamed__r").OK || len(first.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Renamed__r").ChildRelationshipNames) != 1 {
				t.Error("mutated clone did not see its renamed relationship")
			}
		})
	}
}

func BenchmarkRestoredChildRelationshipCaches(b *testing.B) {
	seed := runtimeGenerationIndexTemplate(512)
	template := NewRestoredRuntimeTemplate(seed.Org, New(nil))
	before := jsonChildRelationshipIndexBuilds.Load()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		org := template.CloneOrg()
		machine := template.CloneMachine(nil)
		machine.SetRuntimeTemplateOrg(&org)
		if _, ok := machine.jsonSObjectChildRelationshipType("Parent__c", "Children000__r"); !ok {
			b.Fatal("missing JSON child relationship")
		}
		machine.loadedChildRelationshipLookup("Parent__c", "Parent__c", "Children000__r")
		machine.lazyChildRelationshipLookup("Parent__c", "Children000__r")
	}
	b.StopTimer()
	b.ReportMetric(float64(jsonChildRelationshipIndexBuilds.Load()-before)/float64(b.N), "json-index-builds/op")
}
