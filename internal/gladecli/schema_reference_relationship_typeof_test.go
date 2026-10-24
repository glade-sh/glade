package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// The native metadata rejection is schema_reference_gate/org.tsv:2,13.
// Pin diagnostic ownership even if TYPEOF validity independently rejects a query.
func TestSchemaReferenceGateRelationshipTypeofCLIProjectRoutes(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	for _, api := range []string{"62.0", "67.0"} {
		for _, object := range []string{"Task", "Event"} {
			t.Run(api+"/"+object, func(t *testing.T) {
				root := t.TempDir()
				for path, source := range map[string]string{
					"sfdx-project.json": `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"` + api + `"}`,
					"src/objects/Parcel__c/Parcel__c.object-meta.xml":            `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Parcel__c</fullName><label>Parcel</label><pluralLabel>Parcels</pluralLabel><enableActivities>true</enableActivities></CustomObject>`,
					"src/objects/Parcel__c/fields/Destination__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Destination__c</fullName><type>Lookup</type><referenceTo>UnavailableDestination</referenceTo><relationshipName>Destinations</relationshipName></CustomField>`,
					"src/classes/RelationshipUse.cls":                            `public class RelationshipUse { public static Boolean value() { return true; } public static void query() { Object items = [SELECT TYPEOF What WHEN Parcel__c THEN Destination__c END FROM ` + object + `]; } }`,
					"src/classes/RelationshipTest.cls":                           `@IsTest private class RelationshipTest { @IsTest static void observe() { System.assert(RelationshipUse.value()); } }`,
					"src/classes/IndependentTest.cls":                            `@IsTest private class IndependentTest { @IsTest static void observe() { System.assertEquals(7,7); } }`,
				} {
					writeTestFile(t, filepath.Join(root, path), source)
				}
				var out, errs bytes.Buffer
				code := Run(context.Background(), []string{"test", "--project", root, "--no-cache", "--no-serve", "--no-progress", "--json"}, &out, &errs)
				run, err := decodeTestRunJSON(out.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				owned := false
				for _, excluded := range run.ExcludedClasses {
					if excluded.ClassName != "RelationshipUse" {
						continue
					}
					owned = strings.Contains(excluded.Message, `field Parcel__c.Destination__c references unknown SObject`)
					for _, problem := range excluded.Problems {
						owned = owned || strings.Contains(problem.Message, `field Parcel__c.Destination__c references unknown SObject`)
					}
				}
				if code != 1 || !owned {
					t.Fatalf("TYPEOF metadata owner code=%d owned=%t exclusions=%+v stderr=%s", code, owned, run.ExcludedClasses, errs.String())
				}
			})
		}
	}
}
