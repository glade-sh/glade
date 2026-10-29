package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native rows (API 62.0 and 67.0, Developer scratch org; see
// testdata/runtimegaps/mixed_dml_psla/org.tsv): PermissionSetLicenseAssign is a
// setup object. Account then PSLA and PSLA then Account both raise
// MIXED_DML_OPERATION naming the earlier object; PSLA with PermissionSetAssignment
// does not raise it.
func TestExecMixedDMLPermissionSetLicenseAssignIsSetupObject(t *testing.T) {
	for _, tc := range []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name: "nonsetup_then_psla",
			source: `
insert new Account(Name = 'Grove');
insert new PermissionSetLicenseAssign(AssigneeId = UserInfo.getUserId(), PermissionSetLicenseId = '0PL000000000001AAA');
`,
			wantErr: "MIXED_DML_OPERATION, DML operation on setup object is not permitted after you have updated a non-setup object (or vice versa): PermissionSetLicenseAssign, original object: Account",
		},
		{
			name: "psla_then_nonsetup",
			source: `
insert new PermissionSetLicenseAssign(AssigneeId = UserInfo.getUserId(), PermissionSetLicenseId = '0PL000000000001AAA');
insert new Account(Name = 'Grove');
`,
			wantErr: "MIXED_DML_OPERATION, DML operation on setup object is not permitted after you have updated a non-setup object (or vice versa): Account, original object: PermissionSetLicenseAssign",
		},
		{
			name: "psla_then_setup",
			source: `
insert new PermissionSetLicenseAssign(AssigneeId = UserInfo.getUserId(), PermissionSetLicenseId = '0PL000000000001AAA');
insert new PermissionSetGroup(DeveloperName = 'GrovePermissions', MasterLabel = 'Grove Permissions');
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
			storage.EnsureStandardObject(&org, "PermissionSetLicenseAssign")
			storage.EnsureStandardObject(&org, "PermissionSetGroup")
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			_, err = machine.Execute(program)
			if tc.wantErr == "" {
				if err != nil && strings.Contains(err.Error(), "MIXED_DML_OPERATION") {
					t.Fatalf("setup-only sequence raised mixed DML: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}
