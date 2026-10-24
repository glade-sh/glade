package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func foldedNameLookupVM() *VM {
	org := storage.NewOrgState()
	for _, name := range []string{"Account", "OpportunityLineItem", "demo__Parcel__c"} {
		org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name}}
	}
	return &VM{Org: &org}
}

func TestFoldedNameLookupPreservesCanonicalSpelling(t *testing.T) {
	vm := foldedNameLookupVM()
	for _, tc := range []struct{ input, want string }{
		{"Account", "Account"}, {"ACCOUNT", "Account"}, {" account ", "Account"},
		{"Schema.Account", "Account"}, {"SCHEMA.ACCOUNT", "Account"},
		{"demo__parcel__c", "demo__Parcel__c"}, {"Missing", ""},
	} {
		for range 2 {
			got, ok := vm.resolveObjectName(tc.input)
			if got != tc.want || ok != (tc.want != "") {
				t.Errorf("resolve(%q) = %q, %v; want %q", tc.input, got, ok, tc.want)
			}
		}
	}
}

func TestFoldedNameLookupWarmAllocations(t *testing.T) {
	vm := foldedNameLookupVM()
	for _, tc := range []struct {
		name   string
		lookup func()
	}{
		{"resolve object", func() { vm.resolveObjectName("OpportunityLineItem") }},
		{"resolve schema object", func() { vm.resolveObjectName("Schema.OpportunityLineItem") }},
		{"common object", func() { isCommonSObjectTypeName("Account") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.lookup()
			if got := testing.AllocsPerRun(100, tc.lookup); got != 0 {
				t.Fatalf("warm lookup allocations = %.0f, want 0", got)
			}
		})
	}
}

var foldedNameObjectResult string
var foldedNameCommonResult bool

func BenchmarkFoldedNameLookup(b *testing.B) {
	vm := foldedNameLookupVM()
	b.Run("ResolveObject", func(b *testing.B) {
		vm.resolveObjectName("OpportunityLineItem")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			foldedNameObjectResult, _ = vm.resolveObjectName("OpportunityLineItem")
		}
	})
	b.Run("CommonObject", func(b *testing.B) {
		isCommonSObjectTypeName("Account")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			foldedNameCommonResult = isCommonSObjectTypeName("Account")
		}
	})
}

func foldedParentDefinition(n int) storage.ObjectDefinition {
	parents := make([]string, n)
	for i := range parents {
		parents[i] = fmt.Sprintf(" pkg__Parent%d ", i)
	}
	definition := storage.ObjectDefinition{APIName: "Meadow__c", Relations: []storage.Relationship{{Field: "Parent__c", ParentObjects: parents}}}
	storage.EnsureStandardObjectFields(&definition)
	return definition
}

func TestFoldedRelationshipPreparedAndLiteralRoutes(t *testing.T) {
	for _, parent := range []string{" Account ", "pkg__Account", "pkg__Parcel__c", " Parcel__c ", "Kase", "Σςσ", "ſObject", " ", "pkg__X__c"} {
		definition := storage.ObjectDefinition{APIName: "Meadow__c", Relations: []storage.Relationship{{ParentObjects: []string{parent}}}}
		literal := definition.Relations[0]
		storage.EnsureStandardObjectFields(&definition)
		prepared := definition.Relations[0]
		for _, target := range []string{"Account", "pkg__Account", "other__ACCOUNT", "Parcel__c", "other__Parcel__c", "Kase", "σΣσ", "SObject", "", "Other", "X__c"} {
			want := foldedRelationshipReference(literal, target)
			if got := relationshipTargetsObject(prepared, target); got != want {
				t.Errorf("prepared %q/%q = %v, want %v", parent, target, got, want)
			}
			if got := relationshipTargetsObject(literal, target); got != want {
				t.Errorf("literal %q/%q = %v, want %v", parent, target, got, want)
			}
		}
	}
}

