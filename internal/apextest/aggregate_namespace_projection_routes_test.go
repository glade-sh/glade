package apextest

import (
	"path/filepath"
	"testing"
)

// Restores the pre-train behavior for the qualified SELECT / unqualified GROUP
// shape; these synthetic tests are regression evidence, not new native rows.
func TestAggregateNamespaceProjectionProjectRoutes(t *testing.T) {
	for _, declared := range []bool{true, false} {
		name := "declared"
		if !declared {
			name = "undeclared"
		}
		t.Run(name, func(t *testing.T) { aggregateNamespaceProjectionProjectRoutes(t, declared) })
	}
}

// With no namespace declared by the project, the same spelling must still
// keep the selected group value.
func aggregateNamespaceProjectionProjectRoutes(t *testing.T, declared bool) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			ns := ""
			if declared {
				ns = `"namespace":"pkg",`
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{`+ns+`"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			fields := filepath.Join(root, "force-app/main/default/objects/Account/fields")
			for _, name := range []string{"First__c", "Second__c"} {
				writeFile(t, filepath.Join(fields, name+".field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>`+name+`</fullName><label>Example</label><type>Text</type><length>80</length></CustomField>`)
			}
			classes := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(classes, "ProjectionService.cls"), `public class ProjectionService {
 public static void staticCheck() {
  List<AggregateResult> rows = [SELECT pkg__First__c, COUNT(Id) total FROM Account GROUP BY First__c];
  System.assertEquals('one', rows[0].get('pkg__First__c'));
  System.assertEquals(1, rows[0].get('total'));
 }
 public static void dynamicCheck() {
  List<AggregateResult> rows = Database.query('SELECT pkg__Second__c, COUNT(Id) total FROM Account GROUP BY Second__c');
  System.assertEquals('two', rows[0].get('pkg__Second__c'));
 }
}`)
			writeFile(t, filepath.Join(classes, "ProjectionRouteTest.cls"), `@IsTest private class ProjectionRouteTest {
 static void seed() { insert new Account(Name='Example', First__c='one', Second__c='two'); }
 @IsTest static void namedStatic() { seed(); ProjectionService.staticCheck(); }
 @IsTest static void namedDynamic() { seed(); ProjectionService.dynamicCheck(); }
 @IsTest static void testStatic() {
  seed(); List<AggregateResult> rows = [SELECT pkg__First__c, COUNT(Id) total FROM Account GROUP BY First__c];
  System.assertEquals('one', rows[0].get('pkg__First__c'));
 }
 @IsTest static void testDynamic() {
  seed(); List<AggregateResult> rows = Database.query('SELECT pkg__Second__c, COUNT(Id) total FROM Account GROUP BY Second__c');
  System.assertEquals('two', rows[0].get('pkg__Second__c'));
 }
}`)
			for _, class := range []string{"ProjectionService", "ProjectionRouteTest"} {
				writeFile(t, filepath.Join(classes, class+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			t.Setenv("GLADE_HOME", t.TempDir())
			for _, phase := range []string{"cold", "warm"} {
				t.Run(phase, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"ProjectionRouteTest"}, Parallelism: 1})
					if got := run.Summary(); got.Total != 4 || got.Passed != 4 {
						t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
					}
				})
			}
		})
	}
}
