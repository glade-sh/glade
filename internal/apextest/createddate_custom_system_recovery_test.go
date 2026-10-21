package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// API 67 expectations use the separately captured explicit SYSTEM_MODE context.
// They do not claim success for API 67's default USER_MODE operations.
func TestCreatedDateCustomExplicitSystemRecovery(t *testing.T) {
	// Native source67/transport67 ordinary and JSON A/B/D observations are
	// complete; the original source53/62 tests remain a separate baseline.
	const className = "CustomCreatedDateSystemAssignmentTest"
	const source = `@IsTest private class CustomCreatedDateSystemAssignmentTest {
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
  OwnedDateRow__c row = jsonRow('Owned custom update');
  Account parent = new Account(Name='Owned custom parent');
  insert as system parent;
  Datetime supplied = row.CreatedDate;
  CustomCreatedDateSystemAssignmentState.targetName = row.Name;
  insert as system row;
  System.assertNotEquals(null,row.Id);
  System.assertEquals(supplied,row.CreatedDate);
  OwnedDateRow__c first = [SELECT CreatedDate FROM OwnedDateRow__c WHERE Id=:row.Id WITH SYSTEM_MODE];
  System.assertNotEquals(null,first.CreatedDate);
  System.assertNotEquals(supplied,first.CreatedDate);
  row.Link__c = parent.Id;
  update as system row;
  OwnedDateRow__c stored = [SELECT CreatedDate,Link__c FROM OwnedDateRow__c WHERE Id=:row.Id WITH SYSTEM_MODE];
  System.assertEquals(supplied,row.CreatedDate);
  System.assertEquals(parent.Id,row.Link__c);
  System.assertEquals(parent.Id,stored.Link__c);
  System.assertEquals(first.CreatedDate,stored.CreatedDate);
  System.assertEquals(new List<String>{'bi','ai','bu','au'},CustomCreatedDateSystemAssignmentState.phases);
  System.assertEquals(new List<Datetime>{null,first.CreatedDate,first.CreatedDate,first.CreatedDate},CustomCreatedDateSystemAssignmentState.stamps);
  System.assertEquals(new List<Boolean>{false,true,true,true},CustomCreatedDateSystemAssignmentState.populated);
 }
 @IsTest static void jsonPicklistAssignmentInsert() {
  OwnedDateRow__c row = jsonRow('Owned custom insert');
  Datetime supplied = row.CreatedDate;
  CustomCreatedDateSystemAssignmentState.targetName = row.Name;
  row.Choice__c = 'North';
  insert as system row;
  OwnedDateRow__c stored = [SELECT CreatedDate,Choice__c FROM OwnedDateRow__c WHERE Id=:row.Id WITH SYSTEM_MODE];
  System.assertNotEquals(null,row.Id);
  System.assertEquals(supplied,row.CreatedDate);
  System.assertEquals('North',row.Choice__c);
  System.assertEquals('North',stored.Choice__c);
  System.assertNotEquals(null,stored.CreatedDate);
  System.assertNotEquals(supplied,stored.CreatedDate);
  System.assertEquals(new List<String>{'bi','ai'},CustomCreatedDateSystemAssignmentState.phases);
  System.assertEquals(new List<Datetime>{null,stored.CreatedDate},CustomCreatedDateSystemAssignmentState.stamps);
  System.assertEquals(new List<Boolean>{false,true},CustomCreatedDateSystemAssignmentState.populated);
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
  CustomCreatedDateSystemAssignmentState.targetName = row.Name;
  insert as system row;
  System.assertEquals(suppliedDate,row.CreatedDate);
  OwnedDateRow__c first = [SELECT CreatedDate FROM OwnedDateRow__c WHERE Id=:row.Id WITH SYSTEM_MODE];
  System.assertNotEquals(null,first.CreatedDate);
  System.assertNotEquals(suppliedDate,first.CreatedDate);
  OwnedDateRow__c target = new AliasBuilder(row).prepare();
  System.assertEquals(87.65,row.Amount__c);
  Savepoint boundary = Database.setSavepoint();
  List<Database.SaveResult> results = Database.update(new List<OwnedDateRow__c>{target},false,AccessLevel.SYSTEM_MODE);
  System.assertEquals(1,results.size());
  System.assertEquals(true,results[0].isSuccess());
  System.assertEquals(0,results[0].getErrors().size());
  OwnedDateRow__c stored = [SELECT Amount__c,CreatedDate FROM OwnedDateRow__c WHERE Id=:row.Id WITH SYSTEM_MODE];
  System.assertEquals(suppliedDate,row.CreatedDate);
  System.assertEquals(87.65,stored.Amount__c);
  System.assertEquals(first.CreatedDate,stored.CreatedDate);
  System.assertEquals(new List<String>{'bi','ai','bu','au'},CustomCreatedDateSystemAssignmentState.phases);
  System.assertEquals(new List<Datetime>{null,first.CreatedDate,first.CreatedDate,first.CreatedDate},CustomCreatedDateSystemAssignmentState.stamps);
  System.assertEquals(new List<Boolean>{false,true,true,true},CustomCreatedDateSystemAssignmentState.populated);
  Database.rollback(boundary);
 }
 @IsTest static void ordinaryAssignedPicklistInsert() {
  OwnedDateRow__c row = new OwnedDateRow__c(Name='Owned custom insert');
  CustomCreatedDateSystemAssignmentState.targetName = row.Name;
  row.Choice__c = 'North';
  insert as system row;
  OwnedDateRow__c stored = [SELECT CreatedDate,Choice__c FROM OwnedDateRow__c WHERE Id=:row.Id WITH SYSTEM_MODE];
  System.assertNotEquals(null,row.Id);
  System.assertEquals(null,row.CreatedDate);
  System.assertEquals('North',row.Choice__c);
  System.assertEquals('North',stored.Choice__c);
  System.assertNotEquals(null,stored.CreatedDate);
  System.assertEquals(new List<String>{'bi','ai'},CustomCreatedDateSystemAssignmentState.phases);
  System.assertEquals(new List<Datetime>{null,stored.CreatedDate},CustomCreatedDateSystemAssignmentState.stamps);
  System.assertEquals(new List<Boolean>{false,true},CustomCreatedDateSystemAssignmentState.populated);
 }
}`
	const observer = `public class CustomCreatedDateSystemAssignmentState {
 public static String targetName;
 public static List<String> phases = new List<String>();
 public static List<Datetime> stamps = new List<Datetime>();
 public static List<Boolean> populated = new List<Boolean>();
}`
	const trigger = `trigger CustomCreatedDateSystemAssignmentObserver on OwnedDateRow__c (before insert,after insert,before update,after update) {
 for (OwnedDateRow__c row : Trigger.new) {
  if (row.Name == CustomCreatedDateSystemAssignmentState.targetName) {
   CustomCreatedDateSystemAssignmentState.phases.add(Trigger.isInsert ? (Trigger.isBefore ? 'bi' : 'ai') : (Trigger.isBefore ? 'bu' : 'au'));
   CustomCreatedDateSystemAssignmentState.stamps.add(row.CreatedDate);
   CustomCreatedDateSystemAssignmentState.populated.add(row.getPopulatedFieldsAsMap().containsKey('CreatedDate'));
  }
 }
}`
	for _, context := range []struct {
		name, sourceAPI, transportAPI string
	}{
		{"systemexplicit_source67_transport67", "67.0", "67.0"},
	} {
		t.Run(context.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+context.sourceAPI+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "objects/OwnedDateRow__c/OwnedDateRow__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Owned Date Row</label><nameField><label>Owned Date Row Name</label><type>Text</type></nameField><pluralLabel>Owned Date Rows</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
			writeFile(t, filepath.Join(base, "objects/OwnedDateRow__c/fields/Link__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Link__c</fullName><deleteConstraint>SetNull</deleteConstraint><label>Link</label><referenceTo>Account</referenceTo><relationshipLabel>Owned Date Rows</relationshipLabel><relationshipName>OwnedDateRows</relationshipName><required>false</required><type>Lookup</type></CustomField>`)
			writeFile(t, filepath.Join(base, "objects/OwnedDateRow__c/fields/Choice__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Choice__c</fullName><label>Choice</label><required>false</required><type>Picklist</type><valueSet><restricted>false</restricted><valueSetDefinition><sorted>false</sorted><value><fullName>North</fullName><default>false</default><label>North</label></value><value><fullName>South</fullName><default>false</default><label>South</label></value></valueSetDefinition></valueSet></CustomField>`)
			writeFile(t, filepath.Join(base, "objects/OwnedDateRow__c/fields/Amount__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Amount__c</fullName><label>Amount</label><precision>18</precision><scale>2</scale><type>Currency</type></CustomField>`)
			for name, body := range map[string]string{className: source, "CustomCreatedDateSystemAssignmentState": observer} {
				path := filepath.Join(base, "classes", name+".cls")
				writeFile(t, path, body)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+context.sourceAPI+"</apiVersion></ApexClass>")
			}
			writeFile(t, filepath.Join(base, "triggers/CustomCreatedDateSystemAssignmentObserver.trigger"), trigger)
			writeFile(t, filepath.Join(base, "triggers/CustomCreatedDateSystemAssignmentObserver.trigger-meta.xml"), "<ApexTrigger><apiVersion>"+context.sourceAPI+"</apiVersion><status>Active</status></ApexTrigger>")
			run := Run(loadTestIndex(t, root), Options{
				RuntimeRESTAPIVersion: context.transportAPI,
				SelectedClasses:       []string{className},
				NoDiskCache:           true,
				Parallelism:           1,
			})
			if summary := run.Summary(); summary.Total != 4 || summary.Passed != 4 {
				data, _ := json.Marshal(run)
				t.Fatalf("CreatedDate JSON writable assignment controls: %s", data)
			}
		})
	}
}