// Keep the pre-index matching contract independent of derived metadata.
func foldedRelationshipReference(relationship storage.Relationship, objectName string) bool {
	objectName = strings.TrimSpace(objectName)
	if objectName == "" {
		return false
	}
	for _, parent := range relationship.ParentObjects {
		parent = strings.TrimSpace(parent)
		if parent == "" {
			continue
		}
		if strings.EqualFold(parent, objectName) {
			return true
		}
		if !strings.Contains(objectName, "__") && !strings.Contains(parent, "__") {
			continue
		}
		if strings.EqualFold(stripAnyNamespaceToken(parent), stripAnyNamespaceToken(objectName)) || strings.EqualFold(stripStandardObjectNamespaceToken(parent), stripStandardObjectNamespaceToken(objectName)) {
			return true
		}
	}
	return false
}

func TestFoldedRelationshipCloneRefreshRoute(t *testing.T) {
	definition := foldedParentDefinition(20)
	clone := definition.Clone()
	clone.Relations[0].ParentObjects[19] = " newpkg__Replacement "
	storage.EnsureStandardObjectFields(&clone)
	if !relationshipTargetsObject(clone.Relations[0], "Replacement") || relationshipTargetsObject(clone.Relations[0], "Parent19") {
		t.Fatal("clone retained stale target keys")
	}
	if !relationshipTargetsObject(definition.Relations[0], "Parent19") || relationshipTargetsObject(definition.Relations[0], "Replacement") {
		t.Fatal("clone changed source targets")
	}
}

func TestFoldedRelationshipWarmAllocations(t *testing.T) {
	definition := foldedParentDefinition(20)
	for _, target := range []string{"pkg__Missing", "other__Parent19", "Parent19"} {
		lookup := func() { foldedNameCommonResult = relationshipTargetsObject(definition.Relations[0], target) }
		lookup()
		if got := testing.AllocsPerRun(100, lookup); got != 0 {
			t.Errorf("20-parent %s allocations = %.0f, want 0", target, got)
		}
	}
}

func BenchmarkFoldedRelationshipTargets(b *testing.B) {
	for _, n := range []int{1, 20, 200} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			definition := foldedParentDefinition(n)
			relationshipTargetsObject(definition.Relations[0], "pkg__Missing")
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				foldedNameCommonResult = relationshipTargetsObject(definition.Relations[0], "pkg__Missing")
			}
		})
	}
}

func TestFoldedRelationshipSetOrgAndReinstallRoutes(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Meadow__c"] = storage.ObjectState{Definition: foldedParentDefinition(20)}
	machine := New(nil)
	machine.SetOrg(&org)
	lookup := func() {
		foldedNameCommonResult = relationshipTargetsObject(org.Objects["Meadow__c"].Definition.Relations[0], "pkg__Missing")
	}
	if got := testing.AllocsPerRun(100, lookup); got != 0 {
		t.Errorf("installed definition allocations = %.0f, want 0", got)
	}
	org.Objects["Meadow__c"].Definition.Relations[0].ParentObjects[19] = "pkg__Replacement"
	machine.SetOrg(&org)
	relation := org.Objects["Meadow__c"].Definition.Relations[0]
	if !relationshipTargetsObject(relation, "Replacement") || relationshipTargetsObject(relation, "Parent19") {
		t.Fatal("reinstalled schema retained stale parent keys")
	}
}

func TestFoldedNameNamespaceAndTriggerRoutes(t *testing.T) {
	machine := foldedNameLookupVM()
	machine.Org.Objects["other__Parcel__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "other__Parcel__c"}}
	for _, ns := range []string{"demo", "other", "demo"} {
		machine.currentTrigger = true
		machine.activeTriggerNamespaces = []string{ns}
		for range 2 {
			got, ok := machine.resolveObjectName("Parcel__c")
			if !ok || got != ns+"__Parcel__c" {
				t.Fatalf("namespace %s: %s %v", ns, got, ok)
			}
		}
	}
}

func TestFoldedNamePublicCommonObjectRoute(t *testing.T) {
	lookup := func() {
		if !IsCommonSObjectTypeName("Account") {
			t.Fatal("Account is not a common object")
		}
	}
	lookup()
	if got := testing.AllocsPerRun(100, lookup); got != 0 {
		t.Fatalf("warm public common-object allocations = %.0f, want 0", got)
	}
}
