package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Local preservation controls for matching shapes not captured by the native
// single exact-email upsert rows.
func TestUpsertDuplicateUncapturedScopeProjectRoutes(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	const fixture = "testdata/conformance/dml_policies/project"
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"other_field", "multi_field"} {
			t.Run(api+"/"+shape, func(t *testing.T) {
				root := copyLookupFilterValidationProject(t, fixture, api)
				base := filepath.Join(root, "force-app/main/default")
				writeFile(t, filepath.Join(base, "objects/Contact/fields/RouteKey__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>RouteKey__c</fullName><externalId>true</externalId><label>Route Key</label><length>100</length><type>Text</type></CustomField>`)
				items := `<matchingRuleItems><blankValueBehavior>NullNotAllowed</blankValueBehavior><fieldName>Phone</fieldName><matchingMethod>Exact</matchingMethod></matchingRuleItems>`
				if shape == "multi_field" {
					items += `<matchingRuleItems><blankValueBehavior>NullNotAllowed</blankValueBehavior><fieldName>Email</fieldName><matchingMethod>Exact</matchingMethod></matchingRuleItems>`
				}
				writeFile(t, filepath.Join(base, "matchingRules/Contact.matchingRule-meta.xml"), `<MatchingRules xmlns="http://soap.sforce.com/2006/04/metadata"><matchingRules><fullName>ScopeExact</fullName><label>Scope Exact</label>`+items+`<ruleStatus>Active</ruleStatus></matchingRules></MatchingRules>`)
				writeFile(t, filepath.Join(base, "duplicateRules/Contact.Synthetic_Block.duplicateRule-meta.xml"), `<DuplicateRule xmlns="http://soap.sforce.com/2006/04/metadata"><actionOnInsert>Block</actionOnInsert><actionOnUpdate>Block</actionOnUpdate><duplicateRuleMatchRules><matchRuleSObjectType>Contact</matchRuleSObjectType><matchingRule>ScopeExact</matchingRule></duplicateRuleMatchRules><isActive>true</isActive><masterLabel>Scope Block</masterLabel><securityOption>BypassSharingRules</securityOption><sortOrder>4</sortOrder></DuplicateRule>`)
				if err := os.Remove(filepath.Join(base, "duplicateRules/Contact.Synthetic_Alert.duplicateRule-meta.xml")); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				body := `insert new Contact(LastName='Seed', Email='scope@example.invalid', Phone='5551234567', RouteKey__c='seed'); Contact row=new Contact(LastName='Scope', Email='scope@example.invalid', Phone='5551234567', RouteKey__c='new'); Database.UpsertResult saved=Database.upsert(row, Contact.RouteKey__c, false); System.assertEquals(true,saved.isSuccess());`
				class := filepath.Join(base, "classes/ScopePreservationTest.cls")
				writeFile(t, class, "@IsTest private class ScopePreservationTest { @IsTest static void runCase() { "+body+" } }")
				writeFile(t, class+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				index := loadTestIndex(t, root)
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("compile: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				t.Run("anonymous", func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					if _, err = runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
				for _, route := range []string{"cold", "disk_restore", "memory_warm"} {
					t.Run(route, func(t *testing.T) {
						if route != "memory_warm" {
							InvalidateRuntimeCaches()
						}
						counters := newRunPerfCounters(true)
						if _, _, err := runtimeFromIndexWithSourceDigestsAndPerf(index, nil, newSourceCache(), true, counters); err != nil {
							t.Fatal(err)
						}
						phases := snapshotPerfCounters(counters).Phases
						if (phases.DiskCacheHits == 1) != (route == "disk_restore") || (phases.MemoryCacheHits == 1) != (route == "memory_warm") {
							t.Fatalf("cache route %s: %+v", route, phases)
						}
						run := Run(index, Options{SelectedClasses: []string{"ScopePreservationTest"}, Parallelism: 1})
						if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
							t.Fatalf("local preservation: %+v; suites: %+v", summary, run.Suites)
						}
					})
				}
			})
		}
	}
}

// Blank prior values are uncaptured local shipping-preservation controls.
func TestUpsertDuplicateUncapturedBlankPriorProjectRoutes(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		for _, prior := range []string{"missing", "null"} {
			t.Run(api+"/"+prior, func(t *testing.T) {
				root := copyLookupFilterValidationProject(t, "testdata/conformance/dml_policies/project", api)
				base := filepath.Join(root, "force-app/main/default")
				writeFile(t, filepath.Join(base, "objects/Contact/fields/RouteKey__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>RouteKey__c</fullName><externalId>true</externalId><label>Route Key</label><length>100</length><type>Text</type></CustomField>`)
				priorEmail := ""
				if prior == "null" {
					priorEmail = ", Email=null"
				}
				body := `insert new Contact(LastName='Block', Email='scope@example.invalid', RouteKey__c='seed'); Contact target=new Contact(LastName='Safe', RouteKey__c='target'` + priorEmail + `); insert target; Contact row=new Contact(LastName='Block', Email='scope@example.invalid', RouteKey__c='target'); Database.UpsertResult saved=Database.upsert(row, Contact.RouteKey__c, false); System.assertEquals(true,saved.isSuccess());`
				class := filepath.Join(base, "classes/BlankPriorPreservationTest.cls")
				writeFile(t, class, "@IsTest private class BlankPriorPreservationTest { @IsTest static void runCase() { "+body+" } }")
				writeFile(t, class+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				index := loadTestIndex(t, root)
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("compile: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				t.Run("anonymous", func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					if _, err = runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
				for _, route := range []string{"cold", "disk_restore", "memory_warm"} {
					t.Run(route, func(t *testing.T) {
						if route != "memory_warm" {
							InvalidateRuntimeCaches()
						}
						counters := newRunPerfCounters(true)
						if _, _, err := runtimeFromIndexWithSourceDigestsAndPerf(index, nil, newSourceCache(), true, counters); err != nil {
							t.Fatal(err)
						}
						phases := snapshotPerfCounters(counters).Phases
						if (phases.DiskCacheHits == 1) != (route == "disk_restore") || (phases.MemoryCacheHits == 1) != (route == "memory_warm") {
							t.Fatalf("cache route %s: %+v", route, phases)
						}
						run := Run(index, Options{SelectedClasses: []string{"BlankPriorPreservationTest"}, Parallelism: 1})
						if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
							t.Fatalf("blank prior: %+v; suites: %+v", summary, run.Suites)
						}
					})
				}
			})
		}
	}
}
