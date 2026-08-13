package vm

import "testing"

func TestExecMetadataCustomMetadataAssignsToMetadataBase(t *testing.T) {
	program, err := CompileAnonymous(`
Metadata.CustomMetadata custom = new Metadata.CustomMetadata();
custom.fullName = 'Feature.Default';
Metadata.Metadata base = custom;
System.assertEquals('Feature.Default', base.fullName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
