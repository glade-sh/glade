package vm

import "testing"

func TestStubCallbackPreservesConcreteListArgumentType(t *testing.T) {
	machine := newStubIdentityHashVM(t)
	for _, class := range []Class{
		{Name: "ArgumentShapeService", Methods: map[string]Method{
			"write": {Access: "public", ReturnType: "void", Params: []Param{{Name: "records", Type: "List<SObject>"}}},
		}},
		{Name: "ArgumentShapeProvider", Interfaces: []string{"StubProvider"}, Methods: map[string]Method{
			"handleMethodCall": {
				Access: "public", ReturnType: "Object",
				Params: []Param{
					{Name: "stub", Type: "Object"}, {Name: "member", Type: "String"},
					{Name: "returnType", Type: "Type"}, {Name: "types", Type: "List<Type>"},
					{Name: "names", Type: "List<String>"}, {Name: "args", Type: "List<Object>"},
				},
				Program: collisionProgram(t, `
try {
 Date unexpected = (Date)args[0];
 System.assert(false, 'expected a type exception');
} catch (System.TypeException problem) {
 System.assert(problem.getMessage().contains('List<Account>'), 'callback argument lost concrete list type');
}
return null;
`),
			},
		}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(collisionProgram(t, `
ArgumentShapeService service=(ArgumentShapeService)Test.createStub(ArgumentShapeService.class,new ArgumentShapeProvider());
service.write(new List<Account>{new Account(Name='shape')});
`)); err != nil {
		t.Fatal(err)
	}
}
