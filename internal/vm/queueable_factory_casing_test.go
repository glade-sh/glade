package vm

import (
	"fmt"
	"testing"
)

// Apex method names are case-insensitive. The factory's native size and
// signature representation are owned async_execution.json rows T002/T007/T039.
// https://trailhead.salesforce.com/content/learn/modules/apex_database/apex_database_intro
// https://help.salesforce.com/s/articleView?id=release-notes.rn_apex_queueable_dupe_detection.htm&language=en_US&release=246&type=5
func TestExecQueueableSignatureFactoryCasing(t *testing.T) {
	for _, api := range []string{"62.0", "66.0", "67.0"} {
		for _, callee := range []string{
			"QueueableDuplicateSignature.builder",
			"QueueableDuplicateSignature.Builder",
			"QUEUEABLEDUPLICATESIGNATURE.BUILDER",
			"System.QueueableDuplicateSignature.BuIlDeR",
		} {
			t.Run(api+"/"+callee, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
System.QueueableDuplicateSignature.Builder signatureBuilder = %s();
System.assertEquals(0, signatureBuilder.getSize());
System.assertEquals(32, signatureBuilder.getMaxSize());
System.assertEquals(32, signatureBuilder.getRemainingSize());
QueueableDuplicateSignature signature = signatureBuilder.addString('job').addInteger(42).addId('001000000000001AAA').build();
System.assertEquals('\'job\'_42_001000000000001', signature.toString());
`, callee), CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := Execute(program, nil); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestExecQueueableSignatureFactoryCasingKeepsProjectShadow(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "QueueableDuplicateSignature"}); err != nil {
		t.Fatal(err)
	}
	body, err := CompileAnonymous(`return 'owned-factory';`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name: "QueueableDuplicateSignature.builder", ClassName: "QueueableDuplicateSignature",
		IsStatic: true, ReturnType: "String", Program: body,
	}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('owned-factory', QueueableDuplicateSignature.builder());
System.assertEquals('owned-factory', QueueableDuplicateSignature.Builder());
System.assertEquals(0, System.QueueableDuplicateSignature.Builder().getSize());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecQueueableSignatureFactoryCasingKeepsRuntimeReceivers(t *testing.T) {
	machine := New(nil)
	for _, class := range []Class{
		{Name: "OwnedSignatureFactory"},
		{Name: "SignatureOwner", Fields: map[string]Field{
			"QueueableDuplicateSignature": {
				Name: "QueueableDuplicateSignature", Type: "OwnedSignatureFactory",
				InitialValue: Object("OwnedSignatureFactory"),
			},
		}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	for _, method := range []struct{ owner, name, returns, body string }{
		{"OwnedSignatureFactory", "builder", "String", `return 'instance-factory';`},
		{"SignatureOwner", "run", "void", `
System.assertEquals('instance-factory', QueueableDuplicateSignature.builder());
System.assertEquals('instance-factory', QueueableDuplicateSignature.Builder());
`},
	} {
		body, err := CompileAnonymous(method.body)
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(Method{
			Name: method.owner + "." + method.name, ClassName: method.owner,
			ReturnType: method.returns, Program: body,
		}); err != nil {
			t.Fatal(err)
		}
	}
	program, err := CompileAnonymous(`
new SignatureOwner().run();
OwnedSignatureFactory QueueableDuplicateSignature = new OwnedSignatureFactory();
System.assertEquals('instance-factory', QueueableDuplicateSignature.builder());
System.assertEquals('instance-factory', QueueableDuplicateSignature.Builder());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
