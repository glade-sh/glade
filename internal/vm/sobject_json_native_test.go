package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native rows: ../apextest/testdata/conformance/sobject_json/org.tsv,
// API 62/67 queryNameId, queryIdName, queryFields, queryReversed and queryThenWrite.
func TestSObjectJSONQueryOrderNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, testContext := range []bool{false, true} {
			for _, tc := range []struct{ name, source string }{
				{name: "queryNameId", source: `Account a=new Account(Name='Parent',Phone='123',Description=null); insert a;
Account q=[SELECT Name, Id FROM Account WHERE Id=:a.Id];
System.assertEquals('{"attributes":{"type":"Account","url":"/services/data/v68.0/sobjects/Account/<account>"},"Name":"Parent","Id":"<account>"}', JSON.serialize(q).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{"attributes":{"type":"Account","url":"/services/data/v68.0/sobjects/Account/<account>"},"Name":"Parent","Id":"<account>"}', JSON.serialize(q,true).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{\n  "attributes" : {\n    "type" : "Account",\n    "url" : "/services/data/v68.0/sobjects/Account/<account>"\n  },\n  "Name" : "Parent",\n  "Id" : "<account>"\n}', JSON.serializePretty(q).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{\n  "attributes" : {\n    "type" : "Account",\n    "url" : "/services/data/v68.0/sobjects/Account/<account>"\n  },\n  "Name" : "Parent",\n  "Id" : "<account>"\n}', JSON.serializePretty(q,true).replace(String.valueOf(a.Id),'<account>'));
`},
				{name: "queryIdName", source: `Account a=new Account(Name='Parent',Phone='123',Description=null); insert a;
Account q=[SELECT Id, Name FROM Account WHERE Id=:a.Id];
System.assertEquals('{"attributes":{"type":"Account","url":"/services/data/v68.0/sobjects/Account/<account>"},"Id":"<account>","Name":"Parent"}', JSON.serialize(q).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{"attributes":{"type":"Account","url":"/services/data/v68.0/sobjects/Account/<account>"},"Id":"<account>","Name":"Parent"}', JSON.serialize(q,true).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{\n  "attributes" : {\n    "type" : "Account",\n    "url" : "/services/data/v68.0/sobjects/Account/<account>"\n  },\n  "Id" : "<account>",\n  "Name" : "Parent"\n}', JSON.serializePretty(q).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{\n  "attributes" : {\n    "type" : "Account",\n    "url" : "/services/data/v68.0/sobjects/Account/<account>"\n  },\n  "Id" : "<account>",\n  "Name" : "Parent"\n}', JSON.serializePretty(q,true).replace(String.valueOf(a.Id),'<account>'));
`},
				{name: "queryFields", source: `Account a=new Account(Name='Parent',Phone='123',Description=null); insert a;
Account q=[SELECT Id, Name, Phone, Industry, Description FROM Account WHERE Id=:a.Id];
System.assertEquals('{"attributes":{"type":"Account","url":"/services/data/v68.0/sobjects/Account/<account>"},"Id":"<account>","Name":"Parent","Phone":"123"}', JSON.serialize(q).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{"attributes":{"type":"Account","url":"/services/data/v68.0/sobjects/Account/<account>"},"Id":"<account>","Name":"Parent","Phone":"123"}', JSON.serialize(q,true).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{\n  "attributes" : {\n    "type" : "Account",\n    "url" : "/services/data/v68.0/sobjects/Account/<account>"\n  },\n  "Id" : "<account>",\n  "Name" : "Parent",\n  "Phone" : "123"\n}', JSON.serializePretty(q).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{\n  "attributes" : {\n    "type" : "Account",\n    "url" : "/services/data/v68.0/sobjects/Account/<account>"\n  },\n  "Id" : "<account>",\n  "Name" : "Parent",\n  "Phone" : "123"\n}', JSON.serializePretty(q,true).replace(String.valueOf(a.Id),'<account>'));
`},
				{name: "queryReversed", source: `Account a=new Account(Name='Parent',Phone='123',Description=null); insert a;
Account q=[SELECT Description, Industry, Phone, Name, Id FROM Account WHERE Id=:a.Id];
System.assertEquals('{"attributes":{"type":"Account","url":"/services/data/v68.0/sobjects/Account/<account>"},"Phone":"123","Name":"Parent","Id":"<account>"}', JSON.serialize(q).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{"attributes":{"type":"Account","url":"/services/data/v68.0/sobjects/Account/<account>"},"Phone":"123","Name":"Parent","Id":"<account>"}', JSON.serialize(q,true).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{\n  "attributes" : {\n    "type" : "Account",\n    "url" : "/services/data/v68.0/sobjects/Account/<account>"\n  },\n  "Phone" : "123",\n  "Name" : "Parent",\n  "Id" : "<account>"\n}', JSON.serializePretty(q).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{\n  "attributes" : {\n    "type" : "Account",\n    "url" : "/services/data/v68.0/sobjects/Account/<account>"\n  },\n  "Phone" : "123",\n  "Name" : "Parent",\n  "Id" : "<account>"\n}', JSON.serializePretty(q,true).replace(String.valueOf(a.Id),'<account>'));
`},
				{name: "queryThenWrite", source: `Account a=new Account(Name='Parent',Phone='123',Description=null); insert a;
Account q=[SELECT Phone, Name, Description FROM Account WHERE Id=:a.Id];
q.Industry='Technology'; q.Name='Changed'; q.Description=null;
System.assertEquals('{"attributes":{"type":"Account","url":"/services/data/v68.0/sobjects/Account/<account>"},"Phone":"123","Name":"Changed","Id":"<account>","Industry":"Technology","Description":null}', JSON.serialize(q).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{"attributes":{"type":"Account","url":"/services/data/v68.0/sobjects/Account/<account>"},"Phone":"123","Name":"Changed","Id":"<account>","Industry":"Technology","Description":null}', JSON.serialize(q,true).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{\n  "attributes" : {\n    "type" : "Account",\n    "url" : "/services/data/v68.0/sobjects/Account/<account>"\n  },\n  "Phone" : "123",\n  "Name" : "Changed",\n  "Id" : "<account>",\n  "Industry" : "Technology",\n  "Description" : null\n}', JSON.serializePretty(q).replace(String.valueOf(a.Id),'<account>'));
System.assertEquals('{\n  "attributes" : {\n    "type" : "Account",\n    "url" : "/services/data/v68.0/sobjects/Account/<account>"\n  },\n  "Phone" : "123",\n  "Name" : "Changed",\n  "Id" : "<account>",\n  "Industry" : "Technology",\n  "Description" : null\n}', JSON.serializePretty(q,true).replace(String.valueOf(a.Id),'<account>'));
`},
			} {
				route := "anonymous"
				if testContext {
					route = "isTest"
				}
				t.Run(api+"/"+route+"/"+tc.name, func(t *testing.T) {
					program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := testDataOrg()
					storage.EnsureStandardObject(&org, "Account")
					org.APIVersion = "68.0"
					machine := New(nil)
					machine.SetOrg(&org)
					if testContext {
						machine.EnableTestContext()
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// Local isolation control: JSON-only subquery join values must participate in
// security stripping without changing the public populated-field map.
func TestSObjectJSONSubqueryProjectionStripInaccessible(t *testing.T) {
	const source = `
Account parent = new Account(Name='Parent');
insert parent;
Contact child = new Contact(LastName='Child', AccountId=parent.Id);
insert child;
Account row = [SELECT Name, (SELECT LastName FROM Contacts) FROM Account WHERE Id=:parent.Id];
String original = JSON.serialize(row);
System.assert(original.contains('"AccountId":'), 'JSON includes the implicit subquery join');
Set<String> populatedKeys = new Set<String>(row.Contacts[0].getPopulatedFieldsAsMap().keySet());
System.assertEquals(new Set<String>{'Id', 'LastName'}, populatedKeys);
SObjectAccessDecision decision = Security.stripInaccessible(AccessType.READABLE, new List<Account>{row});
Account stripped = (Account)decision.getRecords()[0];
String filtered = JSON.serialize(stripped);
System.assert(filtered.contains('"LastName":"Child"'), 'The readable child remains present');
System.assertEquals(original, JSON.serialize(row), 'Stripping must not mutate the source projection');
System.assertEquals(populatedKeys, row.Contacts[0].getPopulatedFieldsAsMap().keySet());
System.assertEquals(populatedKeys, stripped.Contacts[0].getPopulatedFieldsAsMap().keySet());
`
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name       string
			accessible bool
			assertions string
		}{
			{
				name:       "denied",
				accessible: false,
				assertions: `
System.assert(!filtered.contains('"AccountId":'), 'Denied join values must not leak through JSON');
System.assert(decision.getRemovedFields().get('Contact').contains('AccountId'));
`,
			},
			{
				name:       "readable",
				accessible: true,
				assertions: `
System.assertEquals(original, filtered, 'Readable join values remain serializable');
System.assert(!decision.getRemovedFields().containsKey('Contact'));
`,
			},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				org := testDataOrg()
				storage.EnsureStandardObject(&org, "Account")
				storage.EnsureStandardObject(&org, "Contact")
				org.APIVersion = "68.0"
				contact := org.Objects["Contact"]
				accountID := contact.Definition.Fields["AccountId"]
				accountID.Accessible = storage.BoolFlag(tc.accessible)
				contact.Definition.Fields["AccountId"] = accountID
				org.Objects["Contact"] = contact
				program, err := CompileAnonymousWithOptions(source+tc.assertions, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
