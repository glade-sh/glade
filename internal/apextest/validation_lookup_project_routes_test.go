package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The synthetic project twins reuse captured arithmetic and lookup outcomes.
// They exercise anonymous execution and the named test runner at each source API.
func TestValidationAndLookupProjectAnonymousAndIsTestNativeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, family := range []string{"validation_rule_formula_semantics", "lookup_filter_validation"} {
			t.Run(api+"/"+family, func(t *testing.T) {
				fixture := filepath.Join("testdata", "conformance", family)
				body := validationLookupProjectTwinBody(t, fixture, api, family)
				root := copyLookupFilterValidationProject(t, filepath.Join(fixture, "project"), api)
				classPath := filepath.Join(root, "force-app/main/default/classes/ValidationLookupRouteTest.cls")
				writeFile(t, classPath, "@IsTest private class ValidationLookupRouteTest { @IsTest static void nativeOutcomes() {\n"+body+"\n} }")
				writeFile(t, classPath+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				index := loadTestIndex(t, root)
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				if analysis := sema.Analyze(index); analysis.HasErrors() {
					t.Fatal(analysis.Diagnostics)
				}
				t.Run("anonymous", func(t *testing.T) {
					if analysis := sema.AnalyzeAnonymous(index, body, api); analysis.HasErrors() {
						t.Fatal(analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					if result, err := runner.execute(program); err != nil {
						t.Fatalf("anonymous native outcomes: %v; result=%#v", err, result)
					}
				})
				t.Run("istest_runner", func(t *testing.T) {
					// API 62 source is supported; the test runner's available REST
					// endpoint contexts use 65 as the nearest supported floor.
					restAPI := api
					if api == "62.0" {
						restAPI = "65.0"
					}
					run := Run(index, Options{SelectedClasses: []string{"ValidationLookupRouteTest"}, SelectedMethod: "nativeOutcomes", RuntimeRESTAPIVersion: restAPI, NoDiskCache: true, Parallelism: 1})
					if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
						t.Fatalf("named project native outcomes: summary=%#v suites=%#v", got, run.Suites)
					}
				})
			})
		}
	}
}

func validationLookupProjectTwinBody(t *testing.T, fixture, api, family string) string {
	t.Helper()
	type outcome struct {
		success bool
		code    string
	}
	oracle := map[string]outcome{}
	if family == "lookup_filter_validation" {
		for id, row := range readLookupFilterValidationOutcomes(t, filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"), "org.tsv")) {
			oracle[id] = outcome{row.Success, row.Status}
		}
	} else {
		raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
			row := strings.Split(line, "\t")
			if len(row) == 4 && row[0] == api {
				oracle[row[1]] = outcome{row[2] == "success", row[3]}
			}
		}
	}
	var body strings.Builder
	body.WriteString("Database.SaveResult result;\n")
	check := func(id string) {
		row, ok := oracle[id]
		if !ok {
			t.Fatalf("missing native row %s/%s", api, id)
		}
		fmt.Fprintf(&body, "System.assertEquals(%t, result.isSuccess(), '%s');\n", row.success, id)
		if !row.success {
			fmt.Fprintf(&body, "System.assertEquals('%s', String.valueOf(result.getErrors()[0].getStatusCode()), '%s status');\n", row.code, id)
		}
	}
	if family == "validation_rule_formula_semantics" {
		for _, row := range []struct{ id, name, number string }{
			{"N014", "Division Blank", ""},
			{"N015", "Division Zero", ", NumberValue__c=0"},
			{"N018", "Division Zero Less", ", NumberValue__c=0"},
			{"N019", "Division Zero Negative", ", NumberValue__c=0"},
			{"N020", "Division Zero Undefined", ", NumberValue__c=0"},
		} {
			fmt.Fprintf(&body, "result = Database.insert(new ValidationSample__c(Name='%s', PickValue__c='Open'%s), false);\n", row.name, row.number)
			check(row.id)
		}
	} else {
		body.WriteString("LookupFilterParent__c parent = new LookupFilterParent__c(Name='Blank target'); insert parent;\n")
		body.WriteString("result = Database.insert(new LookupFilterChild__c(Name='Blank date source', SourceTarget__c=parent.Id), false);\n")
		check("L08")
		for _, row := range []struct{ id, field string }{
			{"L14", "TextEqualsFieldTarget__c"},
			{"L15", "TextNotEqualFieldTarget__c"},
			{"L16", "TextEqualsBlankTarget__c"},
			{"L17", "TextNotEqualBlankTarget__c"},
			{"L18", "DateNotEqualTarget__c"},
		} {
			fmt.Fprintf(&body, "result = Database.insert(new LookupFilterBlankChild__c(Name='Blank comparison', %s=parent.Id), false);\n", row.field)
			check(row.id)
		}
		body.WriteString("LookupFilterChild__c updating = new LookupFilterChild__c(Name='Update blank date'); insert updating; updating.SourceTarget__c=parent.Id; result=Database.update(updating, false);\n")
		check("L08")
	}
	return body.String()
}
