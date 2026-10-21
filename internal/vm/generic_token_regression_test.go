package vm

import "testing"

// Retain the established populated generic-list controls. Null-leading lists
// and Id-only singleton lists remain pending native rows; these tests do not
// assign Salesforce expectations to those candidate outcomes.
func TestGenericRecordListTokenControls(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> ordinary = new List<SObject>{new Account(Name='sample')};
System.assertEquals(Account.SObjectType,ordinary.getSObjectType());
System.assertEquals(Account.SObjectType,ordinary.clone().getSObjectType());
System.assertEquals(Account.SObjectType,ordinary.deepClone().getSObjectType());
List<Account> concrete = new List<Account>();
List<SObject> widened = concrete;
System.assertEquals(Account.SObjectType,widened.getSObjectType());
List<SObject> transported = (List<SObject>)JSON.deserialize('[{"attributes":{"type":"Account"},"Name":"sample"}]',List<SObject>.class);
System.assertEquals(Account.SObjectType,transported.getSObjectType());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestGenericAggregateResultListTokenControls(t *testing.T) {
	program, err := CompileAnonymous(`
AggregateResult row = (AggregateResult)JSON.deserialize(JSON.serialize(new Map<String,Object>()),AggregateResult.class);
System.assertEquals(Schema.AggregateResult.SObjectType,row.getSObjectType());
List<AggregateResult> concrete = new List<AggregateResult>{row};
System.assertEquals(Schema.AggregateResult.SObjectType,concrete.getSObjectType());
List<SObject> generic = new List<SObject>();
generic.add(row);
System.assertEquals(Schema.AggregateResult.SObjectType,generic.getSObjectType());
System.assertEquals(Schema.AggregateResult.SObjectType,generic.clone().getSObjectType());
List<SObject> literal = new List<SObject>{row};
System.assertEquals(Schema.AggregateResult.SObjectType,literal.getSObjectType());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
