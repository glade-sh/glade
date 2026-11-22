package vm

import (
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

func partnerLeadLifecycleMachine(t *testing.T) *VM {
	t.Helper()
	machine := partnerLeadMultipleShares(t)
	shares := machine.Org.Objects["LeadShare"]
	for key, row := range shares.Records {
		if liveNonOwnerLeadShare(row) {
			shares.DeleteRecord(key)
		}
	}
	machine.Org.Objects["LeadShare"] = shares
	return machine
}

func TestPartnerLeadWarmPassTracksRoleAndGroup(t *testing.T) {
	machine := partnerLeadLifecycleMachine(t)
	org := machine.Org
	end := machine.beginPartnerLeadReadPass()
	defer end()
	if !machine.partnerLeadPrivateExternalDefault("Lead") || !machine.partnerLeadShares.ready {
		t.Fatal("fixture did not warm the captured Lead policy and share index")
	}
	reader := machine.currentUserID()
	const owner = "005000000000001"
	const ownerRole = "00E000000000001"
	const readerRole = "00E000000000002"
	storage.EnsureStandardObject(org, "UserRole")
	users := org.Objects["User"]
	user := users.Records[owner]
	user.Fields["UserRoleId"] = storage.IDValue(ownerRole)
	users.PutRecord(owner, user)
	machine.testContext.CurrentUser.Fields["UserRoleId"] = String(readerRole)
	parent := storage.Record{ID: "a00000000000001", Object: "Parent__c", System: storage.SystemFields{OwnerID: owner}}
	org.Objects[parent.Object] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: parent.Object, SharingModel: "Private", Metadata: map[string]string{"gladeHierarchyAccess": "true"},
	}, Records: map[storage.ID]storage.Record{parent.ID: parent}}
	check := func(stage string, row storage.Record, want bool) {
		t.Helper()
		if got := machine.userModeRecordVisible(row.Object, row, reader); got != want {
			t.Fatalf("%s: visible = %v, want %v", stage, got, want)
		}
	}
	check("unrelated roles", parent, false)
	roles := org.Objects["UserRole"]
	role := storage.Record{ID: ownerRole, Fields: map[string]storage.Value{"ParentRoleId": storage.IDValue(readerRole)}}
	roles.PutRecord(ownerRole, role)
	org.Objects["UserRole"] = roles
	check("owner below reader", parent, true)
	roles.DeleteRecord(ownerRole)
	check("hierarchy removed", parent, false)

	// Direct public-group Account shares are resolved for this same reader.
	for _, object := range []string{"Group", "GroupMember", "Account", "AccountShare"} {
		storage.EnsureStandardObject(org, object)
	}
	account := storage.Record{ID: "001000000000001", Object: "Account", System: storage.SystemFields{OwnerID: owner}}
	accounts := org.Objects["Account"]
	accounts.Definition.SharingModel = "Private"
	accounts.PutRecord(account.ID, account)
	org.Objects["Account"] = accounts
	const group = "00G000000000001"
	groups := org.Objects["Group"]
	groups.PutRecord(group, storage.Record{ID: group, Fields: map[string]storage.Value{"Type": storage.StringValue("Regular")}})
	org.Objects["Group"] = groups
	shares := org.Objects["AccountShare"]
	shares.PutRecord("00r000000000001", storage.Record{Fields: map[string]storage.Value{"AccountId": storage.IDValue(account.ID), "UserOrGroupId": storage.IDValue(group), "AccountAccessLevel": storage.StringValue("Read")}})
	org.Objects["AccountShare"] = shares
	check("empty group", account, false)
	members := org.Objects["GroupMember"]
	const membership = "011000000000001"
	members.PutRecord(membership, storage.Record{ID: membership, Fields: map[string]storage.Value{"GroupId": storage.IDValue(group), "UserOrGroupId": storage.IDValue(storage.ID(reader))}})
	org.Objects["GroupMember"] = members
	check("group grant", account, true)
	members.DeleteRecord(membership)
	check("group revoked", account, false)
	if !machine.partnerLeadPrivateExternalDefault("Lead") || !machine.partnerLeadShares.ready {
		t.Fatal("role or group lookup displaced the warmed Lead pass")
	}
}

func compileLeadLifecycle(t *testing.T, source string) ir.Program {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	return program
}

