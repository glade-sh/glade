package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRelationshipContractFamilyApexShapesAtDiscreteAPIs(t *testing.T) {
	const source = `
List<Contract__c> contracts = [
	SELECT Id, Name__c, (SELECT Id, Label__c FROM Items__r)
	FROM Contract__c ORDER BY Name__c
];
System.assertEquals(2, contracts.size());
System.assertEquals(1, contracts[0].Items__r.size());
System.assertEquals('Linked item', contracts[0].Items__r[0].Label__c);
System.assertEquals(0, contracts[1].Items__r.size());

List<Item__c> items = [SELECT Id, Label__c, Contract__r.Name__c FROM Item__c ORDER BY Label__c];
System.assertEquals('Alpha contract', items[0].Contract__r.Name__c);
System.assertEquals(null, items[1].Contract__r);
List<Item__c> nullParents = [SELECT Id FROM Item__c WHERE Contract__r.Name__c = null];
System.assertEquals(1, nullParents.size());

Boolean invalidRelationshipCaught = false;
try {
	Database.query('SELECT Missing__r.Name__c FROM Item__c');
} catch (QueryException qe) {
	invalidRelationshipCaught = true;
}
System.assertEquals(true, invalidRelationshipCaught);

List<Contract__History> historyRows = [SELECT Id, Parent.Name__c FROM Contract__History];
System.assertEquals(1, historyRows.size());
System.assertEquals('Alpha contract', historyRows[0].Parent.Name__c);

List<Task> contactTasks = [SELECT Id, Who.Type FROM Task WHERE Who.Type = 'Contact' WITH USER_MODE];
System.assertEquals(1, contactTasks.size());
System.assertEquals('Contact', contactTasks[0].Who.Type);
List<Task> leadTasks = [SELECT Id, Who.Type FROM Task WHERE Who.Type = 'Lead' WITH USER_MODE];
System.assertEquals(1, leadTasks.size());
System.assertEquals('Lead', leadTasks[0].Who.Type);
`
	for _, apiVersion := range []string{"62.0", "67.0"} {
		t.Run(apiVersion, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: apiVersion})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := relationshipContractApexOrg()
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRelationshipPolymorphicTypeUserModeSecurityControlsAtDiscreteAPIs(t *testing.T) {
	const source = `
List<Task> contactTasks = [SELECT Id, Who.Type FROM Task WHERE Who.Type = 'Contact' WITH USER_MODE];
System.assertEquals(1, contactTasks.size());
System.assertEquals('Contact', contactTasks[0].Who.Type);
List<Task> leadTasks = [SELECT Id, Who.Type FROM Task WHERE Who.Type = 'Lead' WITH USER_MODE];
System.assertEquals(1, leadTasks.size());
System.assertEquals('Lead', leadTasks[0].Who.Type);

Boolean targetFieldDenied = false;
try {
	Database.query('SELECT Id, Who.Secret__c FROM Task WITH USER_MODE');
} catch (QueryException qe) {
	targetFieldDenied = qe.getMessage().contains('Who.Secret__c') && qe.getMessage().contains('USER_MODE');
}
System.assertEquals(true, targetFieldDenied);

Boolean nonPolymorphicTypeDenied = false;
try {
	Database.query('SELECT Id, Account.Type FROM Task WITH USER_MODE');
} catch (QueryException qe) {
	nonPolymorphicTypeDenied = qe.getMessage().contains('Account.Type') && qe.getMessage().contains('USER_MODE');
}
System.assertEquals(true, nonPolymorphicTypeDenied);

Boolean unknownRelationshipDenied = false;
try {
	Database.query('SELECT Id, Missing__r.Name__c FROM Task WITH USER_MODE');
} catch (QueryException qe) {
	unknownRelationshipDenied = qe.getMessage().contains('Missing__r');
}
System.assertEquals(true, unknownRelationshipDenied);
`
	const deniedReferenceFieldSource = `
Boolean whoIdDenied = false;
try {
	Database.query('SELECT Id, Who.Type FROM Task WITH USER_MODE');
} catch (QueryException qe) {
	whoIdDenied = qe.getMessage().contains('WhoId') && qe.getMessage().contains('Task');
}
System.assertEquals(true, whoIdDenied);
`
	const deniedRootObjectSource = `
Boolean taskDenied = false;
try {
	Database.query('SELECT Id, Who.Type FROM Task WITH USER_MODE');
} catch (QueryException qe) {
	taskDenied = qe.getMessage().contains('Task') && qe.getMessage().contains('USER_MODE');
}
System.assertEquals(true, taskDenied);
`
	const sharingSource = `
List<Task> tasks = [SELECT Id, Who.Type FROM Task WITH USER_MODE];
System.assertEquals(0, tasks.size());
`
	for _, apiVersion := range []string{"62.0", "67.0"} {
		t.Run(apiVersion, func(t *testing.T) {
			run := func(name, body string, readWhoID, readTask, denyTaskAtProfile, privateTasks bool) {
				t.Run(name, func(t *testing.T) {
					program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: apiVersion})
					if err != nil {
						t.Fatal(err)
					}
					org, user := relationshipUserModeApexOrg(readWhoID, readTask, denyTaskAtProfile, privateTasks)
					machine := New(nil)
					machine.SetOrg(&org)
					machine.executionUser = user
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
			run("pseudo_type_and_field_controls", source, true, true, false, false)
			run("reference_field_denied", deniedReferenceFieldSource, false, true, false, false)
			run("root_object_denied", deniedRootObjectSource, true, false, true, false)
			run("record_sharing_still_applies", sharingSource, true, true, false, true)
		})
	}
}

func relationshipContractApexOrg() storage.OrgState {
	org := storage.NewOrgState()
	contractDefinition := storage.ObjectDefinition{
		APIName: "Contract__c",
		Fields:  map[string]storage.Field{"Name__c": {APIName: "Name__c", Type: storage.FieldString}},
		Relations: []storage.Relationship{{
			Field: "Contract__c", ParentObjects: []string{"Contract__c"}, ParentRelationship: "Contract__r", ChildRelationship: "Items__r",
		}},
	}
	itemDefinition := storage.ObjectDefinition{
		APIName: "Item__c",
		Fields: map[string]storage.Field{
			"Label__c":    {APIName: "Label__c", Type: storage.FieldString},
			"Contract__c": {APIName: "Contract__c", Type: storage.FieldReference, ReferenceTo: []string{"Contract__c"}, RelationshipName: "Contract__r", ChildRelationshipName: "Items__r"},
		},
		Relations: []storage.Relationship{{
			Field: "Contract__c", ParentObjects: []string{"Contract__c"}, ParentRelationship: "Contract__r", ChildRelationship: "Items__r",
		}},
	}
	storage.EnsureStandardObjectFields(&contractDefinition)
	storage.EnsureStandardObjectFields(&itemDefinition)
	org.Objects["Contract__c"] = storage.ObjectState{
		Definition: contractDefinition,
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {ID: "a00000000000001", Object: "Contract__c", Fields: map[string]storage.Value{"Name__c": storage.StringValue("Alpha contract")}},
			"a00000000000002": {ID: "a00000000000002", Object: "Contract__c", Fields: map[string]storage.Value{"Name__c": storage.StringValue("Empty contract")}},
		},
	}
	org.Objects["Item__c"] = storage.ObjectState{
		Definition: itemDefinition,
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {ID: "a01000000000001", Object: "Item__c", Fields: map[string]storage.Value{"Label__c": storage.StringValue("Linked item"), "Contract__c": storage.IDValue("a00000000000001")}},
			"a01000000000002": {ID: "a01000000000002", Object: "Item__c", Fields: map[string]storage.Value{"Label__c": storage.StringValue("Orphan item")}},
		},
	}
	storage.EnsureStandardObject(&org, "Contact")
	storage.EnsureStandardObject(&org, "Lead")
	storage.EnsureStandardObject(&org, "Task")
	contactID := storage.ID("003000000000001")
	leadID := storage.ID("00Q000000000001")
	contact := org.Objects["Contact"]
	contact.Records[contactID] = storage.Record{ID: contactID, Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Polymorphic contact")}}
	org.Objects["Contact"] = contact
	lead := org.Objects["Lead"]
	lead.Records[leadID] = storage.Record{ID: leadID, Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue("Polymorphic lead"), "Company": storage.StringValue("Fixture company")}}
	org.Objects["Lead"] = lead

	historyDefinition := storage.ObjectDefinition{
		APIName: "Contract__History",
		Fields: map[string]storage.Field{
			"ParentId": {APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Contract__c"}, RelationshipName: "Parent"},
		},
		Relations: []storage.Relationship{{Field: "ParentId", ParentObjects: []string{"Contract__c"}, ParentRelationship: "Parent"}},
	}
	storage.EnsureStandardObjectFields(&historyDefinition)
	org.Objects["Contract__History"] = storage.ObjectState{
		Definition: historyDefinition,
		Records: map[storage.ID]storage.Record{
			"a02000000000001": {ID: "a02000000000001", Object: "Contract__History", Fields: map[string]storage.Value{"ParentId": storage.IDValue("a00000000000001")}},
		},
	}

	task := org.Objects["Task"]
	task.Records = map[storage.ID]storage.Record{
		"00T000000000001": {ID: "00T000000000001", Object: "Task", Fields: map[string]storage.Value{"Subject": storage.StringValue("Contact task"), "WhoId": storage.IDValue(contactID)}},
		"00T000000000002": {ID: "00T000000000002", Object: "Task", Fields: map[string]storage.Value{"Subject": storage.StringValue("Lead task"), "WhoId": storage.IDValue(leadID)}},
	}
	org.Objects["Task"] = task
	return org
}

func relationshipUserModeApexOrg(readWhoID, readTask, denyTaskAtProfile, privateTasks bool) (storage.OrgState, Value) {
	org := relationshipContractApexOrg()
	storage.EnsureStandardObject(&org, "Account")
	accountID := storage.ID("001000000000001")
	account := org.Objects["Account"]
	account.Records[accountID] = storage.Record{
		ID: accountID, Object: "Account",
		Fields: map[string]storage.Value{
			"Name": storage.StringValue("Type control account"),
			"Type": storage.StringValue("Customer"),
		},
	}
	org.Objects["Account"] = account

	contact := org.Objects["Contact"]
	contact.Definition.Fields["Secret__c"] = storage.Field{APIName: "Secret__c", Type: storage.FieldString}
	for id, record := range contact.Records {
		record.Fields["Secret__c"] = storage.StringValue("contact secret")
		contact.Records[id] = record
	}
	org.Objects["Contact"] = contact
	lead := org.Objects["Lead"]
	lead.Definition.Fields["Secret__c"] = storage.Field{APIName: "Secret__c", Type: storage.FieldString}
	for id, record := range lead.Records {
		record.Fields["Secret__c"] = storage.StringValue("lead secret")
		lead.Records[id] = record
	}
	org.Objects["Lead"] = lead

	task := org.Objects["Task"]
	for id, record := range task.Records {
		record.Fields["AccountId"] = storage.IDValue(accountID)
		if privateTasks {
			record.System.OwnerID = storage.ID("005000000000002")
		}
		task.Records[id] = record
	}
	if privateTasks {
		task.Definition.SharingModel = "Private"
	}
	org.Objects["Task"] = task

	permissionSetID := storage.ID("0PS000000000001")
	profileOwnedPermissionSetID := storage.ID("0PS000000000002")
	userID := storage.ID("005000000000001")
	objectPermissions := map[storage.ID]storage.Record{
		"0OP000000000002": {ID: "0OP000000000002", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(permissionSetID), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true)}},
		"0OP000000000003": {ID: "0OP000000000003", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(permissionSetID), "SObjectType": storage.StringValue("Contact"), "PermissionsRead": storage.BooleanValue(true)}},
		"0OP000000000004": {ID: "0OP000000000004", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(permissionSetID), "SObjectType": storage.StringValue("Lead"), "PermissionsRead": storage.BooleanValue(true)}},
	}
	if readTask {
		objectPermissions["0OP000000000001"] = storage.Record{ID: "0OP000000000001", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(permissionSetID), "SObjectType": storage.StringValue("Task"), "PermissionsRead": storage.BooleanValue(true)}}
	}
	profileID := storage.ID("00e000000000001")
	org.Objects["Profile"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		profileID: {ID: profileID, Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Minimum Access - Salesforce"), "UserType": storage.StringValue("Standard")}},
	}}
	if denyTaskAtProfile {
		org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
			profileOwnedPermissionSetID: {ID: profileOwnedPermissionSetID, Object: "PermissionSet", Fields: map[string]storage.Value{"IsOwnedByProfile": storage.BooleanValue(true), "ProfileId": storage.IDValue(profileID)}},
		}}
		objectPermissions["0OP000000000005"] = storage.Record{ID: "0OP000000000005", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(profileOwnedPermissionSetID), "SObjectType": storage.StringValue("Task"), "PermissionsRead": storage.BooleanValue(false)}}
	}
	org.Objects["ObjectPermissions"] = storage.ObjectState{Records: objectPermissions}
	org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"0Pa000000000001": {ID: "0Pa000000000001", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(userID), "PermissionSetId": storage.IDValue(permissionSetID)}},
	}}
	if readWhoID {
		org.Objects["FieldPermissions"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
			"0FP000000000001": {ID: "0FP000000000001", Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(permissionSetID), "SObjectType": storage.StringValue("Task"), "Field": storage.StringValue("Task.WhoId"), "PermissionsRead": storage.BooleanValue(true)}},
		}}
	}

	user := Object("User")
	user.Fields["Id"] = String(string(userID))
	user.Fields["ProfileId"] = String(string(profileID))
	return org, user
}
