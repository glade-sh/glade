package vm

import (
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const permissionIdentityUser = "005Aa0000000aBc"
const permissionIdentitySet = "0PSaB0000000cDe"

func permissionIdentityPut(machine *VM, object, id string, fields map[string]storage.Value) {
	state := machine.Org.Objects[object]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	state.Records[storage.ID(id)] = storage.Record{ID: storage.ID(id), Object: object, Fields: fields}
	machine.Org.Objects[object] = state
}

func permissionIdentityMachine(userID, storedSet, assignee, reference, path string) *VM {
	machine := &VM{Org: &storage.OrgState{Objects: make(map[string]storage.ObjectState)}, executionUser: Object("User")}
	machine.executionUser.Fields["Id"] = String(userID)
	fields := make(map[string]storage.Value)
	if path == "inline" {
		fields["CustomPermissions"] = storage.ListValue(storage.StringValue("OwnedOperation"))
	}
	permissionIdentityPut(machine, "PermissionSet", storedSet, fields)
	permissionIdentityPut(machine, "PermissionSetAssignment", "0Pa000000000001", map[string]storage.Value{
		"AssigneeId": storage.IDValue(storage.ID(assignee)), "PermissionSetId": storage.IDValue(storage.ID(reference)),
	})
	if path == "entity" {
		permissionIdentityPut(machine, "CustomPermission", "0CP000000000001", map[string]storage.Value{"DeveloperName": storage.StringValue("OwnedOperation")})
		permissionIdentityPut(machine, "SetupEntityAccess", "0J0000000000001", map[string]storage.Value{
			"ParentId": storage.IDValue(storage.ID(storedSet)), "SetupEntityId": storage.IDValue("0CP000000000001"),
		})
	}
	return machine
}

func TestPermissionAssignmentEquivalentIDForms(t *testing.T) {
	set15 := permissionIdentitySet
	user18, set18 := apexIDTo18(permissionIdentityUser), apexIDTo18(set15)
	for _, path := range []string{"inline", "entity"} {
		for _, shape := range []struct {
			name, user, stored, assignee, reference string
		}{
			{"set15", user18, set15, user18, set15},
			{"setReference18", user18, set15, user18, set18},
			{"setStored18", user18, set18, user18, set15},
			{"set18", user18, set18, user18, set18},
		} {
			t.Run(path+"/"+shape.name, func(t *testing.T) {
				machine := permissionIdentityMachine(shape.user, shape.stored, shape.assignee, shape.reference, path)
				if got := machine.assignedPermissionSetIDs(shape.user); !reflect.DeepEqual(got, []string{shape.stored}) {
					t.Errorf("assigned IDs = %q, want stored ID %q", got, shape.stored)
				}
				if !machine.currentUserHasPermission("OwnedOperation") {
					t.Error("explicit assigned custom permission was lost")
				}
				if machine.currentUserHasPermission("AbsentOperation") {
					t.Error("undeclared custom permission was granted")
				}
			})
		}
	}
}

func TestPermissionAssignmentIdentityDenialControls(t *testing.T) {
	for _, path := range []string{"inline", "entity"} {
		for _, condition := range []string{"otherUser", "caseDistinctUser", "caseDistinctSet", "missingSet", "missingPermission", "disabledPermission", "wrongPermission", "unassigned"} {
			t.Run(path+"/"+condition, func(t *testing.T) {
				machine := permissionIdentityMachine(permissionIdentityUser, permissionIdentitySet, permissionIdentityUser, permissionIdentitySet, path)
				switch condition {
				case "otherUser", "caseDistinctUser", "caseDistinctSet":
					state := machine.Org.Objects["PermissionSetAssignment"]
					row := state.Records["0Pa000000000001"]
					if condition == "otherUser" {
						row.Fields["AssigneeId"] = storage.IDValue("005Aa0000000aBd")
					} else if condition == "caseDistinctUser" {
						row.Fields["AssigneeId"] = storage.IDValue(storage.ID(strings.Replace(permissionIdentityUser, "Aa", "aa", 1)))
					} else {
						row.Fields["PermissionSetId"] = storage.IDValue(storage.ID(strings.Replace(permissionIdentitySet, "aB", "ab", 1)))
					}
					state.Records[row.ID] = row
					machine.Org.Objects["PermissionSetAssignment"] = state
				case "missingSet":
					delete(machine.Org.Objects["PermissionSet"].Records, storage.ID(permissionIdentitySet))
				case "unassigned":
					delete(machine.Org.Objects["PermissionSetAssignment"].Records, "0Pa000000000001")
				default:
					if path == "inline" {
						row := machine.Org.Objects["PermissionSet"].Records[storage.ID(permissionIdentitySet)]
						switch condition {
						case "missingPermission":
							delete(row.Fields, "CustomPermissions")
						case "disabledPermission":
							row.Fields["CustomPermissions"] = storage.BooleanValue(false)
						case "wrongPermission":
							row.Fields["CustomPermissions"] = storage.StringValue("DifferentOperation")
						}
					} else if condition == "wrongPermission" {
						machine.Org.Objects["CustomPermission"].Records["0CP000000000001"].Fields["DeveloperName"] = storage.StringValue("DifferentOperation")
					} else if condition == "missingPermission" {
						delete(machine.Org.Objects["CustomPermission"].Records, "0CP000000000001")
					} else {
						delete(machine.Org.Objects["SetupEntityAccess"].Records, "0J0000000000001")
					}
				}
				if machine.currentUserHasPermission("OwnedOperation") {
					t.Fatal("ungranted custom permission was allowed")
				}
			})
		}
	}
}

func TestPermissionAssignmentCanonicalizationPreservesReferences(t *testing.T) {
	machine := permissionIdentityMachine(permissionIdentityUser, permissionIdentitySet, permissionIdentityUser, apexIDTo18(permissionIdentitySet), "inline")
	permissionIdentityPut(machine, "PermissionSetAssignment", "0Pa000000000002", map[string]storage.Value{
		"AssigneeId": storage.IDValue(storage.ID(permissionIdentityUser)), "PermissionSetId": storage.IDValue(storage.ID(permissionIdentitySet)),
	})
	missing := apexIDTo18("0PSaB0000000cDf")
	permissionIdentityPut(machine, "PermissionSetAssignment", "0Pa000000000003", map[string]storage.Value{
		"AssigneeId": storage.IDValue(storage.ID(permissionIdentityUser)), "PermissionSetId": storage.IDValue(storage.ID(missing)),
	})
	counts := make(map[string]int)
	for _, id := range machine.assignedPermissionSetIDs(permissionIdentityUser) {
		counts[id]++
	}
	if want := map[string]int{permissionIdentitySet: 2, missing: 1}; !reflect.DeepEqual(counts, want) {
		t.Fatalf("assigned references = %v, want %v", counts, want)
	}
}

func TestPermissionAssignmentIdentityObjectFieldAndSharingControls(t *testing.T) {
	for _, shape := range []string{"assigned", "unassigned", "otherUser", "viewAll"} {
		t.Run(shape, func(t *testing.T) {
			machine := recordSharingControlMachine(t, "Private")
			machine.testContext.CurrentUser.Fields["Id"] = String(apexIDTo18(permissionIdentityUser))
			// Replace the fixture's explicit profile grants with denials so only
			// this assignment can supply the object and field grants.
			for _, row := range machine.Org.Objects["ObjectPermissions"].Records {
				row.Fields["PermissionsRead"] = storage.BooleanValue(false)
			}
			for _, row := range machine.Org.Objects["FieldPermissions"].Records {
				row.Fields["PermissionsRead"] = storage.BooleanValue(false)
			}
			permissionIdentityPut(machine, "PermissionSet", permissionIdentitySet, map[string]storage.Value{})
			assignee := apexIDTo18(permissionIdentityUser)
			if shape == "otherUser" {
				assignee = apexIDTo18("005Aa0000000aBd")
			}
			if shape != "unassigned" {
				permissionIdentityPut(machine, "PermissionSetAssignment", "0Pa000000000001", map[string]storage.Value{
					"AssigneeId": storage.IDValue(storage.ID(assignee)), "PermissionSetId": storage.IDValue(storage.ID(apexIDTo18(permissionIdentitySet))),
				})
			}
			permissionIdentityPut(machine, "ObjectPermissions", "110000000000099", map[string]storage.Value{
				"ParentId": storage.IDValue(storage.ID(permissionIdentitySet)), "SObjectType": storage.StringValue("Widget__c"),
				"PermissionsRead": storage.BooleanValue(true), "PermissionsViewAllRecords": storage.BooleanValue(shape == "viewAll"),
			})
			permissionIdentityPut(machine, "FieldPermissions", "0FP000000000099", map[string]storage.Value{
				"ParentId": storage.IDValue(storage.ID(permissionIdentitySet)), "SObjectType": storage.StringValue("Widget__c"),
				"Field": storage.StringValue("Widget__c.Amount__c"), "PermissionsRead": storage.BooleanValue(true),
			})
			wantGrant := shape == "assigned" || shape == "viewAll"
			if got := machine.currentUserObjectPermission("Widget__c", "isAccessible"); got != wantGrant {
				t.Errorf("object read = %v, want %v", got, wantGrant)
			}
			if got := machine.currentUserFieldPermission("Widget__c", "Amount__c", "isAccessible"); got != wantGrant {
				t.Errorf("field read = %v, want %v", got, wantGrant)
			}
			if machine.currentUserBypassesRecordSharing() {
				t.Fatal("assignment introduced a global sharing bypass")
			}
			wantViewAll := shape == "viewAll"
			if got := machine.currentUserObjectRecordGrant("Widget__c", "PermissionsViewAllRecords"); got != wantViewAll {
				t.Errorf("ViewAll = %v, want %v", got, wantViewAll)
			}
			for _, record := range machine.Org.Objects["Widget__c"].Records {
				if got := machine.userModeRecordVisible("Widget__c", record, apexIDTo18(permissionIdentityUser)); got != wantViewAll {
					t.Errorf("foreign private record visible = %v, want %v", got, wantViewAll)
				}
			}
		})
	}
}
