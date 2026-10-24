package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestGrandchildProjectionFieldPathsIgnoreInsertionOrder(t *testing.T) {
	org := storage.NewOrgState()
	for _, name := range []string{"User", "Account", "Opportunity"} {
		storage.EnsureStandardObject(&org, name)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	paths := []string{"Account.Id", "Account.Owner.Id", "account.owner.Username", "ACCOUNT.Name"}
	values := []storage.Value{
		storage.IDValue("001000000000001"), storage.IDValue("005000000000001"),
		storage.StringValue("orchard@example.com"), storage.StringValue("Orchard North"),
	}
	check := func(t *testing.T, leaf Value) {
		t.Helper()
		branch, ok := leaf.Fields["Account"]
		if !ok || branch.Kind != ValueObject {
			t.Fatalf("canonical Account missing: %v", leaf.Fields)
		}
		for _, name := range []string{"Id", "Name", "Owner"} {
			if _, ok := branch.Fields[name]; !ok {
				t.Errorf("canonical Account.%s missing: %v", name, branch.Fields)
			}
		}
		root := branch.Fields["Owner"]
		for _, name := range []string{"Id", "Username"} {
			if _, ok := root.Fields[name]; !ok {
				t.Errorf("canonical Account.Owner.%s missing: %v", name, root.Fields)
			}
		}
		if len(branch.Fields) != 3 || len(root.Fields) != 2 {
			t.Errorf("duplicate or missing projected fields: branch=%v root=%v", branch.Fields, root.Fields)
		}
		if branch.Type != "Account" || root.Type != "User" ||
			branch.Fields["Name"].Text != "Orchard North" || root.Fields["Username"].Text != "orchard@example.com" ||
			!storage.IDsEqual(sObjectIDFromFields(branch.Fields), "001000000000001") ||
			!storage.IDsEqual(sObjectIDFromFields(root.Fields), "005000000000001") {
			t.Errorf("canonical paths lost parent types or projected values: branch=%v root=%v", branch, root)
		}
	}
	// Each rotation forces a different field to create the shared parents first.
	for rotation := range len(paths) {
		t.Run(fmt.Sprintf("direct_path_rotation_%d", rotation), func(t *testing.T) {
			leaf := Object("Opportunity")
			for i := range len(paths) {
				at := (rotation + i) % len(paths)
				machine.putVMRecordFieldPath(leaf, "Opportunity", paths[at], vmValueFromStorage(values[at]))
			}
			check(t, leaf)
		})
	}
	t.Run("storage_record_materialization", func(t *testing.T) {
		fields := make(map[string]storage.Value, len(paths))
		for i, path := range paths {
			fields[path] = values[i]
		}
		check(t, machine.vmValueFromRecord(storage.Record{Object: "Opportunity", Fields: fields}))
	})
	t.Run("standard_controller_projection_merge", func(t *testing.T) {
		projection := Object("Opportunity")
		branch, root := Object("Account"), Object("User")
		branch.Fields["Id"] = vmValueFromStorage(values[0])
		root.Fields["Id"] = vmValueFromStorage(values[1])
		root.Fields["Username"] = vmValueFromStorage(values[2])
		branch.Fields["Owner"] = root
		branch.Fields["Name"] = vmValueFromStorage(values[3])
		projection.Fields["Account"] = branch
		leaf := Object("Opportunity")
		machine.mergeStandardControllerProjection(&leaf, projection, List(String("account.owner.Username"), String("ACCOUNT.Name")), false)
		branch = leaf.Fields["Account"]
		root = branch.Fields["Owner"]
		if branch.Kind != ValueObject || root.Kind != ValueObject || branch.Fields["Name"].Text != "Orchard North" || root.Fields["Username"].Text != "orchard@example.com" {
			t.Fatalf("controller merge lost canonical parent paths: %v", leaf)
		}
	})
}
