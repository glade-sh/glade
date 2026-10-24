package vm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Replay the native synthetic class bodies through each VM test entry point.
func TestCalloutSavepointBoundaryHTTPAndSOAPVMRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/callout_uncommitted_work/savepoint_boundaries"
	var cases []struct{ ID, Setup, Callout string }
	raw, err := os.ReadFile(filepath.Join(fixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		raw, err := os.ReadFile(filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"), "istest", "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		rows := map[string]string{}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
			id, value, ok := strings.Cut(line, "\t")
			if !ok {
				t.Fatal(line)
			}
			rows[id] = value
		}
		if len(rows) != len(cases) {
			t.Fatalf("native rows=%d cases=%d", len(rows), len(cases))
		}
		for _, route := range []string{"execute_test_context", "named_istest", "test_execution_entry"} {
			for _, row := range cases {
				t.Run(api+"/"+route+"/"+row.ID, func(t *testing.T) {
					machine := New(nil)
					org := testDataOrg()
					org.APIVersion = api
					machine.SetOrg(&org)
					machine.EnableTestContext()
					machine.testContext.HTTPMock = Object("SyntheticResponse")
					if err := machine.RegisterClass(Class{Name: "WindowSoapReply", Methods: map[string]Method{"doInvoke": {Name: "WindowSoapReply.doInvoke", ClassName: "WindowSoapReply", ReturnType: "void", Params: []Param{{Name: "stub", Type: "Object"}, {Name: "request", Type: "Object"}, {Name: "response", Type: "Map<String,Object>"}, {Name: "endpoint", Type: "String"}, {Name: "soapAction", Type: "String"}, {Name: "requestName", Type: "String"}, {Name: "responseNS", Type: "String"}, {Name: "responseName", Type: "String"}, {Name: "responseType", Type: "String"}}}}}); err != nil {
						t.Fatal(err)
					}
					machine.testContext.WebServiceMock = Object("WindowSoapReply")
					call := `HttpRequest request=new HttpRequest();request.setEndpoint('https://native.invalid');request.setMethod('GET');new Http().send(request);`
					if row.Callout == "soap" {
						call = `WebServiceCallout.invoke(new Account(),new Account(),new Map<String,Object>(),new String[]{'https://native.invalid','action','urn:probe','request','urn:probe','response','Account'});`
					}
					source := row.Setup + `String observed='ok';try{` + call + `}catch(Exception failure){observed=failure.getTypeName()+'|'+failure.getMessage();}`
					suffix := `return observed;`
					if route == "execute_test_context" {
						suffix = `System.debug(observed);`
					}
					program, err := CompileAnonymousWithOptions(source+suffix, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					got := ""
					if route == "execute_test_context" {
						result, err := machine.Execute(program)
						if err != nil {
							t.Fatal(err)
						}
						if len(result.Debug) != 1 {
							t.Fatalf("observations: %v", result.Debug)
						}
						got = result.Debug[0]
					} else {
						method := Method{Name: "WindowTest." + row.ID, ClassName: "WindowTest", ReturnType: "String", IsStatic: true, Modifiers: []string{"istest"}, Program: program, APIVersion: api}
						if err := machine.RegisterClass(Class{Name: "WindowTest", Modifiers: []string{"istest"}, Methods: map[string]Method{row.ID: method}}); err != nil {
							t.Fatal(err)
						}
						if route == "named_istest" {
							value, err := machine.CallStatic(method.Name, nil)
							if err != nil {
								t.Fatal(err)
							}
							got = value.Text
						} else {
							invocation, err := CompileAnonymousWithOptions("System.debug("+method.Name+"());", CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							result, err := machine.ExecuteTestInClass(invocation, "WindowTest")
							if err != nil {
								t.Fatal(err)
							}
							if len(result.Debug) != 1 {
								t.Fatalf("observations: %v", result.Debug)
							}
							got = result.Debug[0]
						}
					}
					if got != rows[row.ID] {
						t.Errorf("native %q; Glade %q", rows[row.ID], got)
					}
				})
			}
		}
	}
}
