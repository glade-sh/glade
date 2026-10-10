package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// the native compatibility contract records the observed constructor behavior for contracts 394/395.
// The Account fixture intentionally has no RecordTypeId field.
func TestObservedSchemaConstructorSevenCasesAPIs62To67(t *testing.T) {
	t.Log("contracts 394/395: one-argument Id classification and loadDefaults outcomes")
	const sourcePrologue = `
Id accountId = '001000000000001AAA';
Id masterId = '012000000000000AAA';
`
	cases := []struct {
		name   string
		source string
	}{
		{"case1_onearg_valid_account_id", `
SObject accountById = Account.SObjectType.newSObject(accountId);
System.assertEquals(accountId, accountById.get('Id'), 'case1 one-argument Account Id preserves Id');
`},
		{"case2_onearg_master_recordtype_id", `
Boolean invalidMasterIdCaught = false;
try {
    Account.SObjectType.newSObject(masterId);
    System.assert(false, 'case2 expected invalid Account Id exception');
} catch (SObjectException e) {
    invalidMasterIdCaught = true;
    System.assertEquals('System.SObjectException', e.getTypeName(), 'case2 exception type');
    System.assertEquals('Invalid Id for Account', e.getMessage(), 'case2 exception message');
}
System.assert(invalidMasterIdCaught, 'case2 master RecordType Id is invalid for one-argument overload');
`},
		{"case3_master_false_no_defaults", `
SObject masterWithoutDefaults = Account.SObjectType.newSObject(masterId, false);
System.assertNotEquals(null, masterWithoutDefaults, 'case3 master,false succeeds');
System.assertEquals(null, masterWithoutDefaults.get('Proof_Default__c'), 'case3 no Text default');
System.assertEquals(null, masterWithoutDefaults.get('OwnerId'), 'case3 no OwnerId default');
`},
		{"case4_master_true_unavailable", `
Boolean masterDefaultsCaught = false;
try {
    Account.SObjectType.newSObject(masterId, true);
    System.assert(false, 'case4 expected unavailable Record Type exception');
} catch (SObjectException e) {
    masterDefaultsCaught = true;
    System.assertEquals('System.SObjectException', e.getTypeName(), 'case4 exception type');
    System.assertEquals('Record Type is Unavailable', e.getMessage(), 'case4 exception message');
}
System.assert(masterDefaultsCaught, 'case4 master,true throws');
`},
		{"case5_account_id_true_unavailable", `
Boolean accountDefaultsCaught = false;
try {
    Account.SObjectType.newSObject(accountId, true);
    System.assert(false, 'case5 expected unavailable Record Type exception');
} catch (SObjectException e) {
    accountDefaultsCaught = true;
    System.assertEquals('System.SObjectException', e.getTypeName(), 'case5 exception type');
    System.assertEquals('Record Type is Unavailable', e.getMessage(), 'case5 exception message');
}
System.assert(accountDefaultsCaught, 'case5 Account Id,true throws');
`},
		{"case6_null_false_no_defaults", `
SObject nullWithoutDefaults = Account.SObjectType.newSObject(null, false);
System.assertNotEquals(null, nullWithoutDefaults, 'case6 null,false succeeds');
System.assertEquals(null, nullWithoutDefaults.get('Proof_Default__c'), 'case6 no Text default');
System.assertEquals(null, nullWithoutDefaults.get('OwnerId'), 'case6 no OwnerId default');
`},
		{"case7_null_true_text_and_owner_defaults", `
SObject nullWithDefaults = Account.SObjectType.newSObject(null, true);
System.assertEquals(null, nullWithDefaults.get('Id'), 'case7 Id remains null');
System.assertEquals('glade-schema-default-96', nullWithDefaults.get('Proof_Default__c'), 'case7 exact Text default');
System.assertEquals(UserInfo.getUserId(), nullWithDefaults.get('OwnerId'), 'case7 OwnerId uses current User');
`},
	}
	for api := 62; api <= 67; api++ {
		api := api
		for _, tc := range cases {
			tc := tc
			t.Run(fmt.Sprintf("API%d/%s", api, tc.name), func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(sourcePrologue+tc.source, CompileOptions{
					APIVersion: fmt.Sprintf("%d.0", api),
				})
				if err != nil {
					t.Fatal(err)
				}

				org := schemaConstructorObservedOrg()
				machine := New(nil)
				machine.SetOrg(&org)
				machine.SetCurrentUser(storage.Record{
					ID:     "005000000000001AAA",
					Object: "User",
					Fields: map[string]storage.Value{"Username": storage.StringValue("schema-oracle@example.test")},
				})
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func schemaConstructorObservedOrg() storage.OrgState {
	org := testDataOrg()
	account := org.Objects["Account"]
	delete(account.Definition.Fields, "RecordTypeId")
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{
		ID:            "012000000000000AAA",
		DeveloperName: "Master",
		Name:          "Master",
		Active:        true,
		Available:     true,
		Default:       true,
	}}
	account.Definition.Fields["OwnerId"] = storage.Field{
		APIName:          "OwnerId",
		Type:             storage.FieldReference,
		ReferenceTo:      []string{"User"},
		RelationshipName: "Owner",
	}
	account.Definition.Fields["Proof_Default__c"] = storage.Field{
		APIName:      "Proof_Default__c",
		Type:         storage.FieldString,
		Length:       64,
		DefaultValue: `"glade-schema-default-96"`,
	}
	org.Objects["Account"] = account

	userID := storage.ID("005000000000001AAA")
	org.Objects["User"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "User",
			KeyPrefix: "005",
			Fields: map[string]storage.Field{
				"Id":       {APIName: "Id", Type: storage.FieldID},
				"Username": {APIName: "Username", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			userID: {
				ID:     userID,
				Object: "User",
				Fields: map[string]storage.Value{
					"Username": storage.StringValue("schema-oracle@example.test"),
				},
			},
		},
	}
	return org
}

// These are local preservation controls, not additional Salesforce observations.
func TestSchemaConstructorPreservesCustomRecordTypeAndLookupDefaults(t *testing.T) {
	org := schemaConstructorObservedOrg()
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{
		ID: "012000000000002AAA", Active: true, Available: true, Default: true,
	}}
	account.Definition.Fields["RecordTypeId"] = storage.Field{
		APIName: "RecordTypeId", Type: storage.FieldReference, ReferenceTo: []string{"RecordType"},
	}
	account.Definition.Fields["Reviewer__c"] = storage.Field{
		APIName: "Reviewer__c", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Reviewer__r",
	}
	org.Objects["Account"] = account
	program, err := CompileAnonymous(`
Id recordTypeId = '012000000000002AAA';
SObject explicitDefaults = Account.SObjectType.newSObject(recordTypeId, true);
System.assertEquals(recordTypeId, explicitDefaults.get('RecordTypeId'));
System.assertEquals(null, explicitDefaults.get('Id'));
System.assertEquals('glade-schema-default-96', explicitDefaults.get('Proof_Default__c'));
System.assertEquals(UserInfo.getUserId(), explicitDefaults.get('OwnerId'));
System.assertEquals(null, explicitDefaults.get('Reviewer__c'));
SObject withoutDefaults = Account.SObjectType.newSObject(recordTypeId, false);
System.assertEquals(recordTypeId, withoutDefaults.get('RecordTypeId'));
System.assertEquals(null, withoutDefaults.get('Proof_Default__c'));
System.assertEquals(null, withoutDefaults.get('OwnerId'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(org.Objects["User"].Records["005000000000001AAA"])
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
