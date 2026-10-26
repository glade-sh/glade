package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecMixedDMLGroupInsertUpdateCanShareTransactions(t *testing.T) {
	// The Apex Developer Guide permits Group insert/update alongside other
	// sObjects: apex_dml_non_mix_sobjects.htm. Group must not classify a
	// transaction as non-setup merely because it is absent from the setup list.
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "setup_then_group",
			source: `
insert new PermissionSetGroup(DeveloperName = 'GrovePermissions', MasterLabel = 'Grove Permissions');
insert new Group(Name = 'Grove', DeveloperName = 'Grove', Type = 'Regular');
`,
		},
		{
			name: "group_then_setup",
			source: `
insert new Group(Name = 'Grove', DeveloperName = 'Grove', Type = 'Regular');
insert new PermissionSetGroup(DeveloperName = 'GrovePermissions', MasterLabel = 'Grove Permissions');
`,
		},
		{
			name: "group_update_with_setup",
			source: `
Group publicGroup = new Group(Name = 'Grove', DeveloperName = 'Grove', Type = 'Regular');
insert publicGroup;
insert new PermissionSetGroup(DeveloperName = 'GrovePermissions', MasterLabel = 'Grove Permissions');
publicGroup.Name = 'New Grove';
update publicGroup;
System.assertEquals('New Grove', [SELECT Name FROM Group WHERE Id = :publicGroup.Id].Name);
`,
		},
		{
			name: "group_insert_update_with_nonsetup",
			source: `
insert new Account(Name = 'Grove');
Group publicGroup = new Group(Name = 'Grove', DeveloperName = 'Grove', Type = 'Regular');
insert publicGroup;
publicGroup.Name = 'New Grove';
update publicGroup;
System.assertEquals('New Grove', [SELECT Name FROM Group WHERE Id = :publicGroup.Id].Name);
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "Group")
			storage.EnsureStandardObject(&org, "PermissionSetGroup")
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExecMixedDMLGroupDoesNotExemptOtherObjects(t *testing.T) {
	for _, source := range []string{
		`insert new PermissionSetGroup(DeveloperName = 'GrovePermissions', MasterLabel = 'Grove Permissions');
insert new Group(Name = 'Grove', DeveloperName = 'Grove', Type = 'Regular');
insert new Account(Name = 'Grove');`,
		`insert new Account(Name = 'Grove');
insert new Group(Name = 'Grove', DeveloperName = 'Grove', Type = 'Regular');
insert new PermissionSetGroup(DeveloperName = 'GrovePermissions', MasterLabel = 'Grove Permissions');`,
	} {
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		org := testDataOrg()
		storage.EnsureDeterministicPlatformData(&org)
		storage.EnsureStandardObject(&org, "Group")
		storage.EnsureStandardObject(&org, "PermissionSetGroup")
		machine := New(nil)
		machine.SetOrg(&org)
		machine.EnableTestContext()
		if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "MIXED_DML_OPERATION") {
			t.Fatalf("expected mixed DML rejection, got %v", err)
		}
	}
}

func TestMixedDMLGroupExceptionKeepsOtherOperationChecks(t *testing.T) {
	// Preserve the existing checks outside the documented insert/update
	// exception; this is a scope control, not a native contract for these ops.
	for _, operation := range []string{"upsert", "delete", "undelete", "merge", "emptyRecycleBin"} {
		t.Run(operation, func(t *testing.T) {
			machine := New(nil)
			machine.EnableTestContext()
			machine.testContext.SetupDML = true
			err := machine.checkMixedDML(operation, []storage.Record{{Object: "Group"}})
			if err == nil || !strings.Contains(err.Error(), "Mixed DML") {
				t.Fatalf("expected existing mixed DML check for %s, got %v", operation, err)
			}
		})
	}
}
