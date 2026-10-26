package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Unit twins use the same native cells as the project-level route test.
func TestStandardEventPublishRouteTwinsNative(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/standard_event_route_twins/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	shapes := map[string]string{
		"publish_scalar":          `SObject value=new BatchApexErrorEvent(); Database.SaveResult saved=EventBus.publish(value); List<Database.SaveResult> results=new List<Database.SaveResult>{saved};`,
		"publish_erased_single":   `List<Database.SaveResult> results=EventBus.publish(new List<SObject>{new BatchApexErrorEvent()});`,
		"publish_concrete_single": `List<SObject> values=new List<BatchApexErrorEvent>{new BatchApexErrorEvent()}; List<Database.SaveResult> results=EventBus.publish(values);`,
		"publish_erased_multi":    `List<Database.SaveResult> results=EventBus.publish(new List<SObject>{new BatchApexErrorEvent(),new BatchApexErrorEvent()});`,
		"publish_concrete_multi":  `List<SObject> values=new List<BatchApexErrorEvent>{new BatchApexErrorEvent(),new BatchApexErrorEvent()}; List<Database.SaveResult> results=EventBus.publish(values);`,
	}
	statements := map[string]string{
		"dml_scalar":          `SObject value=new BatchApexErrorEvent(); insert value; observed='inserted';`,
		"dml_erased_single":   `List<SObject> values=new List<SObject>{new BatchApexErrorEvent()}; insert values; observed='inserted';`,
		"dml_concrete_single": `List<SObject> values=new List<BatchApexErrorEvent>{new BatchApexErrorEvent()}; insert values; observed='inserted';`,
	}
	shapes["database_erased_scalar"] = `SObject value=new BatchApexErrorEvent(); Database.SaveResult saved=Database.insert(value,false); List<Database.SaveResult> results=new List<Database.SaveResult>{saved};`
	for id := range statements {
		shapes[id] = ""
	}
	for _, row := range rows[1:] {
		api, route := row[0], row[1]
		var answers map[string]string
		if err := json.Unmarshal([]byte(row[2]), &answers); err != nil {
			t.Fatal(err)
		}
		for id, shape := range shapes {
			t.Run(api+"/"+route+"/"+id, func(t *testing.T) {
				observe := shape + `
List<String> summaries=new List<String>();
for(Database.SaveResult savedResult:results){
 List<String> codes=new List<String>();
 for(Database.Error detail:savedResult.getErrors()) codes.add(String.valueOf(detail.getStatusCode()));
 summaries.add(savedResult.isSuccess()+'|'+(savedResult.getId()==null)+'|'+savedResult.getErrors().size()+'|'+String.join(codes,','));
}
observed=String.join(summaries,';');`
				if statement, ok := statements[id]; ok {
					observe = statement
				}
				body := `String observed; try { ` + observe + `
} catch(Exception problem){observed=problem.getTypeName()+'|'+problem.getMessage();}
System.assertEquals('` + strings.ReplaceAll(answers[id], "'", "\\'") + `',observed);`
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				org.APIVersion = api
				storage.EnsureDeterministicPlatformData(&org)
				storage.EnsureStandardObject(&org, "BatchApexErrorEvent")
				machine := New(nil)
				machine.SetOrg(&org)
				if route != "anonymous" {
					if err := machine.RegisterClass(Class{Name: "EventRouteHelper", APIVersion: api, Methods: map[string]Method{"observe": {Name: "EventRouteHelper.observe", ClassName: "EventRouteHelper", ReturnType: "void", IsStatic: true, APIVersion: api, Program: program}}}); err != nil {
						t.Fatal(err)
					}
					program, err = CompileAnonymousWithOptions("EventRouteHelper.observe();", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
				}
				if route == "isTest" {
					machine.EnableTestContext()
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
