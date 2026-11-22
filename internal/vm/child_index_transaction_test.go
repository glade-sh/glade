package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestChildIndexQueriesAfterRollbackAndUndelete(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = new Account(Name='One');
Account b = new Account(Name='Two');
insert new List<Account>{a,b};
Contact c = new Contact(LastName='Child', AccountId=a.Id);
insert c;
System.assertEquals(1, [SELECT Id,(SELECT Id FROM Contacts) FROM Account WHERE Id=:a.Id].Contacts.size());
Savepoint sp = Database.setSavepoint();
c.AccountId = b.Id;
update c;
System.assertEquals(0, [SELECT Id,(SELECT Id FROM Contacts) FROM Account WHERE Id=:a.Id].Contacts.size());
System.assertEquals(1, [SELECT Id,(SELECT Id FROM Contacts) FROM Account WHERE Id=:b.Id].Contacts.size());
Database.rollback(sp);
System.assertEquals(1, [SELECT Id,(SELECT Id FROM Contacts) FROM Account WHERE Id=:a.Id].Contacts.size());
System.assertEquals(0, [SELECT Id,(SELECT Id FROM Contacts) FROM Account WHERE Id=:b.Id].Contacts.size());
c = [SELECT Id,AccountId,LastName FROM Contact WHERE Id=:c.Id];
delete c;
System.assertEquals(0, [SELECT Id,(SELECT Id FROM Contacts) FROM Account WHERE Id=:a.Id].Contacts.size());
undelete c;
System.assertEquals(1, [SELECT Id,(SELECT Id FROM Contacts) FROM Account WHERE Id=:a.Id].Contacts.size());`)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	var contact storage.ObjectState
	contact.Definition = storage.ObjectDefinition{
		APIName: "Contact", KeyPrefix: "003",
		Fields: map[string]storage.Field{
			"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
			"LastName":  {APIName: "LastName", Type: storage.FieldString},
			"Name":      {APIName: "Name", Type: storage.FieldString},
		},
		Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account", ChildRelationship: "Contacts"}},
	}
	contact.SetRecords(make(map[storage.ID]storage.Record))
	org.Objects["Contact"] = contact
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
