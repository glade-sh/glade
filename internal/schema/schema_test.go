package schema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/project"
)

func TestLoadProject(t *testing.T) {
	root := t.TempDir()
	objectPath := filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml")
	fieldPath := filepath.Join(root, "force-app/main/objects/Thing__c/fields/Parent__c.field-meta.xml")
	globalPicklistFieldPath := filepath.Join(root, "force-app/main/objects/Thing__c/fields/State__c.field-meta.xml")
	formulaFieldPath := filepath.Join(root, "force-app/main/objects/Thing__c/fields/Score__c.field-meta.xml")
	encryptedFieldPath := filepath.Join(root, "force-app/main/objects/Thing__c/fields/Secret__c.field-meta.xml")
	rootFieldPath := filepath.Join(root, "force-app/main/objects/Thing__c/Legacy__c.field-meta.xml")
	valueSetPath := filepath.Join(root, "force-app/main/globalValueSets/States.globalValueSet-meta.xml")
	recordTypePath := filepath.Join(root, "force-app/main/objects/Thing__c/recordTypes/Business.recordType-meta.xml")
	lowercaseRecordTypePath := filepath.Join(root, "force-app/main/objects/Thing__c/recordTypes/Consumer.recordtype-meta.xml")
	validationRulePath := filepath.Join(root, "force-app/main/objects/Thing__c/validationRules/Block.validationRule-meta.xml")
	writeFile(t, objectPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label><pluralLabel>Things</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, fieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Parent__c</fullName><label>Parent</label><type>Picklist</type><referenceTo>Thing__c</referenceTo><referenceTo>Account</referenceTo><relationshipName>Parent__r</relationshipName><childRelationshipName>Children__r</childRelationshipName><deleteConstraint>Cascade</deleteConstraint><valueSet><valueSetDefinition><value><fullName>Hot</fullName><default>true</default><label>Hot Label</label></value><value><fullName>Cold</fullName><isActive>false</isActive></value></valueSetDefinition></valueSet></CustomField>`)
	writeFile(t, globalPicklistFieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>State__c</fullName><label>State</label><type>Picklist</type><valueSet><restricted>true</restricted><valueSetName>States</valueSetName></valueSet></CustomField>`)
	writeFile(t, valueSetPath, `<GlobalValueSet xmlns="http://soap.sforce.com/2006/04/metadata"><customValue><fullName>AL</fullName><default>false</default><label>Alabama</label></customValue><customValue><fullName>PA</fullName><isActive>false</isActive><label>Pennsylvania</label></customValue></GlobalValueSet>`)
	writeFile(t, formulaFieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Score__c</fullName><label>Score</label><type>Number</type><formula>1 + 1</formula></CustomField>`)
	writeFile(t, encryptedFieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Secret__c</fullName><label>Secret</label><type>EncryptedText</type></CustomField>`)
	writeFile(t, rootFieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Legacy__c</fullName><label>Legacy</label><type>Text</type></CustomField>`)
	writeFile(t, recordTypePath, `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Business</fullName><label>Business Thing</label><active>true</active><default>true</default><description>Business records</description><picklistValues><picklist>State__c</picklist><values><fullName>AL</fullName><default>true</default></values><values><fullName>PA</fullName><default>false</default></values></picklistValues></RecordType>`)
	writeFile(t, lowercaseRecordTypePath, `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><label>Consumer Thing</label><active>false</active></RecordType>`)
	writeFile(t, validationRulePath, `<ValidationRule xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Block</fullName><active>true</active><errorConditionFormula>Parent__c = "Blocked"</errorConditionFormula><errorMessage>blocked by rule</errorMessage><errorDisplayField>Parent__c</errorDisplayField></ValidationRule>`)

	s, err := LoadProject(project.Project{ObjectFiles: []string{objectPath}, FieldFiles: []string{fieldPath, globalPicklistFieldPath, formulaFieldPath, encryptedFieldPath, rootFieldPath}, GlobalValueSetFiles: []string{valueSetPath}, RecordTypeFiles: []string{recordTypePath, lowercaseRecordTypePath}, ValidationRuleFiles: []string{validationRulePath}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Objects) != 1 || s.Objects[0].Name != "Thing__c" {
		t.Fatalf("objects = %#v", s.Objects)
	}
	if len(s.Objects[0].Fields) != 5 {
		t.Fatalf("fields = %#v", s.Objects[0].Fields)
	}
	fields := fieldsByName(s.Objects[0].Fields)
	parent := fields["Parent__c"]
	if len(parent.ReferenceTo) != 2 || parent.ReferenceTo[0] != "Thing__c" || parent.ReferenceTo[1] != "Account" {
		t.Fatalf("parent field = %#v", parent)
	}
	if _, ok := fields["Legacy__c"]; !ok {
		t.Fatalf("missing root object field: %#v", s.Objects[0].Fields)
	}
	if got := fields["Score__c"].Formula; got != "1 + 1" {
		t.Fatalf("formula = %q", got)
	}
	if !fields["Secret__c"].Encrypted {
		t.Fatalf("encrypted field = %#v", fields["Secret__c"])
	}
	if got := parent.DeleteConstraint; got != "Cascade" {
		t.Fatalf("delete constraint = %q", got)
	}
	if got := parent.ChildRelationshipName; got != "Children__r" {
		t.Fatalf("child relationship name = %q", got)
	}
	values := parent.PicklistValues
	if len(values) != 2 || values[0].FullName != "Hot" || !values[0].Default || !values[0].Active || values[0].Label != "Hot Label" {
		t.Fatalf("picklist values = %#v", values)
	}
	if values[1].Active {
		t.Fatalf("inactive picklist value marked active: %#v", values[1])
	}
	state := fields["State__c"]
	if state.ValueSetName != "States" {
		t.Fatalf("value set name = %q", state.ValueSetName)
	}
	if len(state.PicklistValues) != 2 || state.PicklistValues[0].FullName != "AL" || state.PicklistValues[1].Active {
		t.Fatalf("global picklist values = %#v", state.PicklistValues)
	}
	recordTypes := s.Objects[0].RecordTypes
	if len(recordTypes) != 2 || recordTypes[0].DeveloperName != "Business" || recordTypes[0].Label != "Business Thing" || !recordTypes[0].Active || !recordTypes[0].Default {
		t.Fatalf("record types = %#v", recordTypes)
	}
	if got := recordTypes[0].PicklistDefaults["State__c"]; got != "AL" {
		t.Fatalf("record type picklist defaults = %#v", recordTypes[0].PicklistDefaults)
	}
	if recordTypes[1].DeveloperName != "Consumer" || recordTypes[1].Label != "Consumer Thing" || recordTypes[1].Active {
		t.Fatalf("record types = %#v", recordTypes)
	}
	rules := s.Objects[0].ValidationRules
	if len(rules) != 1 || rules[0].Name != "Block" || !rules[0].Active || rules[0].ErrorMessage != "blocked by rule" || rules[0].ErrorDisplayField != "Parent__c" {
		t.Fatalf("validation rules = %#v", rules)
	}
}

func TestLoadProjectInfersMissingReferencedCustomObjects(t *testing.T) {
	root := t.TempDir()
	objectPath := filepath.Join(root, "force-app/main/objects/Line__c/Line__c.object-meta.xml")
	fieldPath := filepath.Join(root, "force-app/main/objects/Line__c/fields/ManagedCart__c.field-meta.xml")
	writeFile(t, objectPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Line</label></CustomObject>`)
	writeFile(t, fieldPath, `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>ManagedCart__c</fullName><label>Managed Cart</label><type>Lookup</type><referenceTo>znu__CartItemLine__c</referenceTo><referenceTo>Account</referenceTo><relationshipName>ManagedCart__r</relationshipName></CustomField>`)

	s, err := LoadProject(project.Project{ObjectFiles: []string{objectPath}, FieldFiles: []string{fieldPath}})
	if err != nil {
		t.Fatal(err)
	}

	objects := objectsByName(s.Objects)
	if _, ok := objects["Line__c"]; !ok {
		t.Fatalf("missing local object: %#v", s.Objects)
	}
	if inferred, ok := objects["znu__CartItemLine__c"]; !ok || len(inferred.Fields) != 0 {
		t.Fatalf("missing inferred managed package object: %#v", s.Objects)
	}
	if _, ok := objects["Account"]; ok {
		t.Fatalf("standard reference should not be inferred into project schema: %#v", s.Objects)
	}
}

