package apextest

import (
	"path/filepath"
	"testing"
)

// The capture deploys unqualified source fields. Native F001 returns qualified
// custom field and relationship paths; standard fields retain their names.
func writeNamespacedFieldSetProject(t *testing.T, root, api string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"nsx","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Thing__c/Thing__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label><pluralLabel>Things</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Thing__c/fields/Size__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Size__c</fullName><label>Size</label><type>Number</type><precision>5</precision><scale>0</scale></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Thing__c/fields/Owner__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Owner__c</fullName><label>Owner</label><type>Lookup</type><referenceTo>Contact</referenceTo><relationshipName>Owner</relationshipName><relationshipLabel>Things</relationshipLabel><deleteConstraint>SetNull</deleteConstraint></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Thing__c/fieldSets/Shown.fieldSet-meta.xml"), `<FieldSet xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Shown</fullName><description>d</description><displayedFields><field>Size__c</field><isFieldManaged>false</isFieldManaged><isRequired>false</isRequired></displayedFields><displayedFields><field>Owner__r.Name</field><isFieldManaged>false</isFieldManaged><isRequired>false</isRequired></displayedFields><displayedFields><field>Name</field><isFieldManaged>false</isFieldManaged><isRequired>false</isRequired></displayedFields><label>Shown</label></FieldSet>`)
}

func TestNamespacedFieldSetMemberPathMatchesFieldDescribeName(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for route, entry := range map[string]string{
			"isTest/directToken": "Schema.SObjectType.Thing__c.FieldSets.Shown",
			"isTest/tokenMap":    "Schema.SObjectType.Thing__c.FieldSets.getMap().get('nsx__Shown')",
			"isTest/describeMap": "Thing__c.SObjectType.getDescribe().fieldSets.getMap().get('nsx__Shown')",
		} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				writeNamespacedFieldSetProject(t, root, api)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/FieldSetPathTest.cls"), `@IsTest private class FieldSetPathTest {
 @IsTest static void memberPathIsTheDescribeName() {
  List<String> nativeValues = new List<String>();
  for (Schema.FieldSetMember member : `+entry+`.getFields()) {
   nativeValues.add(member.getFieldPath() + '|' + member.getSObjectField().getDescribe().getName() + '|' + member.getLabel());
  }
  System.assertEquals(`+conformanceApexString(namespacedNativeObservation(t, api, "isTest", "F001"))+`, String.join(nativeValues, ';'), 'native F001');
  Map<String, String> labelByName = new Map<String, String>();
  for (Schema.FieldSetMember member : `+entry+`.getFields()) {
   String describeName = member.getSObjectField().getDescribe().getName();
   if (!member.getFieldPath().contains('.')) { System.assertEquals(describeName, member.getFieldPath(), 'member path'); }
   labelByName.put(describeName, member.getLabel());
  }
  Set<String> paths = new Set<String>();
  for (Schema.FieldSetMember member : `+entry+`.getFields()) { paths.add(member.getFieldPath()); }
  System.assertEquals(new Set<String>{ 'nsx__Size__c', 'nsx__Owner__r.Name', 'Name' }, paths, 'relationship path');
  System.assertEquals('Size', labelByName.get('nsx__Size__c'));
  System.assertEquals(true, labelByName.containsKey('Name'));
 }
}`)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/FieldSetPathTest.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
				if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
					t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
				}
			})
		}
	}
}
