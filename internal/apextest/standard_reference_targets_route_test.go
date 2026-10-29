package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// Assessment.AssessmentStagedDataId and DocumentGenerationProcess.DocGenerationBatchProcessId
// name objects with no describe at API 62.0 or 67.0 in native scratch orgs
// (testdata/standard_reference_targets_native). A project that customizes the owners
// must compile and run; queries and describes of the targets stay unsupported
// like the native "sObject type ... is not supported" answer.
func TestReferenceTargetsOfIndustriesObjectsCompileAndStayUnqueryable(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, features := range []bool{false, true} {
			name := api + "/no-features"
			if features {
				name = api + "/features"
			}
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				if features {
					writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [EnableSetPasswordInApi]\n")
				}
				for _, object := range []string{"Assessment", "DocumentGenerationProcess"} {
					writeFile(t, filepath.Join(root, "force-app/main/default/objects", object, "fields/Note__c.field-meta.xml"),
						`<?xml version="1.0" encoding="UTF-8"?><CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Note__c</fullName><label>Note</label><type>Text</type><length>20</length></CustomField>`)
				}
				cls := filepath.Join(root, "force-app/main/default/classes/ReferenceTargetProbeTest.cls")
				writeFile(t, cls, `@IsTest
private class ReferenceTargetProbeTest {
  @IsTest static void ownersConstruct() {
    Assessment a = new Assessment(Name = 'x', Note__c = 'n');
    DocumentGenerationProcess p = new DocumentGenerationProcess(Note__c = 'n');
    System.assertEquals('n', a.Note__c);
    System.assertEquals('n', p.Note__c);
    System.assert(Schema.SObjectType.Assessment.fields.getMap().containsKey('note__c'));
  }
  @IsTest static void targetsAreNotQueryable() {
    for (String target : new String[]{'AssessmentStagedData', 'DocGenerationBatchProcess', 'ActivationTarget', 'DataAction', 'BusinessUnit'}) {
      Boolean queryFailed = false;
      String seen = 'no exception';
      try {
        Database.query('SELECT Id FROM ' + target + ' LIMIT 1');
      } catch (Exception e) {
        seen = e.getTypeName() + ': ' + e.getMessage();
        queryFailed = e instanceof QueryException && e.getMessage().contains('is not supported');
      }
      System.assert(queryFailed, target + ' ' + seen);
      System.assertEquals(false, Schema.getGlobalDescribe().containsKey(target.toLowerCase()), target);
      Boolean describeFailed = false;
      try {
        Schema.describeSObjects(new String[]{target});
      } catch (System.InvalidParameterValueException e) {
        describeFailed = true;
      }
      System.assert(describeFailed, target);
    }
  }
}
`)
				writeFile(t, cls+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				index := loadTestIndex(t, root)
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					var messages []string
					for _, diag := range analysis.Diagnostics {
						messages = append(messages, diag.Message)
					}
					t.Fatalf("project compilation: %v %s", index.Diagnostics, strings.Join(messages, "; "))
				}
				run := Run(index, Options{SelectedClasses: []string{"ReferenceTargetProbeTest"}, NoDiskCache: true, Parallelism: 1})
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						if c.Problem != nil {
							t.Fatalf("%s: %s", c.Name, c.Problem.Message)
						}
					}
				}
				if got := run.Summary(); got.Passed != 2 {
					t.Fatalf("summary = %+v, want 2 passed", got)
				}
			})
		}
	}
}
