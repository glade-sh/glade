package storage

import "testing"

func TestCustomPermissionRejectsMissingAssignee(t *testing.T) {
	const sid ID = "0PS000000000001"
	for _, assignee := range []Value{NullValue(), StringValue("")} {
		org := NewOrgState()
		org.Objects["PermissionSet"] = ObjectState{Records: map[ID]Record{sid: {Fields: map[string]Value{"CustomPermissions": StringValue("Bypass")}}}}
		org.Objects["PermissionSetAssignment"] = ObjectState{Records: map[ID]Record{"0Pa000000000001": {Fields: map[string]Value{"AssigneeId": assignee, "PermissionSetId": IDValue(sid)}}}}
		if CustomPermissionGranted(&org, "", nil, "Bypass") {
			t.Fatal("missing assignee granted permission to missing user")
		}
	}
}
