package vm

import "testing"

func TestGenericAllocationRelayPreservesSourceList(t *testing.T) {
	machine := New(nil)
	body, err := CompileAnonymous(`return rows.getSObjectType();`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "AllocationRelay.classify", ClassName: "AllocationRelay", IsStatic: true, ReturnType: "Schema.SObjectType", Params: []Param{{Name: "rows", Type: "List<SObject>"}}, Program: body}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<SObject> rows=new List<SObject>();
User missing;
rows.add(missing);
rows.add(new User(Id='005000000000001'));
Object token=AllocationRelay.classify(rows);
System.assertEquals(2,rows.size());
System.assertEquals(null,rows[0]);
System.assertEquals(Id.valueOf('005000000000001'),rows[1].Id);
List<User> concrete=new List<User>{new User()};
System.assertEquals(User.SObjectType,AllocationRelay.classify(concrete));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if rows := machine.Globals["rows"]; !allocatedGenericSObjectList(rows) || rows.List[1].classInstance {
		t.Fatal("type lookup changed source allocation provenance")
	}
}

func TestGenericAllocationRelayRetainsUnsavedRecordTypeTokens(t *testing.T) {
	machine := New(nil)
	body, err := CompileAnonymous(`return rows.getSObjectType();`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "AllocationRelay.classify", ClassName: "AllocationRelay", IsStatic: true, ReturnType: "Schema.SObjectType", Params: []Param{{Name: "rows", Type: "List<SObject>"}}, Program: body}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<SObject> records=new List<SObject>{new Account(Name='sample')};
System.assertEquals(Account.SObjectType,records.getSObjectType());
System.assertEquals(Account.SObjectType,AllocationRelay.classify(records));
System.assertEquals(Account.SObjectType,(new List<SObject>{new Account()}).getSObjectType());
List<SObject> identified=new List<SObject>{new Account(Id='001000000000001',Name='sample')};
System.assertEquals(Account.SObjectType,identified.getSObjectType());
System.assertEquals(Account.SObjectType,AllocationRelay.classify(identified));
AggregateResult aggregate=(AggregateResult)JSON.deserialize('{}',AggregateResult.class);
List<SObject> aggregates=new List<SObject>{aggregate};
System.assertEquals(Schema.AggregateResult.SObjectType,aggregate.getSObjectType());
System.assertEquals(Schema.AggregateResult.SObjectType,aggregates.getSObjectType());
System.assertEquals(Schema.AggregateResult.SObjectType,AllocationRelay.classify(aggregates));
System.assertEquals(Schema.AggregateResult.SObjectType,(new List<SObject>{aggregate}).getSObjectType());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestGenericAllocationCandidateExcludesClassInstances(t *testing.T) {
	for _, project := range []bool{false, true} {
		record := Object("Account")
		record.Fields["Id"] = String("001000000000001")
		record.classInstance, record.projectClass = !project, project
		rows := List(record)
		rows.Type, rows.Runtime, rows.genericSObjectList = "List<SObject>", "List<SObject>", true
		if allocatedGenericSObjectList(rows) {
			t.Fatal("class instance acquired schema-record allocation behavior")
		}
	}
}
