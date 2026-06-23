package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecApprovalProcessLocalEngineSubmitCreatesInstanceAndWorkitem(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Needs Approval');
insert account;
Approval.ProcessSubmitRequest request = new Approval.ProcessSubmitRequest();
request.setObjectId(account.Id);
request.setComments('local submit');
request.setSkipEntryCriteria(true);
request.setNextApproverIds(new List<Id>{ '005000000000002' });
Approval.ProcessResult result = Approval.process(request);
System.assertEquals(true, result.isSuccess());
System.assertEquals(account.Id, result.getEntityId());
System.assertEquals('Pending', result.getInstanceStatus());
System.assertEquals(0, result.getErrors().size());
System.assertNotEquals(null, result.getInstanceId());
System.assertEquals(1, result.getNewWorkitemIds().size());
System.assertEquals('005000000000002', result.getActorIds().get(0));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	testSeedApprovalMetadata(t, &org)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	instance := testOnlyRecord(t, machine.Org, "ProcessInstance")
	if got := storageStringField(instance, "Status"); got != "Pending" {
		t.Fatalf("ProcessInstance.Status = %q, want Pending", got)
	}
	if got := storageStringField(instance, "ProcessDefinitionId"); got != "04a000000000001AAA" {
		t.Fatalf("ProcessInstance.ProcessDefinitionId = %q", got)
	}
	workitem := testOnlyRecord(t, machine.Org, "ProcessInstanceWorkitem")
	if got := storageStringField(workitem, "ProcessInstanceId"); got != string(instance.ID) {
		t.Fatalf("ProcessInstanceWorkitem.ProcessInstanceId = %q, want %s", got, instance.ID)
	}
	if got := storageStringField(workitem, "ActorId"); got != "005000000000002" {
		t.Fatalf("ProcessInstanceWorkitem.ActorId = %q", got)
	}
}

