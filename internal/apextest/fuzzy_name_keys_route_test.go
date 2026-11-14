package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The custom FirstName and LastName match keys reach Apex DML on the
// anonymous and @IsTest routes. Each pair stores A, then inserts B with
// Database.insert(row, false) under a Block rule.
// NATIVE internal/dml/testdata/conformance/fuzzy_name_keys/org.tsv din and
// dsuf (Alert rules; find= is the verdict asserted here), anonymous and
// istest, 62.0 and 67.0.
func TestFuzzyNameKeysProjectRoutes(t *testing.T) {
	t.Cleanup(InvalidateRuntimeCaches)
	type pair struct {
		stored, candidate string
		duplicate         bool
	}
	cases := []struct {
		method string
		pairs  []pair
	}{
		// din pairs 6, 0 and 3: Initials and the first-word key.
		{"FirstName", []pair{{"P W G", "P", true}, {"Y", "Y R S", true}, {"YR", "Y R", false}}},
		// dsuf pairs 2, 0 and 5: suffix words, the last-word key.
		{"LastName", []pair{{"Harlow", "Harlow III", true}, {"Harlow", "Harlow Jr", false}, {"McAllen", "Mc Allen", false}}},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, c := range cases {
			t.Run(api+"/"+c.method, func(t *testing.T) {
				root := copyLookupFilterValidationProject(t, "testdata/conformance/dml_policies/project", api)
				base := filepath.Join(root, "force-app/main/default")
				writeFile(t, filepath.Join(base, "matchingRules/Contact.matchingRule-meta.xml"), `<MatchingRules xmlns="http://soap.sforce.com/2006/04/metadata"><matchingRules><fullName>NameKey</fullName><label>Name Key</label><matchingRuleItems><blankValueBehavior>NullNotAllowed</blankValueBehavior><fieldName>`+c.method+`</fieldName><matchingMethod>`+c.method+`</matchingMethod></matchingRuleItems><ruleStatus>Active</ruleStatus></matchingRules></MatchingRules>`)
				writeFile(t, filepath.Join(base, "duplicateRules/Contact.Synthetic_Block.duplicateRule-meta.xml"), `<DuplicateRule xmlns="http://soap.sforce.com/2006/04/metadata"><actionOnInsert>Block</actionOnInsert><actionOnUpdate>Block</actionOnUpdate><duplicateRuleMatchRules><matchRuleSObjectType>Contact</matchRuleSObjectType><matchingRule>NameKey</matchingRule></duplicateRuleMatchRules><isActive>true</isActive><masterLabel>Name Key</masterLabel><securityOption>BypassSharingRules</securityOption><sortOrder>4</sortOrder></DuplicateRule>`)
				if err := os.Remove(filepath.Join(base, "duplicateRules/Contact.Synthetic_Alert.duplicateRule-meta.xml")); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				var body strings.Builder
				for i, p := range c.pairs {
					// The other name field is distinct per record.
					fields := func(value, other string) string {
						if c.method == "FirstName" {
							return "FirstName='" + value + "', LastName='" + other + "'"
						}
						return "FirstName='" + other + "', LastName='" + value + "'"
					}
					want := "true"
					if p.duplicate {
						want = "false"
					}
					n := string(rune('a' + i))
					body.WriteString("Savepoint sp" + n + "=Database.setSavepoint(); insert new Contact(" + fields(p.stored, "Qvelt") + "); ")
					body.WriteString("Database.SaveResult r" + n + "=Database.insert(new Contact(" + fields(p.candidate, "Wenlock") + "), false); ")
					body.WriteString("System.assertEquals(" + want + ", r" + n + ".isSuccess(), '" + p.stored + "=>" + p.candidate + "'); Database.rollback(sp" + n + "); ")
				}
				class := filepath.Join(base, "classes/NameKeyRouteTest.cls")
				writeFile(t, class, "@IsTest private class NameKeyRouteTest { @IsTest static void runCase() { "+body.String()+" } }")
				writeFile(t, class+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				index := loadTestIndex(t, root)
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("compile: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				t.Run("anonymous", func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions(body.String(), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					if _, err = runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
				t.Run("istest", func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"NameKeyRouteTest"}, Parallelism: 1})
					if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
						t.Fatalf("summary %+v; suites: %+v", summary, run.Suites)
					}
				})
			})
		}
	}
}
