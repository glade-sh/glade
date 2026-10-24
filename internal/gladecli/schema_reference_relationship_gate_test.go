package gladecli

import (
	"bytes"
	"context"
	"github.com/glade-sh/glade/internal/testreport"
	"path/filepath"
	"testing"
)

// Keep the parent-query regression end to end: metadata rejection must not
// become two passing tests. Native baseline: schema_reference_gate/org.tsv,
// missing_lookup at API 62/67; dependency attribution is local gate policy.
func TestSchemaReferenceGateRelationshipCLIProjectRoutes(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			for path, source := range map[string]string{
				"sfdx-project.json": `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"` + api + `"}`,
				"src/objects/Account/fields/Destination__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Destination__c</fullName><type>Lookup</type><referenceTo>UnavailableDestination</referenceTo><relationshipName>Destinations</relationshipName></CustomField>`,
				"src/classes/Independent.cls":                              `public class Independent { public static Integer value() { return 7; } }`,
				"src/classes/RelationshipUse.cls":                          `public class RelationshipUse { public static Boolean value() { return true; } public static void query() { Object items = [SELECT Account.Destination__c FROM Contact]; } }`,
				"src/classes/RelationshipTest.cls":                         `@IsTest private class RelationshipTest { @IsTest static void observe() { System.assert(RelationshipUse.value()); } }`,
				"src/classes/IndependentTest.cls":                          `@IsTest private class IndependentTest { @IsTest static void observe() { System.assertEquals(7,7); } }`,
			} {
				writeTestFile(t, filepath.Join(root, path), source)
			}
			t.Run("runner_istest", func(t *testing.T) {
				var out, errs bytes.Buffer
				code := Run(context.Background(), []string{"test", "--project", root, "--no-cache", "--no-serve", "--no-progress", "--json"}, &out, &errs)
				run, err := decodeTestRunJSON(out.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				got := map[string]testreport.Status{}
				for _, s := range run.Suites {
					for _, c := range s.Cases {
						got[c.ClassName] = c.Status
					}
				}
				if code != 1 || len(got) != 2 || got["IndependentTest"] != testreport.StatusPass || got["RelationshipTest"] != testreport.StatusCompileError {
					t.Fatalf("relationship runner code=%d statuses=%v stderr=%s", code, got, errs.String())
				}
			})
			for _, route := range []struct {
				name, source string
				code         int
			}{
				{"anonymous_independent", "System.assertEquals(7,7);", 0},
				{"named_independent", "System.assertEquals(7,Independent.value());", 0},
			} {
				t.Run(route.name, func(t *testing.T) {
					var out, errs bytes.Buffer
					code := Run(context.Background(), []string{"exec", "--project", root, "--json", route.source}, &out, &errs)
					if code != route.code {
						t.Fatalf("exec code=%d want=%d out=%s stderr=%s", code, route.code, out.String(), errs.String())
					}
				})
			}
		})
	}
}
