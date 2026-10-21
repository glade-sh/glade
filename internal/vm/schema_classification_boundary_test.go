package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// schema_classification_boundaries captures named source/transport API 62/65/67.
func TestSchemaIDCollectionsKeepGenericListType(t *testing.T) {
	machine := schemaClassificationVM(t)
	machine.EnableTestContext()
	convert, err := CompileAnonymous(`
List<SObject> records=new List<SObject>();
for(Id key:keys)records.add(key.getSObjectType().newSObject(key));
return records;
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "TypeProjection.convert", ClassName: "TypeProjection", IsStatic: true, ReturnType: "List<SObject>", Params: []Param{{Name: "keys", Type: "Iterable<Id>"}}, Program: convert}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Id current=UserInfo.getUserId();
for(Integer size:new List<Integer>{1,2,5}) {
 List<Id> keys=new List<Id>();
 for(Integer i=0;i<size;i++)keys.add(current);
 List<SObject> records=TypeProjection.convert(keys);
 System.assertEquals(size,records.size());
 System.assertEquals(null,records.getSObjectType(),'generic schema-created list');
 System.assertEquals(User.SObjectType,records[0].getSObjectType(),'element type remains concrete');
}
System.assertEquals(null,TypeProjection.convert(new Set<Id>{current}).getSObjectType());
List<User> concrete=new List<User>{(User)User.SObjectType.newSObject(current)};
List<SObject> widened=concrete;
System.assertEquals(User.SObjectType,widened.getSObjectType(),'typed list control');
List<SObject> populated=new List<SObject>{new Account(Name='Owned control')};
System.assertEquals(Account.SObjectType,populated.getSObjectType(),'existing populated-record control');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaTemplateIDImplicitConversionBoundary(t *testing.T) {
	machine := schemaClassificationVM(t)
	program, err := CompileAnonymous(`
String prefix=CaseComment.SObjectType.getDescribe().getKeyPrefix();
Integer width=CaseComment.SObjectType.getDescribe().fields.getMap().get('Id').getDescribe().getLength();
String shortText=prefix+'0'.repeat(11)+'1';
Id shortId=shortText;
for(String text:new List<String>{shortText,String.valueOf(shortId),prefix+'0'.repeat(width-prefix.length()-1)+'1'}) {
 Boolean caught=false;
 try {Id key=text;Schema.SObjectType token=key.getSObjectType();}
 catch(System.SObjectException failure) {
  caught=true;
  System.assert(failure.getMessage().startsWith('Cannot locate Apex Type for ID: '));
 }
 System.assert(caught,'unstored template Id must not acquire a token');
}
CaseComment instance=new CaseComment();
System.assertEquals(CaseComment.SObjectType,instance.getSObjectType(),'instance token control');
Id actor=UserInfo.getUserId();
System.assertEquals(User.SObjectType,actor.getSObjectType(),'standard Id control');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func schemaClassificationVM(t *testing.T) *VM {
	t.Helper()
	org := storage.NewOrgState()
	for _, name := range []string{"User", "Account", "CaseComment"} {
		storage.EnsureStandardObject(&org, name)
	}
	generator := storage.NewRuntimeIDGeneratorForOrg(&org)
	id, err := generator.Next("User")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: id, Object: "User"})
	return machine
}
