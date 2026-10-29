package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native standard_describe_profiles/P0 field maps and grandchild_projection
// rows already pin the returned tokens and traversal results. This is a local
// work bound: reading a real field must not search every possible child edge.
// It deliberately uses scan counts rather than a host-dependent CPU deadline.
func TestFieldMapDirectReadsDoNotScanChildRelationships(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, object := range []string{"Account", "Contact", "Opportunity", "User"} {
			for _, method := range []string{"get", "containsKey"} {
				t.Run(api+"/"+object+"/"+method, func(t *testing.T) {
					org := storage.NewOrgState()
					for _, name := range []string{"Account", "Contact", "Opportunity", "User"} {
						storage.EnsureStandardObject(&org, name)
					}
					machine := New(nil)
					machine.SetOrg(&org)
					setup, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Map<String, Schema.SObjectField> meadowFields = %s.SObjectType.getDescribe().fields.getMap();
`, object), CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(setup); err != nil {
						t.Fatal(err)
					}
					assertion := "meadowFields.get(meadowKey) != null"
					if method == "containsKey" {
						assertion = "meadowFields.containsKey(meadowKey)"
					}
					program, err := CompileAnonymousWithOptions(`
for (String meadowKey : meadowFields.keySet()) {
    System.assert(`+assertion+`);
}
`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					for iteration := range 2 {
						before := childRelationshipScanVisits.Load()
						if _, err := machine.Execute(program); err != nil {
							t.Fatal(err)
						}
						if visits := childRelationshipScanVisits.Load() - before; visits != 0 {
							t.Errorf("iteration %d: direct field reads scanned %d child candidates, want 0", iteration, visits)
						}
					}
				})
			}
		}
	}
}
