package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The owned Account observations separate an owner's scope from ordinary
// without-sharing visibility. Reuse one machine to exercise actor/cache changes.
func TestSOQLScopeMineUsesExecutionActor(t *testing.T) {
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "Account")
	state := org.Objects["Account"]
	state.Definition.SharingModel = "ReadWrite"
	state.Records = map[storage.ID]storage.Record{}
	owners := []storage.ID{"005000000000101", "005000000000101", "005000000000102"}
	for i, owner := range owners {
		id := storage.ID(fmt.Sprintf("00100000000010%d", i+1))
		state.Records[id] = storage.Record{ID: id, Object: "Account", System: storage.SystemFields{OwnerID: owner}, Fields: map[string]storage.Value{"Name": storage.StringValue(fmt.Sprintf("Owned scope %c", 'A'+i))}}
	}
	org.Objects["Account"] = state
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "OwnedScopeReader", APIVersion: "67.0", Modifiers: []string{"without sharing"}}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id     string
		count  int
		window int
	}{{"005000000000101", 2, 1}, {"005000000000102", 1, 0}, {"005000000000103", 0, 0}, {"005000000000101", 2, 1}} {
		t.Run(tc.id, func(t *testing.T) {
			machine.SetCurrentUser(storage.Record{ID: storage.ID(tc.id), Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(storage.ID(tc.id))}})
			program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
System.assertEquals(%d,Database.countQuery('SELECT COUNT() FROM Account USING SCOPE Mine'));
System.assertEquals(%d,Database.query('SELECT Id FROM Account USING SCOPE Mine').size());
System.assertEquals(%d,Database.query('SELECT Id FROM Account USING SCOPE mInE ORDER BY Name LIMIT 1 OFFSET 1').size());
System.assertEquals(3,Database.query('SELECT Id FROM Account USING SCOPE Everything').size());
System.assertEquals(3,Database.query('SELECT Id FROM Account').size());
`, tc.count, tc.count, tc.window), CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.ExecuteInClass(program, "OwnedScopeReader"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Keep handle routes and unobserved scopes at their prior unsupported boundary.
func TestSOQLScopeMinePreservesHandleAndOtherScopeBoundaries(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	for _, source := range []string{"Database.getQueryLocator('SELECT Id FROM Account USING SCOPE Mine');", "Database.getCursor('SELECT Id FROM Account USING SCOPE Mine');", "Database.getPaginationCursor('SELECT Id FROM Account USING SCOPE Mine');", "Database.query('SELECT Id FROM Account USING SCOPE Team');"} {
		program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = machine.Execute(program); err == nil || !strings.Contains(err.Error(), "USING SCOPE") {
			t.Fatalf("unobserved query shape lost unsupported scope boundary: %s: %v", source, err)
		}
	}
}