func TestExecApprovalProcessLocalEngineApproveAndReject(t *testing.T) {
	tests := []struct {
		name   string
		action string
		status string
	}{
		{name: "approve", action: "Approve", status: "Approved"},
		{name: "reject", action: "Reject", status: "Rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(`
Account account = new Account(Name = 'Needs Approval');
insert account;
Approval.ProcessSubmitRequest submit = new Approval.ProcessSubmitRequest();
submit.setObjectId(account.Id);
submit.setSkipEntryCriteria(true);
submit.setNextApproverIds(new List<Id>{ '005000000000002' });
Approval.ProcessResult submitted = Approval.process(submit);
Approval.ProcessWorkitemRequest work = new Approval.ProcessWorkitemRequest();
work.setWorkitemId(submitted.getNewWorkitemIds().get(0));
work.setAction('` + tt.action + `');
work.setComments('finished locally');
Approval.ProcessResult finished = Approval.process(work);
System.assertEquals(true, finished.isSuccess());
System.assertEquals('` + tt.status + `', finished.getInstanceStatus());
System.assertEquals(0, finished.getNewWorkitemIds().size());
`)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			testSeedApprovalMetadata(t, &org)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			instance := testOnlyRecord(t, machine.Org, "ProcessInstance")
			if got := storageStringField(instance, "Status"); got != tt.status {
				t.Fatalf("ProcessInstance.Status = %q, want %s", got, tt.status)
			}
			workitem := testOnlyRecord(t, machine.Org, "ProcessInstanceWorkitem")
			if got := strings.ToLower(storageStringField(workitem, "IsDeleted")); got != "true" {
				t.Fatalf("ProcessInstanceWorkitem.IsDeleted = %q, want true", got)
			}
		})
	}
}

func TestExecApprovalProcessLocalEngineMissingMetadataAllOrNoneFalse(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Needs Approval');
insert account;
Approval.ProcessSubmitRequest request = new Approval.ProcessSubmitRequest();
request.setObjectId(account.Id);
Approval.ProcessResult result = Approval.process(request, false);
System.assertEquals(false, result.isSuccess());
System.assertEquals(1, result.getErrors().size());
System.assert(result.getErrors().get(0).getMessage().contains('ProcessDefinition'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "ProcessDefinition")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecApprovalProcessLocalEngineMissingMetadataAllOrNoneTrue(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Needs Approval');
insert account;
Approval.ProcessSubmitRequest request = new Approval.ProcessSubmitRequest();
request.setObjectId(account.Id);
Boolean caught = false;
try {
	Approval.process(request, true);
} catch (DmlException e) {
	caught = e.getMessage().contains('ProcessDefinition');
}
System.assertEquals(true, caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "ProcessDefinition")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecApprovalProcessLocalEngineListProcessesRequestsInOrder(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	testSeedApprovalMetadata(t, &org)
	testSeedApprovalAccount(t, &org, "001000000000101AAA", "First")
	testSeedApprovalAccount(t, &org, "001000000000102AAA", "Second")
	machine.SetOrg(&org)

	first := Object("Approval.ProcessSubmitRequest")
	first.Fields["ObjectId"] = platformScalar("Id", "001000000000101AAA")
	first.Fields["SkipEntryCriteria"] = Bool(true)
	first.Fields["NextApproverIds"] = List(platformScalar("Id", "005000000000002"))
	second := Object("Approval.ProcessSubmitRequest")
	second.Fields["ObjectId"] = platformScalar("Id", "001000000000102AAA")
	second.Fields["SkipEntryCriteria"] = Bool(true)
	second.Fields["NextApproverIds"] = List(platformScalar("Id", "005000000000002"))
	requests := typedList("List<Approval.ProcessRequest>")
	requests.List = append(requests.List, first, second)

	result, err := machine.executeApprovalProcess([]Value{requests})
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != ValueList || !strings.EqualFold(result.Type, "List<Approval.ProcessResult>") {
		t.Fatalf("result = %#v, want typed List<Approval.ProcessResult>", result)
	}
	if len(result.List) != 2 {
		t.Fatalf("result size = %d, want 2", len(result.List))
	}
	if got := testApprovalResultID(t, result.List[0], "entityId"); got != "001000000000101AAA" {
		t.Fatalf("first entityId = %q, want first account", got)
	}
	if got := testApprovalResultID(t, result.List[1], "entityId"); got != "001000000000102AAA" {
		t.Fatalf("second entityId = %q, want second account", got)
	}
}

func TestExecApprovalProcessLocalEngineListAllOrNoneFalseReturnsPerRequestResults(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	testSeedApprovalMetadata(t, &org)
	testSeedApprovalAccount(t, &org, "001000000000101AAA", "First")
	machine.SetOrg(&org)

	good := Object("Approval.ProcessSubmitRequest")
	good.Fields["ObjectId"] = platformScalar("Id", "001000000000101AAA")
	good.Fields["SkipEntryCriteria"] = Bool(true)
	good.Fields["NextApproverIds"] = List(platformScalar("Id", "005000000000002"))
	bad := Object("Approval.ProcessSubmitRequest")
	requests := typedList("List<Approval.ProcessRequest>")
	requests.List = append(requests.List, good, bad)

	result, err := machine.executeApprovalProcess([]Value{requests, Bool(false)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != ValueList || !strings.EqualFold(result.Type, "List<Approval.ProcessResult>") {
		t.Fatalf("result = %#v, want typed List<Approval.ProcessResult>", result)
	}
	if len(result.List) != 2 {
		t.Fatalf("result size = %d, want 2", len(result.List))
	}
	if success := result.List[0].Fields["success"].Bool; !success {
		t.Fatalf("first success = false, want true")
	}
	if success := result.List[1].Fields["success"].Bool; success {
		t.Fatalf("second success = true, want false")
	}
	if got := len(result.List[1].Fields["errors"].List); got != 1 {
		t.Fatalf("second errors = %d, want 1", got)
	}
}

func TestExecApprovalProcessLocalEngineListAllOrNoneTrueRollsBackEarlierRequests(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	testSeedApprovalMetadata(t, &org)
	testSeedApprovalAccount(t, &org, "001000000000101AAA", "First")
	machine.SetOrg(&org)

	good := Object("Approval.ProcessSubmitRequest")
	good.Fields["ObjectId"] = platformScalar("Id", "001000000000101AAA")
	good.Fields["SkipEntryCriteria"] = Bool(true)
	good.Fields["NextApproverIds"] = List(platformScalar("Id", "005000000000002"))
	bad := Object("Approval.ProcessSubmitRequest")
	requests := typedList("List<Approval.ProcessRequest>")
	requests.List = append(requests.List, good, bad)

	_, err := machine.executeApprovalProcess([]Value{requests})
	if err == nil {
		t.Fatal("expected allOrNone=true list failure")
	}
	if object, ok := machine.Org.Objects["ProcessInstance"]; ok && len(object.Records) != 0 {
		t.Fatalf("ProcessInstance records survived rollback: %#v", object.Records)
	}
	if object, ok := machine.Org.Objects["ProcessInstanceWorkitem"]; ok && len(object.Records) != 0 {
		t.Fatalf("ProcessInstanceWorkitem records survived rollback: %#v", object.Records)
	}
}

func TestExecApprovalProcessHostedRoutingStaysUnsupported(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Needs Approval');
insert account;
Approval.ProcessSubmitRequest request = new Approval.ProcessSubmitRequest();
request.setObjectId(account.Id);
request.setSkipEntryCriteria(false);
Approval.process(request);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	testSeedApprovalMetadata(t, &org)
	machine.SetOrg(&org)
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != `unsupported call "Approval.process hosted approval engine routing"` {
		t.Fatalf("error = %#v, want hosted routing UnsupportedFeature", err)
	}
}

func testSeedApprovalMetadata(t *testing.T, org *storage.OrgState) {
	t.Helper()
	storage.EnsureStandardObject(org, "ProcessDefinition")
	definitions := org.Objects["ProcessDefinition"]
	definitions.Records["04a000000000001AAA"] = storage.Record{
		ID:     "04a000000000001AAA",
		Object: "ProcessDefinition",
		Fields: map[string]storage.Value{
			"Id":            storage.IDValue("04a000000000001AAA"),
			"DeveloperName": storage.StringValue("Account_Local_Approval"),
			"Name":          storage.StringValue("Account Local Approval"),
			"State":         storage.StringValue("Active"),
			"TableEnumOrId": storage.StringValue("Account"),
			"Type":          storage.StringValue("Approval"),
		},
	}
	org.Objects["ProcessDefinition"] = definitions

	storage.EnsureStandardObject(org, "ProcessNode")
	nodes := org.Objects["ProcessNode"]
	nodes.Records["04h000000000001AAA"] = storage.Record{
		ID:     "04h000000000001AAA",
		Object: "ProcessNode",
		Fields: map[string]storage.Value{
			"Id":                  storage.IDValue("04h000000000001AAA"),
			"DeveloperName":       storage.StringValue("Manager_Approval"),
			"Name":                storage.StringValue("Manager Approval"),
			"ProcessDefinitionId": storage.IDValue("04a000000000001AAA"),
			"ActorId":             storage.IDValue("005000000000002"),
		},
	}
	org.Objects["ProcessNode"] = nodes
}

func testSeedApprovalAccount(t *testing.T, org *storage.OrgState, id storage.ID, name string) {
	t.Helper()
	storage.EnsureStandardObject(org, "Account")
	accounts := org.Objects["Account"]
	accounts.Records[id] = storage.Record{
		ID:     id,
		Object: "Account",
		Fields: map[string]storage.Value{
			"Id":   storage.IDValue(id),
			"Name": storage.StringValue(name),
		},
	}
	org.Objects["Account"] = accounts
}

func testApprovalResultID(t *testing.T, result Value, field string) string {
	t.Helper()
	value, ok := result.Fields[field]
	if !ok {
		t.Fatalf("missing result field %s", field)
	}
	text, ok := idValueText(value)
	if !ok {
		t.Fatalf("result field %s = %#v, want Id", field, value)
	}
	return text
}

func testOnlyRecord(t *testing.T, org *storage.OrgState, objectName string) storage.Record {
	t.Helper()
	object, ok := org.Objects[objectName]
	if !ok {
		t.Fatalf("%s missing", objectName)
	}
	if len(object.Records) != 1 {
		t.Fatalf("%s records = %d, want 1", objectName, len(object.Records))
	}
	for _, record := range object.Records {
		return record
	}
	panic("unreachable")
}
