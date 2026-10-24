package vm

import "testing"

// Local consistency regression: released handles must not reappear in the callout guard.
// Cleanup tolerates native cross-window handle invalidation; org.tsv pins callout eligibility.
func TestCalloutSavepointBoundaryReleasedHandleExecuteRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, operation := range []string{"release", "rollback_then_release"} {
			t.Run(api+"/"+operation, func(t *testing.T) {
				machine := New(nil)
				org := testDataOrg()
				org.APIVersion = api
				machine.SetOrg(&org)
				machine.EnableTestContext()
				machine.testContext.HTTPMock = Object("SyntheticResponse")
				source := `Savepoint point=Database.setSavepoint();`
				if operation == "rollback_then_release" {
					source += `Database.setSavepoint();`
				}
				source += `Test.startTest();`
				if operation == "release" {
					source += `try{Database.releaseSavepoint(point);}catch(TypeException ignored){}`
				} else {
					source += `try{Database.rollback(point);}catch(TypeException ignored){}`
				}
				source += `Test.stopTest();`
				if operation == "rollback_then_release" {
					source += `try{Database.releaseSavepoint(point);}catch(TypeException ignored){}`
				}
				source += `HttpRequest request=new HttpRequest();request.setEndpoint('https://native.invalid');request.setMethod('GET');new Http().send(request);`
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = machine.Execute(program)
				if err != nil {
					t.Fatalf("native permits the post-stop callout: %v", err)
				}
			})
		}
	}
}
