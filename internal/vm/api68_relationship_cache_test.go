package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// The native rows in platform_relationships/org.tsv and api68_boundary/org.tsv
// require 62/67 rejection and 68 acceptance, even in a reused runtime.
func TestAPI68PlatformRelationshipWarmVersionBoundary(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine := New(nil)
	machine.SetOrg(&org)
	describe := Value{Fields: map[string]Value{"name": String("Account")}}
	for _, api := range []string{"67.0", "68.0", "62.0", "68.0"} {
		machine.currentMethod.APIVersion = api
		want := api == "68.0"
		got := false
		for _, row := range machine.describeChildRelationships("Account") {
			if row.Fields["relationshipName"].Text == "ContactPointOttSvcs" {
				got = true
			}
		}
		if got != want {
			t.Errorf("API %s describe present = %t, want %t", api, got, want)
		}
		described, err := machine.describeChildRelationshipsForDescribe(&describe)
		if err != nil {
			t.Fatal(err)
		}
		got = false
		for _, row := range described.List {
			if row.Fields["relationshipName"].Text == "ContactPointOttSvcs" {
				got = true
			}
		}
		if got != want {
			t.Errorf("API %s reused describe value present = %t, want %t", api, got, want)
		}
		child := machine.childRelationshipLookup("Account", "ContactPointOttSvcs").ChildType
		if (child == "ContactPointOttSvc") != want {
			t.Errorf("API %s lookup child = %q, want present %t", api, child, want)
		}
	}
}

func TestAPI68PlatformRelationshipLoadedVersionBoundary(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	org.Objects["ContactPointOttSvc"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "ContactPointOttSvc", Relations: []storage.Relationship{{Field: "ParentId", ParentObjects: []string{"Account", "Individual"}, ChildRelationship: "ContactPointOttSvcs", CascadeDelete: true}}}}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, api := range []string{"67.0", "68.0", "62.0", "68.0"} {
		machine.currentMethod.APIVersion = api
		count := 0
		for _, row := range machine.describeChildRelationships("Account") {
			if row.Fields["relationshipName"].Text == "ContactPointOttSvcs" {
				count++
			}
		}
		want := 0
		if api == "68.0" {
			want = 1
		}
		if count != want {
			t.Errorf("API %s loaded describe count = %d, want %d", api, count, want)
		}
		if got := machine.isChildRelationshipField(org.Objects["Account"].Definition, "ContactPointOttSvcs"); got != (want == 1) {
			t.Errorf("API %s loaded field = %t, want %t", api, got, want == 1)
		}
		if got := machine.lazyChildRelationshipLookup("Account", "ContactPointOttSvcs").OK; got != (want == 1) {
			t.Errorf("API %s lazy lookup = %t, want %t", api, got, want == 1)
		}
		if got := len(machine.loadedChildRelationshipLookup("Account", "Account", "ContactPointOttSvcs").ChildRelationshipNames) > 0; got != (want == 1) {
			t.Errorf("API %s loaded value lookup = %t, want %t", api, got, want == 1)
		}
		child := machine.childRelationshipLookup("Account", "ContactPointOttSvcs").ChildType
		if (child == "ContactPointOttSvc") != (want == 1) {
			t.Errorf("API %s loaded lookup = %q, want present %t", api, child, want == 1)
		}
	}
}
