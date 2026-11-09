package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// TestURADirectCarryAsyncJobStartsCold checks that a warm direct
// UserRecordAccess row does not reach a Queueable drained by Test.stopTest.
// The job is its own transaction, so its direct query starts from the record
// share (Edit), while the enqueuing block keeps its warm Transfer row.
func TestURADirectCarryAsyncJobStartsCold(t *testing.T) {
	const api = "62.0"
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contract")
	org.APIVersion = api
	var profile storage.ID
	for id, p := range org.Objects["Profile"].Records {
		if p.Fields["Name"].String == "Minimum Access - Salesforce" {
			profile = id
			break
		}
	}
	const actor storage.ID = "005000000000097"
	const perm storage.ID = "0PS000000000097"
	fixture := storage.NewFixture()
	fixture.Objects = []storage.FixtureObject{
		{Name: "User", Records: []storage.FixtureRecord{{ID: actor, Fields: map[string]storage.Value{"Username": storage.StringValue("carryasync@example.invalid"), "ProfileId": storage.IDValue(profile), "LastName": storage.StringValue("CarryAsync")}}}},
		{Name: "PermissionSet", Records: []storage.FixtureRecord{{ID: perm, Fields: map[string]storage.Value{"Name": storage.StringValue("CarryAsync"), "Label": storage.StringValue("CarryAsync")}}}},
		{Name: "ObjectPermissions", Records: []storage.FixtureRecord{
			{Fields: map[string]storage.Value{"ParentId": storage.IDValue(perm), "SObjectType": storage.StringValue("Contract"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true)}},
			{Fields: map[string]storage.Value{"ParentId": storage.IDValue(perm), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true)}},
		}},
		{Name: "PermissionSetAssignment", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(actor), "PermissionSetId": storage.IDValue(perm)}}}},
	}
	if err := storage.ApplyFixture(&org, fixture); err != nil {
		t.Fatal(err)
	}
	contract := org.Objects["Contract"]
	contract.Definition.SharingModel = "ReadWrite"
	other := storage.ID(contract.Definition.KeyPrefix + "000000000001")
	contract.Records = map[storage.ID]storage.Record{other: {ID: other, Object: "Contract", Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: storage.ID(storage.DefaultUserID)}}}
	org.Objects["Contract"] = contract
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(org.Objects["User"].Records[storage.ID(storage.DefaultUserID)])
	machine.EnableTestContext()
	direct := "[SELECT MaxAccessLevel FROM UserRecordAccess WHERE UserId=:uid AND RecordId=:rid][0].MaxAccessLevel"
	job, err := CompileAnonymousWithOptions("Id uid=UserInfo.getUserId();Id rid=[SELECT Id FROM Contract LIMIT 1].Id;System.debug('job='+"+direct+");", CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "CarryWorker", APIVersion: api, Interfaces: []string{"Queueable"}, Methods: map[string]Method{
		"execute": {Name: "CarryWorker.execute", ClassName: "CarryWorker", APIVersion: api, ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: job},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "CarryAsyncTest", APIVersion: api, Modifiers: []string{"isTest"}}); err != nil {
		t.Fatal(err)
	}
	src := "Id uid='" + string(actor) + "';Id rid='" + string(other) + "';System.runAs(new User(Id=uid)){" +
		"Database.query('SELECT UserRecordAccess.MaxAccessLevel FROM Contract WHERE Id=:rid WITH SYSTEM_MODE');" +
		"Test.startTest();System.enqueueJob(new CarryWorker());Test.stopTest();" +
		"System.debug('block='+" + direct + ");}"
	program, err := CompileAnonymousWithOptions(src, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.ExecuteTestInClass(program, "CarryAsyncTest")
	if err != nil {
		t.Fatal(err)
	}
	debug := strings.Join(result.Debug, "\n")
	for _, want := range []string{"job=Edit", "block=Transfer"} {
		if !strings.Contains(debug, want) {
			t.Errorf("missing %q in debug output:\n%s", want, debug)
		}
	}
}
