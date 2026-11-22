package apextest

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestDuplicateIndexedProjectRunnerBulk(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyDuplicateRuleProject(t, api)
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "matchingRules/Contact.matchingRule-meta.xml"), `<MatchingRules xmlns="http://soap.sforce.com/2006/04/metadata"><matchingRules><fullName>BulkName</fullName><label>Bulk Name</label><matchingRuleItems><blankValueBehavior>NullNotAllowed</blankValueBehavior><fieldName>LastName</fieldName><matchingMethod>LastName</matchingMethod></matchingRuleItems><ruleStatus>Active</ruleStatus></matchingRules></MatchingRules>`)
			writeFile(t, filepath.Join(base, "duplicateRules/Contact.BulkName.duplicateRule-meta.xml"), `<DuplicateRule xmlns="http://soap.sforce.com/2006/04/metadata"><actionOnInsert>Block</actionOnInsert><duplicateRuleMatchRules><matchRuleSObjectType>Contact</matchRuleSObjectType><matchingRule>BulkName</matchingRule></duplicateRuleMatchRules><isActive>true</isActive><masterLabel>Bulk Name</masterLabel><securityOption>BypassSharingRules</securityOption><sortOrder>1</sortOrder></DuplicateRule>`)
			class := filepath.Join(base, "classes/DuplicateBulkTest.cls")
			writeFile(t, class, `@IsTest private class DuplicateBulkTest {
			  @IsTest static void observe() {
			    List<Contact> seed = new List<Contact>();
			    for (Integer i=0; i<300; i++) seed.add(new Contact(LastName=String.valueOf(i)));
			    insert seed;
			    List<Contact> rows = new List<Contact>{new Contact(LastName='44'),new Contact(LastName='45'),new Contact(LastName='99999')};
			    List<Datacloud.FindDuplicatesResult> found = Datacloud.FindDuplicates.findDuplicates(rows);
			    List<Database.SaveResult> saved = Database.insert(rows, false);
			    System.assert(false, 'bulk:' + JSON.serialize(new List<Object>{found,saved}));
			  }
			}`)
			writeFile(t, class+"-meta.xml", fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"DuplicateBulkTest"}, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("runner report: %+v", run)
			}
			result := run.Suites[0].Cases[0]
			if result.Status != "fail" || result.Problem == nil || !strings.Contains(result.Problem.Message, "bulk:") {
				t.Fatalf("runner report: %+v", result)
			}
			if result.Problem.Type != "System.AssertException" || !strings.Contains(result.Problem.Message, "DUPLICATES_DETECTED") || !strings.Contains(result.Problem.Message, `"matchConfidence":100.0`) {
				t.Fatalf("runner report: %+v", result.Problem)
			}
			// The complete assertion payload, including reports and save errors,
			// is compared with the same runner workload on origin/main.
			if got := fmt.Sprintf("%x", sha256.Sum256([]byte(result.Problem.Message))); got != "b75db72660ee823331eec5a47cb3ef2feee5c5ff489dcda9b5ca189f42d9e124" {
				t.Fatalf("parent report differs (%s): %+v", got, result.Problem)
			}
		})
	}
}