func TestLoadProjectNormalizesLegacyObjectAndCustomMetadata(t *testing.T) {
	root := t.TempDir()
	objectPath := filepath.Join(root, "src/objects/Feature__mdt.object")
	targetPath := filepath.Join(root, "src/objects/Target__mdt.object")
	defaultPath := filepath.Join(root, "src/customMetadata/Feature.Default.md")
	modernPath := filepath.Join(root, "src/customMetadata/Feature.Modern.md-meta.xml")
	writeFile(t, objectPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
<label>Feature</label>
<fields><fullName>Enabled__c</fullName><label>Enabled</label><type>Checkbox</type><defaultValue>false</defaultValue></fields>
<fields><fullName>Target__c</fullName><label>Target</label><type>MetadataRelationship</type><referenceTo>Target__mdt</referenceTo><relationshipName>Target__r</relationshipName></fields>
<recordTypes><fullName>Internal</fullName><active>true</active></recordTypes>
<validationRules><fullName>HasTarget</fullName><active>true</active><errorMessage>target required</errorMessage></validationRules>
</CustomObject>`)
	writeFile(t, targetPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Target</label></CustomObject>`)
	writeFile(t, defaultPath, `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata"><label>Default Label</label><protected>true</protected><values><field>Enabled__c</field><value>true</value></values><values><field>Target__c</field><value>Target</value></values></CustomMetadata>`)
	writeFile(t, modernPath, `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata"><values><field>Enabled__c</field><value>false</value></values></CustomMetadata>`)

	s, err := LoadProject(project.Project{ObjectFiles: []string{objectPath, targetPath}, CustomMetadataFiles: []string{modernPath, defaultPath}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Objects) != 2 || s.Objects[0].Name != "Feature__mdt" {
		t.Fatalf("objects = %#v", s.Objects)
	}
	feature := s.Objects[0]
	if len(feature.Fields) != 2 || feature.Fields[1].Name != "Target__c" || feature.Fields[1].Type != "MetadataRelationship" || feature.Fields[1].RelationshipName != "Target__r" {
		t.Fatalf("legacy fields = %#v", feature.Fields)
	}
	if len(feature.RecordTypes) != 1 || feature.RecordTypes[0].DeveloperName != "Internal" || len(feature.ValidationRules) != 1 || feature.ValidationRules[0].Name != "HasTarget" {
		t.Fatalf("legacy children = %#v %#v", feature.RecordTypes, feature.ValidationRules)
	}
	if len(s.CustomMetadataRecords) != 2 || s.CustomMetadataRecords[0].FullName != "Feature.Default" || s.CustomMetadataRecords[0].ObjectName != "Feature__mdt" || s.CustomMetadataRecords[0].DeveloperName != "Default" || !s.CustomMetadataRecords[0].Protected {
		t.Fatalf("custom metadata records = %#v", s.CustomMetadataRecords)
	}
}

func fieldsByName(fields []Field) map[string]Field {
	out := make(map[string]Field, len(fields))
	for _, field := range fields {
		out[field.Name] = field
	}
	return out
}

func objectsByName(objects []Object) map[string]Object {
	out := make(map[string]Object, len(objects))
	for _, object := range objects {
		out[object.Name] = object
	}
	return out
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