const readSharedLead = "System.debug([SELECT Id FROM Lead WITH USER_MODE].size());"
const shareFirstLead = "insert as system new LeadShare(LeadId='00Q000000000001',UserOrGroupId='005000000000002',RowCause='Manual',LeadAccessLevel='Read');"
const shareSecondLead = "insert as system new LeadShare(LeadId='00Q000000000002',UserOrGroupId='005000000000002',RowCause='Manual',LeadAccessLevel='Read');"

func TestPartnerLeadSharingAcrossTransactions(t *testing.T) {
	machine := partnerLeadLifecycleMachine(t)
	for _, tc := range []struct {
		body string
		want []string
	}{
		{readSharedLead + shareFirstLead + readSharedLead, []string{"0", "1"}},
		{readSharedLead + "Savepoint sp=Database.setSavepoint();" + shareSecondLead + readSharedLead + "Database.rollback(sp);" + readSharedLead, []string{"1", "200", "1"}},
		{readSharedLead, []string{"1"}},
	} {
		result, err := machine.Execute(compileLeadLifecycle(t, tc.body))
		if err != nil || !slices.Equal(result.Debug, tc.want) || machine.partnerLeadShares != nil {
			t.Fatalf("transaction: observed %v, want %v, active pass %t, err %v", result.Debug, tc.want, machine.partnerLeadShares != nil, err)
		}
	}
}

func TestPartnerLeadSharingAcrossTestMethods(t *testing.T) {
	template := partnerLeadLifecycleMachine(t)
	journal := storage.NewIsolationJournal(template.Org)
	for _, tc := range []struct {
		method, body string
		want         []string
	}{
		{"grant", readSharedLead + shareFirstLead + readSharedLead + shareSecondLead + readSharedLead, []string{"0", "1", "200"}},
		{"isolated", readSharedLead, []string{"0"}},
	} {
		t.Run(tc.method, func(t *testing.T) {
			mark := journal.Mark()
			defer func() {
				if err := journal.Rollback(mark); err != nil {
					t.Fatal(err)
				}
			}()
			end := template.beginPartnerLeadReadPass()
			if !template.partnerLeadPrivateExternalDefault("Lead") || !template.partnerLeadShares.ready {
				t.Fatal("template did not warm the Lead index")
			}
			machine := template.CloneRuntimeFrozenShared(nil)
			end()
			if machine.partnerLeadShares != nil {
				t.Fatal("new method inherited the template's read pass")
			}
			machine.SetRuntimeTemplateOrg(journal.Org())
			machine.SetIsolationJournal(journal)
			machine.EnableTestContext()
			machine.testContext.CurrentUser = template.testContext.CurrentUser
			machine.testContext.RunAsDepth = 1
			const class = "LeadSharingTest"
			name := class + "." + tc.method
			if err := machine.RegisterMethod(Method{Name: name, ClassName: class, IsStatic: true, Access: "public", Modifiers: []string{"isTest"}, APIVersion: "67.0", Program: compileLeadLifecycle(t, tc.body)}); err != nil {
				t.Fatal(err)
			}
			result, err := machine.ExecuteTestInClass(compileLeadLifecycle(t, name+"();"), class)
			if err != nil || !slices.Equal(result.Debug, tc.want) || machine.partnerLeadShares != nil {
				t.Fatalf("method: observed %v, want %v, active pass %t, err %v", result.Debug, tc.want, machine.partnerLeadShares != nil, err)
			}
		})
	}
}

func TestPartnerLeadPassSwitchesNamedReaders(t *testing.T) {
	machine := partnerLeadLifecycleMachine(t)
	defer machine.beginPartnerLeadReadPass()()
	if !machine.partnerLeadPrivateExternalDefault("Lead") {
		t.Fatal("Partner policy did not match the first reader")
	}
	standard := machine.Org.Objects["User"].Records["005000000000003"]
	restore := machine.actAsNamedUser(machine.vmValueFromRecord(standard))
	if machine.partnerLeadPrivateExternalDefault("Lead") {
		t.Fatal("named Standard user inherited the Partner policy")
	}
	restore()
	if !machine.partnerLeadPrivateExternalDefault("Lead") {
		t.Fatal("name switch lost the original reader's policy")
	}
}
