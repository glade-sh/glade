package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestVisualforceHTMLStandardControllerUserModeProjectionAPI67(t *testing.T) {
	const (
		accountID = storage.ID("001000000000001")
		userID    = storage.ID("005000000000002")
		otherID   = storage.ID("005000000000999")
	)
	tests := []struct {
		name      string
		configure func(*testing.T, *VM, *storage.OrgState)
		wantFound bool
		wantError bool
	}{
		{
			name: "ownerCanReadSelectedProjection",
			configure: func(_ *testing.T, _ *VM, org *storage.OrgState) {
				setVisualforceAccountOwner(org, userID)
			},
			wantFound: true,
		},
		{
			name: "privateOtherOwnerIsHidden",
			configure: func(_ *testing.T, _ *VM, org *storage.OrgState) {
				setVisualforceAccountOwner(org, otherID)
			},
		},
		{
			name: "manualReadShareAllowsRecord",
			configure: func(_ *testing.T, _ *VM, org *storage.OrgState) {
				setVisualforceAccountOwner(org, otherID)
				storage.EnsureStandardObject(org, "AccountShare")
				shares := org.Objects["AccountShare"]
				if shares.Records == nil {
					shares.Records = make(map[storage.ID]storage.Record)
				}
				shares.Records["00A000000000001"] = storage.Record{
					ID: "00A000000000001", Object: "AccountShare",
					Fields: map[string]storage.Value{
						"AccountId":          storage.IDValue(accountID),
						"UserOrGroupId":      storage.IDValue(userID),
						"AccountAccessLevel": storage.StringValue("Read"),
					},
				}
				org.Objects["AccountShare"] = shares
			},
			wantFound: true,
		},
		{
			name: "objectReadDenied",
			configure: func(_ *testing.T, _ *VM, org *storage.OrgState) {
				permissions := org.Objects["ObjectPermissions"]
				permission := permissions.Records["110000000000001"]
				permission.Fields["PermissionsRead"] = storage.BooleanValue(false)
				permissions.Records["110000000000001"] = permission
				org.Objects["ObjectPermissions"] = permissions
			},
			wantError: true,
		},
		{
			name: "nameFieldReadDenied",
			configure: func(_ *testing.T, _ *VM, org *storage.OrgState) {
				permissions := org.Objects["FieldPermissions"]
				permission := permissions.Records["0FP000000000001"]
				permission.Fields["PermissionsRead"] = storage.BooleanValue(false)
				permissions.Records["0FP000000000001"] = permission
				org.Objects["FieldPermissions"] = permissions
			},
			wantError: true,
		},
		{
			name: "persistedNameFLSAppliesToPartialSameIDExecutionUser",
			configure: func(_ *testing.T, machine *VM, org *storage.OrgState) {
				setVisualforceAccountOwner(org, userID)
				permissions := org.Objects["FieldPermissions"]
				permission := permissions.Records["0FP000000000001"]
				permission.Fields["PermissionsRead"] = storage.BooleanValue(false)
				permissions.Records["0FP000000000001"] = permission
				org.Objects["FieldPermissions"] = permissions
				machine.SetCurrentUser(storage.Record{ID: userID, Object: "User"})
			},
			wantError: true,
		},
		{
			name: "sameIDCallerViewAllDoesNotOverridePersistedUser",
			configure: func(_ *testing.T, machine *VM, org *storage.OrgState) {
				setVisualforceAccountOwner(org, otherID)
				machine.SetCurrentUser(storage.Record{
					ID: userID, Object: "User",
					Fields: map[string]storage.Value{
						"ProfileId": storage.IDValue("00e000000000002"),
						"PermissionsViewAllData": storage.BooleanValue(true),
					},
				})
			},
		},
		{
			name: "sameIDTestContextPrivilegeMismatchFailsClosed",
			configure: func(_ *testing.T, machine *VM, org *storage.OrgState) {
				setVisualforceAccountOwner(org, otherID)
				machine.EnableTestContext()
				machine.testContext.CurrentUser.Fields["PermissionsViewAllData"] = Bool(true)
			},
			wantError: true,
		},
		{
			name: "missingExecutionUserFailsClosed",
			configure: func(_ *testing.T, machine *VM, _ *storage.OrgState) {
				machine.SetCurrentUser(storage.Record{})
			},
			wantError: true,
		},
		{
			name: "unknownExecutionUserFailsClosed",
			configure: func(_ *testing.T, machine *VM, _ *storage.OrgState) {
				machine.SetCurrentUser(storage.Record{
					ID: otherID, Object: "User",
					Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000002")},
				})
			},
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			machine, org := visualforceRecordAccessFixture()
			tc.configure(t, machine, org)
			got, found, err := machine.ReadVisualforceRecord("Account", accountID)
			if tc.wantError {
				if err == nil || found {
					t.Fatalf("ReadVisualforceRecord found=%t err=%v; want denied read", found, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadVisualforceRecord: %v", err)
			}
			if found != tc.wantFound {
				t.Fatalf("ReadVisualforceRecord found=%t; want %t", found, tc.wantFound)
			}
			if !found {
				return
			}
			if got.Object != "Account" || got.ID != accountID {
				t.Errorf("record identity = (%q, %q); want (Account, %q)", got.Object, got.ID, accountID)
			}
			if len(got.Fields) != 1 {
				t.Errorf("projected fields = %#v; want only Name", got.Fields)
			} else if name := got.Fields["Name"]; name.Kind != storage.ValueString || name.String != "Acme" {
				t.Errorf("projected Name = %#v; want Acme", name)
			}
			if got.System != (storage.SystemFields{}) {
				t.Errorf("raw system fields leaked into HTML record: %#v", got.System)
			}
			if len(got.Children) != 0 || len(got.ParentRelationships) != 0 {
				t.Errorf("raw relationships leaked into HTML record: children=%#v parents=%#v", got.Children, got.ParentRelationships)
			}
			if len(got.ExplicitNulls) != 0 || len(got.LoadedReferences) != 0 {
				t.Errorf("raw record metadata leaked into HTML record: nulls=%#v references=%#v", got.ExplicitNulls, got.LoadedReferences)
			}
		})
	}
}

func visualforceRecordAccessFixture() (*VM, *storage.OrgState) {
	const (
		accountID = storage.ID("001000000000001")
		userID    = storage.ID("005000000000002")
	)
	org := stripInaccessibleTestOrg()
	storage.EnsureStandardObject(&org, "User")
	users := org.Objects["User"]
	if users.Records == nil {
		users.Records = make(map[storage.ID]storage.Record)
	}
	user := storage.Record{
		ID: userID, Object: "User",
		Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000002")},
	}
	users.Records[userID] = user
	org.Objects["User"] = users

	account := org.Objects["Account"]
	account.Definition.SharingModel = "Private"
	row := account.Records[accountID]
	row.System = storage.SystemFields{OwnerID: "005000000000999", CreatedDate: "2026-09-28T00:00:00Z"}
	row.Children = map[string][]storage.Record{
		"Contacts": {{ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"Name": storage.StringValue("Private child")}}},
	}
	account.Records[accountID] = row
	org.Objects["Account"] = account

	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "67.0"}
	machine.SetCurrentUser(user)
	return machine, &org
}

func setVisualforceAccountOwner(org *storage.OrgState, ownerID storage.ID) {
	account := org.Objects["Account"]
	row := account.Records["001000000000001"]
	row.System.OwnerID = ownerID
	account.Records["001000000000001"] = row
	org.Objects["Account"] = account
}
