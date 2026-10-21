package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestStrippedFeatureRecordJSON(t *testing.T) {
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "AccountBrand")
	machine := New(nil)
	machine.SetOrg(&org)
	user := Object("User")
	user.Fields["Id"] = String("005000000000002")
	profile := Object("Profile")
	profile.Fields["Name"] = String("Standard User")
	user.Fields["Profile"] = profile
	machine.executionUser = user
	program, err := CompileAnonymous(`
AccountBrand source = (AccountBrand)AccountBrand.SObjectType.newSObject(null,true);
source.Id=(Id)(AccountBrand.SObjectType.getDescribe().getKeyPrefix()+'000000000001');
source.Name='Owned';
source.CompanyName='Owned';
SObject stripped = Security.stripInaccessible(AccessType.READABLE, new List<SObject>{source}, false).getRecords()[0];
System.assertEquals(JSON.serializePretty(new AccountBrand(Id=source.Id)), JSON.serializePretty(stripped));
AccountBrand transientRecord = (AccountBrand)AccountBrand.SObjectType.newSObject(null,true);
transientRecord.Name='Owned';
SObject blank = Security.stripInaccessible(AccessType.READABLE, new List<SObject>{transientRecord}, false).getRecords()[0];
System.assertEquals(JSON.serializePretty(new AccountBrand()), JSON.serializePretty(blank));
System.assertEquals(JSON.serializePretty(new List<SObject>{new AccountBrand(Id=source.Id),new AccountBrand()}),JSON.serializePretty(new List<SObject>{stripped,blank}));
System.assertEquals(JSON.serializePretty(new Map<String,SObject>{String.valueOf(source.Id)=>new AccountBrand(Id=source.Id),''=>new AccountBrand()}),JSON.serializePretty(new Map<String,SObject>{String.valueOf(source.Id)=>stripped,''=>blank}));
System.assertEquals('Owned',source.Name);
System.assertNotEquals(null,source.OwnerId);
System.assertEquals('Owned',transientRecord.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
