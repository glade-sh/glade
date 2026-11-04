package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase349DecimalExternalKeyComparison(t *testing.T) {
	for _, tc := range []struct {
		a, b  string
		equal bool
	}{
		{"1.25", "2.50", false}, {"1.25", "1.25", true}, {"3.75", "4.00", false},
	} {
		if got := storageValuesEqualForVM(storage.Field{}, storage.DecimalValue(tc.a), storage.DecimalValue(tc.b)); got != tc.equal {
			t.Errorf("decimal comparison %q vs %q = %v, want %v", tc.a, tc.b, got, tc.equal)
		}
	}
}

func TestCase349PersonEmailWhitespaceSignal(t *testing.T) {
	for _, value := range []string{"", "   ", " \t "} {
		record := storage.Record{Fields: map[string]storage.Value{"PersonEmail": storage.StringValue(value)}}
		if accountRecordHasPersonAccountSignal(record) {
			t.Errorf("PersonEmail %q unexpectedly signals a person account", value)
		}
	}
	for _, value := range []string{"person@example.test", "false"} {
		record := storage.Record{Fields: map[string]storage.Value{"PersonEmail": storage.StringValue(value)}}
		if !accountRecordHasPersonAccountSignal(record) {
			t.Errorf("nonempty PersonEmail %q did not signal a person account", value)
		}
	}
}

func TestCase349DecimalUpsertAnonymous(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
EqualityProbe__c a = new EqualityProbe__c(Key__c=1.25); insert a;
EqualityProbe__c b = new EqualityProbe__c(Key__c=2.50);
Database.UpsertResult r=Database.upsert(b, EqualityProbe__c.Key__c, false);
System.assertEquals(true,r.isSuccess()); System.assertEquals(true,r.isCreated()); System.assertNotEquals(a.Id,b.Id);
EqualityProbe__c c = new EqualityProbe__c(Key__c=1.25);
Database.UpsertResult s=Database.upsert(c, EqualityProbe__c.Key__c, false);
System.assertEquals(true,s.isSuccess()); System.assertEquals(false,s.isCreated()); System.assertEquals(a.Id,c.Id);
EqualityProbe__c d = new EqualityProbe__c(Key__c=3.75); insert d;
EqualityProbe__c e = new EqualityProbe__c(Key__c=4.00);
Database.UpsertResult q=Database.upsert(e, EqualityProbe__c.Key__c, false);
System.assertEquals(true,q.isSuccess()); System.assertEquals(true,q.isCreated());
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			org.APIVersion = api
			org.Objects["EqualityProbe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "EqualityProbe__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
				"Key__c": {APIName: "Key__c", Type: storage.FieldDecimal, ExternalID: true},
			}}, Records: map[storage.ID]storage.Record{}}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCase349PersonEmailWhitespaceAnonymous(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Account a = new Account(Name='Business probe', PersonEmail='   ');
Database.SaveResult r = Database.insert(a, false);
System.assertEquals(true, r.isSuccess());
System.assertEquals(false, [SELECT IsPersonAccount FROM Account WHERE Id=:a.Id].IsPersonAccount);
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			org.APIVersion = api
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
