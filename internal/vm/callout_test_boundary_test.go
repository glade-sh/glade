package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native rows cover both callout dispatchers across the test limit boundaries.
func TestCalloutUncommittedHTTPAndSOAPNamedIsTestBoundaryRoutes(t *testing.T) {
	testCalloutUncommittedBoundaryRoutes(t, false)
}

func TestCalloutUncommittedHTTPAndSOAPTestExecutionEntryBoundaryRoutes(t *testing.T) {
	testCalloutUncommittedBoundaryRoutes(t, true)
}

func testCalloutUncommittedBoundaryRoutes(t *testing.T, testEntry bool) {
	t.Helper()
	route := "named_istest"
	if testEntry {
		route = "test_execution_entry"
	}
	for _, api := range []string{"62.0", "67.0"} {
		raw, err := os.ReadFile(filepath.Join("../apextest/testdata/conformance/callout_uncommitted_work/boundaries/native", "api"+strings.TrimSuffix(api, ".0"), "istest", "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
			id, want, ok := strings.Cut(line, "\t")
			if !ok {
				t.Fatal(line)
			}
			t.Run(api+"/"+route+"/"+id, func(t *testing.T) {
				machine := New(nil)
				org := testDataOrg()
				org.APIVersion = api
				machine.SetOrg(&org)
				machine.EnableTestContext()
				mock, err := CompileAnonymous(`HttpResponse response=new HttpResponse();response.setStatusCode(204);return response;`)
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{Name: "BoundaryReply", Methods: map[string]Method{"respond": {Name: "BoundaryReply.respond", ClassName: "BoundaryReply", ReturnType: "HttpResponse", Params: []Param{{Name: "request", Type: "HttpRequest"}}, Program: mock}}}); err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{Name: "BoundarySoapReply", Methods: map[string]Method{"doInvoke": {Name: "BoundarySoapReply.doInvoke", ClassName: "BoundarySoapReply", ReturnType: "void", Params: []Param{{Name: "stub", Type: "Object"}, {Name: "request", Type: "Object"}, {Name: "response", Type: "Map<String,Object>"}, {Name: "endpoint", Type: "String"}, {Name: "soapAction", Type: "String"}, {Name: "requestName", Type: "String"}, {Name: "responseNS", Type: "String"}, {Name: "responseName", Type: "String"}, {Name: "responseType", Type: "String"}}}}}); err != nil {
					t.Fatal(err)
				}
				machine.testContext.HTTPMock = Object("BoundaryReply")
				machine.testContext.WebServiceMock = Object("BoundarySoapReply")
				source := ""
				if strings.HasPrefix(id, "before_") || strings.HasPrefix(id, "both_") {
					source += `insert new Account(Name='Before boundary');`
				}
				source += `Test.startTest();`
				if strings.HasPrefix(id, "inside_") || strings.HasPrefix(id, "both_") {
					source += `insert new Account(Name='Inside boundary');`
				}
				if !strings.Contains(id, "_inside_") {
					source += `Test.stopTest();`
				}
				if strings.HasPrefix(id, "after_") {
					source += `insert new Account(Name='After boundary');`
				}
				call := `HttpRequest request=new HttpRequest();request.setEndpoint('https://native.invalid');request.setMethod('GET');new Http().send(request);`
				if strings.HasSuffix(id, "_soap") {
					call = `WebServiceCallout.invoke(new Account(),new Account(),new Map<String,Object>(),new String[]{'https://native.invalid','action','urn:probe','request','urn:probe','response','Account'});`
				}
				source += `String observed='ok';try{` + call + `}catch(Exception failure){observed=failure.getTypeName()+'|'+failure.getMessage();}return observed;`
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				method := Method{Name: "BoundaryTest." + id, ClassName: "BoundaryTest", ReturnType: "String", IsStatic: true, Modifiers: []string{"istest"}, Program: program, APIVersion: api}
				if err := machine.RegisterClass(Class{Name: "BoundaryTest", Modifiers: []string{"istest"}, Methods: map[string]Method{id: method}}); err != nil {
					t.Fatal(err)
				}
				var got Value
				if testEntry {
					invocation, err := CompileAnonymousWithOptions("System.debug("+method.Name+"());", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.ExecuteTestInClass(invocation, "BoundaryTest")
					if err != nil {
						t.Fatal(err)
					}
					if len(result.Debug) != 1 {
						t.Fatalf("missing test-entry observation: %v", result.Debug)
					}
					got = String(result.Debug[0])
				} else {
					got, err = machine.CallStatic(method.Name, nil)
					if err != nil {
						t.Fatal(err)
					}
				}
				if got.Kind != ValueString || got.Text != want {
					t.Errorf("native %q; Glade %q", want, got.Text)
				}
			})
		}
	}
}
