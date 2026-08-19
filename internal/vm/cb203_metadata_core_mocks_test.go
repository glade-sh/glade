package vm

import "testing"

func TestCB203MetadataDeploymentInvokesCallbackWithResult(t *testing.T) {
	callbackProgram, err := CompileAnonymous(`
CB203MetadataCallback.called = true;
System.assertEquals('0Af000000000001', (String)result.id);
System.assertEquals('SUCCEEDED', result.status.name());
System.assertEquals(1, result.numberComponentsTotal);
System.assertEquals('0Af000000000001', (String)context.getCallbackJobId());
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Metadata.DeployContainer container = new Metadata.DeployContainer();
Metadata.CustomMetadata item = new Metadata.CustomMetadata();
item.fullName = 'Feature.Callback';
container.addMetadata(item);
CB203MetadataCallback callback = new CB203MetadataCallback();
Id deploymentId = Metadata.Operations.enqueueDeployment(container, callback);
System.assertEquals('0Af000000000001', (String)deploymentId);
System.assertEquals(true, CB203MetadataCallback.called);
`)
	if err != nil {
		t.Fatal(err)
	}

	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "CB203MetadataCallback",
		Interfaces: []string{"Metadata.DeployCallback"},
		StaticFields: map[string]Field{
			"called": {Name: "called", Type: "Boolean", Value: Bool(false)},
		},
		Methods: map[string]Method{
			"handleResult": {
				Name:       "CB203MetadataCallback.handleResult",
				ClassName:  "CB203MetadataCallback",
				ReturnType: "void",
				Params: []Param{
					{Name: "result", Type: "Metadata.DeployResult"},
					{Name: "context", Type: "Metadata.DeployCallbackContext"},
				},
				Program: callbackProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	org := customDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB203MetadataDeploymentInvokesCallbackWithFailureResult(t *testing.T) {
	callbackProgram, err := CompileAnonymous(`
CB203MetadataCallback.failed = true;
System.assertEquals('FAILED', result.status.name());
System.assertEquals(false, result.success);
System.assertEquals('0Af000000000001', (String)context.getCallbackJobId());
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Metadata.DeployContainer container = new Metadata.DeployContainer();
Metadata.CustomMetadata item = new Metadata.CustomMetadata();
item.fullName = 'InvalidCallbackFailure';
container.addMetadata(item);
CB203MetadataCallback callback = new CB203MetadataCallback();
Id deploymentId = Metadata.Operations.enqueueDeployment(container, callback);
System.assertEquals('0Af000000000001', (String)deploymentId);
System.assertEquals(true, CB203MetadataCallback.failed);
`)
	if err != nil {
		t.Fatal(err)
	}

	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "CB203MetadataCallback",
		Interfaces: []string{"Metadata.DeployCallback"},
		StaticFields: map[string]Field{
			"failed": {Name: "failed", Type: "Boolean", Value: Bool(false)},
		},
		Methods: map[string]Method{
			"handleResult": {
				Name:       "CB203MetadataCallback.handleResult",
				ClassName:  "CB203MetadataCallback",
				ReturnType: "void",
				Params: []Param{
					{Name: "result", Type: "Metadata.DeployResult"},
					{Name: "context", Type: "Metadata.DeployCallbackContext"},
				},
				Program: callbackProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	org := customDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
