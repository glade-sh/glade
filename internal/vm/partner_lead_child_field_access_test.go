package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPartnerLeadChildLookupReadsCurrentFieldGrant(t *testing.T) {
	machine, _ := partnerLeadChildVisibilityMachine(t)
	query := compileLeadLifecycle(t, `List<Case> rows=[SELECT (SELECT Id FROM SyntheticLeads__r) FROM Case WITH USER_MODE];
		Integer populated=0;
		for (Lead row : rows[0].SyntheticLeads__r) {
			if (row.getPopulatedFieldsAsMap().containsKey('CaseLink__c')) populated++;
		}
		System.debug(populated);`)
	state := machine.Org.Objects["FieldPermissions"]
	grant := state.Records["0FP000000000001"]
	for _, tc := range []struct {
		read bool
		want string
	}{{true, "200"}, {false, "0"}, {true, "200"}} {
		grant.Fields["PermissionsRead"] = storage.BooleanValue(tc.read)
		state.PutRecord(grant.ID, grant)
		machine.Org.Objects["FieldPermissions"] = state
		result, err := machine.Execute(query)
		if err != nil || len(result.Debug) != 1 || result.Debug[0] != tc.want || machine.permissionChecks != nil {
			t.Fatalf("read grant %t: populated %v, want %s, active permission memo %t, err %v", tc.read, result.Debug, tc.want, machine.permissionChecks != nil, err)
		}
	}
}
