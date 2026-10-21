package vm

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// standard_object_sharing.json records Account/Opportunity ReadWrite,
// Case ReadWriteTransfer and Contact ControlledByParent defaults. The
// runas_default_security harness supplies that Account model explicitly.
func TestStandardObjectDefaultSharingWritePolicy(t *testing.T) {
	for _, tc := range []struct {
		object, model string
		write         bool
	}{
		{"Account", "", true}, {"Opportunity", "", true}, {"Case", "", true},
		{"Contact", "", false}, {"Campaign", "", false}, {"OwnedEntry__c", "", false},
		{"Account", "ReadWrite", true}, {"Account", "PublicReadWrite", true},
		{"Account", "Private", false}, {"Account", "Read", false},
		{"Account", "ReadOnly", false}, {"Account", "ControlledByParent", false},
	} {
		t.Run(tc.object+"/"+tc.model, func(t *testing.T) {
			machine, record := defaultSharingWriteFixture(tc.object, tc.model, false)
			for _, operation := range []string{"update", "upsert", "delete", "undelete"} {
				want := tc.write && (operation == "update" || operation == "upsert")
				if got := machine.currentUserCanWriteRecord(tc.object, record, "005000000000002", operation); got != want {
					t.Fatalf("%s another-owner write = %v, want %v", operation, got, want)
				}
				if !machine.currentUserCanWriteRecord(tc.object, record, string(record.System.OwnerID), operation) {
					t.Fatalf("%s denied the owner", operation)
				}
			}
		})
	}
}

// PENDING-ROWS: the captured BusinessAccount default model is extended to the
// omitted-model PersonAccount preset. Exact API24 test entry, API36 unsaved
// runAs-user construction, Standard Platform User and PersonAccount native
// results remain pending; this unit control asserts only the record-write gate.
func TestPersonAccountDefaultSharingWritePendingRows(t *testing.T) {
	for _, tc := range []struct {
		model string
		write bool
	}{{"", true}, {"ReadWrite", true}, {"Private", false}, {"ReadOnly", false}, {"ControlledByParent", false}} {
		t.Run(tc.model, func(t *testing.T) {
			machine, record := defaultSharingWriteFixture("Account", tc.model, true)
			if got := machine.currentUserCanWriteRecord("Account", record, "005000000000002", "update"); got != tc.write {
				t.Fatalf("PersonAccount another-owner update = %v, want %v", got, tc.write)
			}
		})
	}
}

// runas_default_security/matrix.json observes readable-only Account DEFAULT
// updates failing with TypeException at API67/68 inside runAs. A public record
// grant must not bypass this source-version/user-mode permission boundary.
func TestStandardDefaultSharingWritePreservesUserModeDenial(t *testing.T) {
	for _, api := range []string{"67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			org := stripInaccessibleTestOrg()
			account := org.Objects["Account"]
			account.Definition.SharingModel = ""
			stored := account.Records["001000000000001"]
			stored.System.OwnerID = "005000000000001"
			account.Records[stored.ID] = stored
			org.Objects["Account"] = account
			permissions := org.Objects["ObjectPermissions"]
			for id, record := range permissions.Records {
				record.Fields["PermissionsCreate"] = storage.BooleanValue(false)
				record.Fields["PermissionsEdit"] = storage.BooleanValue(false)
				record.Fields["PermissionsDelete"] = storage.BooleanValue(false)
				permissions.Records[id] = record
			}
			org.Objects["ObjectPermissions"] = permissions
			machine := New(nil)
			machine.SetOrg(&org)
			// Isolate the direct runAs DEFAULT-DML guard with its actual
			// test-context preconditions; package-version runAs stays absent.
			machine.EnableTestContext()
			machine.testContext.CurrentUser = stripInaccessibleTestUser()
			machine.testContext.RunAsDepth = 1
			machine.currentClass = "SharingWriteProbe"
			machine.currentMethod = Method{APIVersion: api}
			program, err := CompileAnonymousWithOptions("Account target=new Account(Id='001000000000001',Name='changed'); update target;", CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || exceptionQualifiedTypeName(runtimeErr.Type) != "System.TypeException" ||
				runtimeErr.ExceptionMessage() != "DML operation UPDATE not allowed on Account" {
				t.Fatalf("runAs readable-only default update = %v, want exact TypeException denial", err)
			}
			if got := org.Objects["Account"].Records[stored.ID].Fields["Name"].String; got != "Acme" {
				t.Fatalf("denied update changed the stored record to %q", got)
			}
		})
	}
}

func defaultSharingWriteFixture(object, model string, person bool) (*VM, storage.Record) {
	org := storage.NewOrgState()
	org.Objects[object] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: object, SharingModel: model},
		Records:    make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000002", Object: "User"})
	record := storage.Record{
		ID: "001000000000001", Object: object,
		System: storage.SystemFields{OwnerID: "005000000000001"},
		Fields: map[string]storage.Value{"Name": storage.StringValue("Owned entry")},
	}
	if person {
		record.Fields["IsPersonAccount"] = storage.BooleanValue(true)
	}
	return machine, record
}
