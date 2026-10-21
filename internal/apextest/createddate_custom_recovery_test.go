package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Test-only proposal grounded in the six complete source53/65 and source62/67
// custom observations. Unrun; source67 expectations remain withheld.
func TestCreatedDateCustomJSONWritableAssignmentRecovery(t *testing.T) {
	// Source53/65 is diagnostic coverage; source62/67 is the supported floor.
	// The source67 outcomes require a separate cause diagnostic and are excluded.
	const className = "CustomCreatedDateJSONAssignmentTest"
	const source = `@IsTest private class CustomCreatedDateJSONAssignmentTest {
 private static OwnedDateRow__c jsonRow(String name) {
  Map<String,Object> values = new Map<String,Object>{
   'attributes'=>new Map<String,Object>{'type'=>'OwnedDateRow__c'},
   'Id'=>null,
   'Name'=>name,
   'CreatedDate'=>Datetime.newInstanceGmt(2001,2,3,4,5,6)
  };
  return (OwnedDateRow__c)JSON.deserialize(JSON.serialize(values),OwnedDateRow__c.class);
 }
 private class AliasBuilder {
  private OwnedDateRow__c value;
  AliasBuilder(OwnedDateRow__c input) { value = input; }
  OwnedDateRow__c prepare() {
   value.put('Amount__c',87.65);
   return value;
  }
 }
 @IsTest static void sameCallerInsertLookupUpdate() {
  Account parent = new Account(Name='Owned custom parent');
  insert parent;
  OwnedDateRow__c row = jsonRow('Owned assignment update');
  Datetime supplied = row.CreatedDate;
  CustomCreatedDateJSONAssignmentState.targetName = row.Name;
  insert row;
  System.assertNotEquals(null,row.Id);
  System.assertEquals(supplied,row.CreatedDate);
  OwnedDateRow__c first = [SELECT CreatedDate FROM OwnedDateRow__c WHERE Id=:row.Id];
  System.assertNotEquals(null,first.CreatedDate);
  System.assertNotEquals(supplied,first.CreatedDate);
  row.Link__c = parent.Id;
  update row;
  OwnedDateRow__c stored = [SELECT CreatedDate,Link__c FROM OwnedDateRow__c WHERE Id=:row.Id];
  System.assertEquals(supplied,row.CreatedDate);
  System.assertEquals(parent.Id,row.Link__c);
  System.assertEquals(parent.Id,stored.Link__c);
  System.assertEquals(first.CreatedDate,stored.CreatedDate);
  System.assertEquals(new List<String>{'bi','ai','bu','au'},CustomCreatedDateJSONAssignmentState.phases);
  System.assertEquals(new List<Datetime>{null,first.CreatedDate,first.CreatedDate,first.CreatedDate},CustomCreatedDateJSONAssignmentState.stamps);
  System.assertEquals(new List<Boolean>{false,true,true,true},CustomCreatedDateJSONAssignmentState.populated);
 }
 @IsTest static void jsonPicklistAssignmentInsert() {
  OwnedDateRow__c row = jsonRow('Owned assignment insert');
  Datetime supplied = row.CreatedDate;
  CustomCreatedDateJSONAssignmentState.targetName = row.Name;
  row.Choice__c = 'North';
  insert row;
  OwnedDateRow__c stored = [SELECT CreatedDate,Choice__c FROM OwnedDateRow__c WHERE Id=:row.Id];
  System.assertNotEquals(null,row.Id);
  System.assertEquals(supplied,row.CreatedDate);
  System.assertEquals('North',row.Choice__c);
  System.assertEquals('North',stored.Choice__c);
  System.assertNotEquals(null,stored.CreatedDate);
  System.assertNotEquals(supplied,stored.CreatedDate);
  System.assertEquals(new List<String>{'bi','ai'},CustomCreatedDateJSONAssignmentState.phases);
  System.assertEquals(new List<Datetime>{null,stored.CreatedDate},CustomCreatedDateJSONAssignmentState.stamps);
  System.assertEquals(new List<Boolean>{false,true},CustomCreatedDateJSONAssignmentState.populated);
 }
 @IsTest static void retainedCallerCurrencyListUpdate() {
  Map<String,Object> values = new Map<String,Object>{'Name'=>'Owned retained custom','Amount__c'=>21.34};
  OwnedDateRow__c initial = new OwnedDateRow__c();
  for (String field : values.keySet()) initial.put(field,values.get(field));
  Map<String,Object> supplied = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(initial));
  supplied.put('attributes',new Map<String,Object>{'type'=>'OwnedDateRow__c'});
  supplied.put('Id',null);
  supplied.put('CreatedDate','2007-02-03T04:05:06.000Z');
  OwnedDateRow__c row = (OwnedDateRow__c)JSON.deserialize(JSON.serialize(supplied),OwnedDateRow__c.class);
  Datetime suppliedDate = row.CreatedDate;
  CustomCreatedDateJSONAssignmentState.targetName = row.Name;
  insert row;
  System.assertEquals(suppliedDate,row.CreatedDate);
  OwnedDateRow__c first = [SELECT CreatedDate FROM OwnedDateRow__c WHERE Id=:row.Id];
  System.assertNotEquals(null,first.CreatedDate);
  System.assertNotEquals(suppliedDate,first.CreatedDate);
  OwnedDateRow__c target = new AliasBuilder(row).prepare();
  System.assertEquals(87.65,row.Amount__c);
  Savepoint boundary = Database.setSavepoint();
  List<Database.SaveResult> results = Database.update(new List<OwnedDateRow__c>{target},false);
  System.assertEquals(1,results.size());
  System.assertEquals(true,results[0].isSuccess());
  System.assertEquals(0,results[0].getErrors().size());
  OwnedDateRow__c stored = [SELECT Amount__c,CreatedDate FROM OwnedDateRow__c WHERE Id=:row.Id];
  System.assertEquals(suppliedDate,row.CreatedDate);
  System.assertEquals(87.65,stored.Amount__c);
  System.assertEquals(first.CreatedDate,stored.CreatedDate);
  System.assertEquals(new List<String>{'bi','ai','bu','au'},CustomCreatedDateJSONAssignmentState.phases);
  System.assertEquals(new List<Datetime>{null,first.CreatedDate,first.CreatedDate,first.CreatedDate},CustomCreatedDateJSONAssignmentState.stamps);
  System.assertEquals(new List<Boolean>{false,true,true,true},CustomCreatedDateJSONAssignmentState.populated);
  Database.rollback(boundary);
 }
}`
	const observer = `public class CustomCreatedDateJSONAssignmentState {
 public static String targetName;
 public static List<String> phases = new List<String>();
 public static List<Datetime> stamps = new List<Datetime>();
 public static List<Boolean> populated = new List<Boolean>();
}`
	const trigger = `trigger CustomCreatedDateJSONAssignmentObserver on OwnedDateRow__c (before insert,after insert,before update,after update) {
 for (OwnedDateRow__c row : Trigger.new) {
  if (row.Name == CustomCreatedDateJSONAssignmentState.targetName) {
   CustomCreatedDateJSONAssignmentState.phases.add(Trigger.isInsert ? (Trigger.isBefore ? 'bi' : 'ai') : (Trigger.isBefore ? 'bu' : 'au'));
   CustomCreatedDateJSONAssignmentState.stamps.add(row.CreatedDate);
   CustomCreatedDateJSONAssignmentState.populated.add(row.getPopulatedFieldsAsMap().containsKey('CreatedDate'));
  }
 }
}`
	for _, context := range []struct {
		name, sourceAPI, transportAPI string
	}{
		{"diagnostic_source53_transport65", "53.0", "65.0"},
		{"floor_source62_transport67", "62.0", "67.0"},
	} {
		t.Run(context.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+context.sourceAPI+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "objects/OwnedDateRow__c/OwnedDateRow__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Owned Date Row</label><nameField><label>Owned Date Row Name</label><type>Text</type></nameField><pluralLabel>Owned Date Rows</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
			writeFile(t, filepath.Join(base, "objects/OwnedDateRow__c/fields/Link__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Link__c</fullName><deleteConstraint>SetNull</deleteConstraint><label>Link</label><referenceTo>Account</referenceTo><relationshipLabel>Owned Date Rows</relationshipLabel><relationshipName>OwnedDateRows</relationshipName><required>false</required><type>Lookup</type></CustomField>`)
			writeFile(t, filepath.Join(base, "objects/OwnedDateRow__c/fields/Choice__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Choice__c</fullName><label>Choice</label><required>false</required><type>Picklist</type><valueSet><restricted>false</restricted><valueSetDefinition><sorted>false</sorted><value><fullName>North</fullName><default>false</default><label>North</label></value><value><fullName>South</fullName><default>false</default><label>South</label></value></valueSetDefinition></valueSet></CustomField>`)
			writeFile(t, filepath.Join(base, "objects/OwnedDateRow__c/fields/Amount__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Amount__c</fullName><label>Amount</label><precision>18</precision><scale>2</scale><type>Currency</type></CustomField>`)
			for name, body := range map[string]string{className: source, "CustomCreatedDateJSONAssignmentState": observer} {
				path := filepath.Join(base, "classes", name+".cls")
				writeFile(t, path, body)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+context.sourceAPI+"</apiVersion></ApexClass>")
			}
			writeFile(t, filepath.Join(base, "triggers/CustomCreatedDateJSONAssignmentObserver.trigger"), trigger)
			writeFile(t, filepath.Join(base, "triggers/CustomCreatedDateJSONAssignmentObserver.trigger-meta.xml"), "<ApexTrigger><apiVersion>"+context.sourceAPI+"</apiVersion><status>Active</status></ApexTrigger>")
			run := Run(loadTestIndex(t, root), Options{
				RuntimeRESTAPIVersion: context.transportAPI,
				SelectedClasses:       []string{className},
				NoDiskCache:           true,
				Parallelism:           1,
			})
			if summary := run.Summary(); summary.Total != 3 || summary.Passed != 3 {
				data, _ := json.Marshal(run)
				t.Fatalf("CreatedDate JSON writable assignment controls: %s", data)
			}
		})
	}
}
