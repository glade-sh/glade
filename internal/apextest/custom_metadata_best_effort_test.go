package apextest

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/vm"
	"testing"
)

// A record file that cannot load (here a templated type name with no object)
// must not change how unrelated custom metadata relationships resolve. A
// 15- or 18-character developer name is also a well-formed Id, so a
// relationship value must resolve by record name once its parent exists.
func TestProjectAnonymousAndIsTestCustomMetadataSkipsUnloadableRecordWithoutChangingRelationships(t *testing.T) {
	const apexTest = `@IsTest
private class ParentRelationshipTest {
 @IsTest static void childResolvesParentByName() {
  List<ChildRow__mdt> rows = [SELECT DeveloperName, Parent__c, Parent__r.DeveloperName, Parent__r.Label FROM ChildRow__mdt ORDER BY DeveloperName];
  System.assertEquals(3, rows.size());
  System.assertEquals('LongNamedParentRec', rows[0].Parent__r.DeveloperName, 'eighteen character parent');
  System.assertEquals('ShortParent', rows[1].Parent__r.DeveloperName, 'short parent');
  System.assertEquals('FifteenCharsPar', rows[2].Parent__r.DeveloperName, 'fifteen character parent');
  System.assertEquals(1, [SELECT COUNT() FROM ChildRow__mdt WHERE Parent__r.Label = 'Long parent']);
 }
}
`
	files := map[string]string{
		"sfdx-project.json": `{"sourceApiVersion": "62.0", "packageDirectories": [{"path": "force-app", "default": true}, {"path": "extra", "default": false}]}`,
		"force-app/main/default/classes/ParentRelationshipTest.cls":                    apexTest,
		"force-app/main/default/classes/ParentRelationshipTest.cls-meta.xml":           "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>\n",
		"force-app/main/default/objects/ParentRow__mdt/ParentRow__mdt.object-meta.xml": `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Parent Row</label><pluralLabel>Parent Rows</pluralLabel><visibility>Public</visibility></CustomObject>` + "\n",
		"force-app/main/default/objects/ChildRow__mdt/ChildRow__mdt.object-meta.xml":   `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Child Row</label><pluralLabel>Child Rows</pluralLabel><visibility>Public</visibility></CustomObject>` + "\n",
		"force-app/main/default/objects/ChildRow__mdt/fields/Parent__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Parent__c</fullName><label>Parent</label><type>MetadataRelationship</type><referenceTo>ParentRow__mdt</referenceTo><relationshipLabel>Children</relationshipLabel><relationshipName>Children</relationshipName></CustomField>` + "\n",
		// Child records sort before their parents so a one-record-at-a-time load
		// sees the relationship value before the parent exists.
		"force-app/main/default/customMetadata/ChildRow.A.md-meta.xml":                   bestEffortMetadataRecord("A", "Parent__c", "LongNamedParentRec"),
		"force-app/main/default/customMetadata/ChildRow.B.md-meta.xml":                   bestEffortMetadataRecord("B", "Parent__c", "ShortParent"),
		"force-app/main/default/customMetadata/ChildRow.C.md-meta.xml":                   bestEffortMetadataRecord("C", "Parent__c", "FifteenCharsPar"),
		"force-app/main/default/customMetadata/ParentRow.LongNamedParentRec.md-meta.xml": bestEffortMetadataRecord("Long parent", "", ""),
		"force-app/main/default/customMetadata/ParentRow.ShortParent.md-meta.xml":        bestEffortMetadataRecord("Short parent", "", ""),
		"force-app/main/default/customMetadata/ParentRow.FifteenCharsPar.md-meta.xml":    bestEffortMetadataRecord("Fifteen parent", "", ""),
		// Unloadable: templated type with no object and a templated field.
		"extra/customMetadata/___NAMESPACE___Stray.___NAMESPACE___Row.md-meta.xml": `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>Stray</label><protected>false</protected><values><field>%%%NAMESPACE%%%Flag__c</field><value xsi:type="xsd:boolean">true</value></values></CustomMetadata>` + "\n",
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			for path, content := range files {
				writeFile(t, filepath.Join(root, path), strings.ReplaceAll(content, "62.0", api))
			}
			index := loadTestIndex(t, root)
			t.Run("istest", func(t *testing.T) {
				run := Run(index, Options{NoDiskCache: true})
				data, _ := json.Marshal(run)
				if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 {
					t.Fatalf("unloadable custom metadata record changed unrelated relationships: %s", data)
				}
			})
			t.Run("anonymous", func(t *testing.T) {
				_, body, found := strings.Cut(apexTest, "static void childResolvesParentByName() {")
				if !found {
					t.Fatal("missing anonymous assertion source")
				}
				body = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body), "}"))
				body = strings.TrimSpace(strings.TrimSuffix(body, "}"))
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err := runner.newMachine().Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}

func bestEffortMetadataRecord(label, field, value string) string {
	body := `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>` + label + `</label><protected>false</protected>`
	if field != "" {
		body += `<values><field>` + field + `</field><value xsi:type="xsd:string">` + value + `</value></values>`
	}
	return body + `</CustomMetadata>` + "\n"
}
