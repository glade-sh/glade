package vm

import (
	"testing"
)

// Native before_after_http permits the callout; after_after_http rejects new DML.
func TestCalloutUncommittedHTTPExecuteTestContextDMLBoundaryRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, afterDML := range []bool{false, true} {
			route := "pre_start_dml"
			want := "ok"
			after := ""
			if afterDML {
				route = "post_stop_dml"
				after = `insert new Account(Name='After');`
				want = "System.CalloutException|You have uncommitted work pending. Please commit or rollback before calling out"
			}
			t.Run(api+"/execute_test_context/"+route, func(t *testing.T) {
				machine := New(nil)
				org := testDataOrg()
				org.APIVersion = api
				machine.SetOrg(&org)
				machine.EnableTestContext()
				machine.testContext.HTTPMock = Object("SyntheticResponse")
				source := `insert new Account(Name='Before');Test.startTest();Test.stopTest();` + after + `String observed='ok';try{HttpRequest request=new HttpRequest();request.setEndpoint('https://native.invalid');request.setMethod('GET');new Http().send(request);}catch(Exception failure){observed=failure.getTypeName()+'|'+failure.getMessage();}System.debug(observed);`
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Debug) != 1 || result.Debug[0] != want {
					t.Fatalf("native %q; Glade %q", want, result.Debug)
				}
			})
		}
	}
}
