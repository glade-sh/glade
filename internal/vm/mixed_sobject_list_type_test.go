package vm

import "testing"

// Native MT01-MT09 twins are pending. Only a heterogeneous generic list's
// first-element fallback changes; concrete runtime views remain authoritative.
func TestMixedGenericSObjectListDoesNotChooseFirstType(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> records=new List<SObject>{new Account(),new Contact(),new Opportunity(),new Account()};
System.assertEquals(null,records.getSObjectType());
records=new List<SObject>{new Contact(),new Account()};
System.assertEquals(null,records.getSObjectType());
records=new List<SObject>{null,new Account(),new Contact()};
System.assertEquals(null,records.getSObjectType());
Map<Schema.SObjectType,List<SObject>> buckets=new Map<Schema.SObjectType,List<SObject>>();
for(SObject row:new List<SObject>{new Account(),new Contact(),new Opportunity(),new Account()}){
 Schema.SObjectType token=row.getSObjectType();
 if(!buckets.containsKey(token))buckets.put(token,new List<SObject>());
 buckets.get(token).add(row);
}
System.assertEquals(3,buckets.size());
System.assertEquals(2,buckets.get(Account.SObjectType).size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestMixedSObjectTypeKeepsConcreteListViews(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> concrete=new List<Account>();
Object saved=concrete;
List<SObject> view=(List<SObject>)saved;
System.assertEquals(Account.SObjectType,view.getSObjectType());
concrete.add(new Account());
System.assertEquals(Account.SObjectType,view.getSObjectType());
System.assertEquals(null,new List<SObject>().getSObjectType());

`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
