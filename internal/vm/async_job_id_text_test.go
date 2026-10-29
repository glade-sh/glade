package vm

import "testing"

func TestQueueableContextGetJobIDConvertsTo18CharacterStringVMIsTestDrainRoute(t *testing.T) {
	queueProgram, err := CompileAnonymous(`
String contextId = context.getJobId();
System.assertEquals(18, contextId.length());
System.assertEquals('707', contextId.substring(0, 3));
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
System.enqueueJob(new Worker());
Test.stopTest();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "Worker",
		Interfaces: []string{"Queueable"},
		Methods: map[string]Method{
			"execute": {
				Name:       "Worker.execute",
				ClassName:  "Worker",
				ReturnType: "void",
				Params:     []Param{{Name: "context", Type: "QueueableContext"}},
				Program:    queueProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
