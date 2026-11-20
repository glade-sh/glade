package vm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The unpruned implementation from the baseline supplies a differential oracle.
func (vm *VM) baselineDescribeChildRelationships(name string) []Value {
	vm.materializeLazyStandardSpread()
	vm.ensureMetadataCacheGeneration()
	if vm == nil || vm.Org == nil {
		return nil
	}
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return nil
	}
	if vm.childRelCache == nil {
		vm.childRelCache = newChildRelationshipCache()
	}
	cacheKey := target + "\x00" + vm.currentSourceExecutionPolicy().APIVersion
	if cached, ok := vm.childRelCache.load(cacheKey); ok {
		return cached
	}
	platformChildren := vm.contentChildRelationships(name)
	storage.VisitVersionedPlatformChildRelationships(name, vm.currentSourceExecutionPolicy().APIVersion, func(child string, relation storage.Relationship) {
		platformChildren = append(platformChildren, contentChildRelationship{childObject: child, relationship: relation})
	})
	platformByField := make(map[string]storage.Relationship, len(platformChildren))
	childFieldKey := func(child, field string) string { return strings.ToLower(child) + "\x00" + strings.ToLower(field) }
	for _, child := range platformChildren {
		platformByField[childFieldKey(child.childObject, child.relationship.Field)] = child.relationship
	}
	seenPlatformFields := make(map[string]bool, len(platformChildren))
	childRelationships := make([]Value, 0)
	childObjects := make([]string, 0, len(vm.Org.Objects))
	for childName := range vm.Org.Objects {
		childObjects = append(childObjects, childName)
	}
	sort.Strings(childObjects)
	for _, childName := range childObjects {
		if eventDefinition := vm.Org.Objects[childName].Definition; storage.IsChangeEventObject(eventDefinition) {
			childRelationships = append(childRelationships, vm.standardChangeEventChildRelationships(name, childName, eventDefinition)...)
			continue
		}
		childDefinition := vm.describePreparedDefinition(childName, vm.Org.Objects[childName].Definition)
		relationships := append([]storage.Relationship(nil), childDefinition.Relations...)
		relationships = append(relationships, syntheticSystemParentRelationsForDefinition(childDefinition)...)
		sort.Slice(relationships, func(i, j int) bool {
			if relationships[i].Field == relationships[j].Field {
				return relationships[i].ChildRelationship < relationships[j].ChildRelationship
			}
			return relationships[i].Field < relationships[j].Field
		})
		for _, relationship := range relationships {
			if !storage.PlatformChildRelationshipAvailableAtVersion(name, childName, relationship.Field, vm.currentSourceExecutionPolicy().APIVersion) {
				continue
			}
			// A208/A217: inherited Activity lookups are not separate Task/Event
			// entries in the referenced object's child relationship describe.
			if relationship.InheritedFrom != "" {
				continue
			}
			if relationshipTargetsObject(relationship, name) {
				if len(platformChildren) != 0 {
					key := childFieldKey(childName, relationship.Field)
					if platform, ok := platformByField[key]; ok {
						relationship = platform
					}
					seenPlatformFields[key] = true
				}
				if relationship.ChildRelationship == "" && !vm.capturedTaskAccountChildRelationship(childName, relationship) && canDeriveChildRelationshipName(name, childName, relationship) {
					relationship.ChildRelationship = derivedVMChildRelationshipName(childDefinition)
				}
				if !vm.contentChildDescribeRelationshipVisible(name, relationship.ChildRelationship) {
					continue
				}
				childRelationships = append(childRelationships, vm.childRelationshipValue(childName, relationship))
			}
		}
	}
	for _, child := range platformChildren {
		if !seenPlatformFields[childFieldKey(child.childObject, child.relationship.Field)] {
			childRelationships = append(childRelationships, vm.childRelationshipValue(child.childObject, child.relationship))
		}
	}
	return vm.childRelCache.store(cacheKey, childRelationships)
}

func compareChildDescribeWithBaseline(t *testing.T, machine *VM, name string) {
	t.Helper()
	got := machine.describeChildRelationships(name)
	machine.childRelCache = newChildRelationshipCache()
	want := machine.baselineDescribeChildRelationships(name)
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("child describe %q differs from unpruned baseline (got %d bytes, want %d)", name, len(gotJSON), len(wantJSON))
	}
}

func describePruningTemplate() RestoredRuntimeTemplate {
	org := shapedLazyOrg([]string{"MultiCurrency"})
	for i := 0; i < 120; i++ {
		parent := fmt.Sprintf("Root%03d__c", i)
		child := fmt.Sprintf("Branch%03d__c", i)
		org.Objects[parent] = candidatePlanObject(parent)
		state := candidatePlanChild(child, parent, "Children__r", 4)
		storage.EnsureStandardObjectFields(&state.Definition)
		org.Objects[child] = state
	}
	org.ClearRuntimeSchemaStamp()
	return NewRestoredRuntimeTemplate(org, New(nil))
}

func describePruningClone(template RestoredRuntimeTemplate) *VM {
	org := template.CloneOrg()
	machine := template.CloneMachine(nil)
	machine.SetRuntimeTemplateOrg(&org)
	return machine
}

func TestDescribeChildPruningMatchesBaseline(t *testing.T) {
	template := describePruningTemplate()
	for _, name := range []string{"Root000__c", "root001__C", "Account", "User", "Missing__c", "Ωther__c"} {
		t.Run(name, func(t *testing.T) {
			compareChildDescribeWithBaseline(t, describePruningClone(template), name)
		})
	}
}

func TestDescribeChildPruningInvalidation(t *testing.T) {
	template := describePruningTemplate()
	machine := describePruningClone(template)
	_ = machine.describeChildRelationships("Root000__c")
	definition, ok := storage.EnsureMutableObjectDefinition(machine.Org, "Branch000__c")
	if !ok {
		t.Fatal("missing child definition")
	}
	for i := range definition.Relations {
		if definition.Relations[i].Field == "Parent__c" {
			definition.Relations[i].ParentObjects = []string{"Root001__c"}
		}
	}
	field := definition.Fields["Parent__c"]
	field.ReferenceTo = []string{"Root001__c"}
	definition.Fields["Parent__c"] = field
	machine.SetRuntimeTemplateOrg(machine.Org)
	compareChildDescribeWithBaseline(t, machine, "Root001__c")
	compareChildDescribeWithBaseline(t, describePruningClone(template), "Root000__c")
}

func BenchmarkDescribeChildRelationshipsSpread(b *testing.B) {
	template := describePruningTemplate()
	names := []string{"Root000__c", "Root001__c", "Root002__c", "Root003__c", "Account", "Contact"}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		machine := describePruningClone(template)
		for _, name := range names {
			_ = machine.describeChildRelationships(name)
		}
	}
}
