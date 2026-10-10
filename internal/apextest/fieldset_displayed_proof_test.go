package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// SF219 exact API62/project40 displayed-only FieldSet contract.
func TestFieldSetDisplayedAPI62Project40Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "40.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeFieldSetDisplayed62Successor.cls"), `@IsTest private class GladeFieldSetDisplayed62Successor {
 @IsTest static void getFieldsReturnsDisplayedOrderOnly() {
  List<Schema.FieldSetMember> fields=Contact.SObjectType.getDescribe().fieldSets.getMap().get('GladeDisplayed62Next').getFields();
  System.assertEquals(2,fields.size());
  System.assertEquals('LastName',fields[0].getFieldPath());
  System.assertEquals('Email',fields[1].getFieldPath());
  for(Schema.FieldSetMember field:fields) System.assertNotEquals('CreatedDate',field.getFieldPath());
 }
 @IsTest static void availableOnlyFieldSetHasNoDisplayedMembers() {
  List<Schema.FieldSetMember> fields=Contact.SObjectType.getDescribe().fieldSets.getMap().get('GladeAvailableOnly62Next').getFields();
  System.assertEquals(0,fields.size());
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeFieldSetDisplayed62Successor.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Contact/fieldSets/GladeDisplayed62Next.fieldSet-meta.xml"), `<FieldSet xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>GladeDisplayed62Next</fullName><availableFields><field>CreatedDate</field><isFieldManaged>false</isFieldManaged><isRequired>false</isRequired></availableFields><description>Fields available for configuration are distinct from the fields displayed to users.</description><displayedFields><field>LastName</field><isFieldManaged>false</isFieldManaged><isRequired>false</isRequired></displayedFields><displayedFields><field>Email</field><isFieldManaged>false</isFieldManaged><isRequired>false</isRequired></displayedFields><label>GladeDisplayed62Next</label></FieldSet>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Contact/fieldSets/GladeAvailableOnly62Next.fieldSet-meta.xml"), `<FieldSet xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>GladeAvailableOnly62Next</fullName><availableFields><field>CreatedDate</field><isFieldManaged>false</isFieldManaged><isRequired>false</isRequired></availableFields><description>Fields available for configuration are distinct from the fields displayed to users.</description><label>GladeAvailableOnly62Next</label></FieldSet>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Errors != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("SF219: %s", data)
	}
}
