package vm

import "testing"

// The named API 62/67 observations in stub_numeric_arguments/org.tsv also
// exercise interface and static facade calls through the Apex test runner.
func TestStubNumericCallbackArguments(t *testing.T) {
	for _, source := range []string{
		"System.assertEquals('stub', service.format(7));",
		"System.assertEquals('stub', service.format(7, 'pattern'));",
	} {
		t.Run(source, func(t *testing.T) {
			machine := New(nil)
			machine.EnableTestContext()
			for _, class := range []Class{{Name: "AmountService"}, {Name: "AmountRecorder", Interfaces: []string{"StubProvider"}}} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			for _, params := range [][]Param{
				{{Name: "amount", Type: "Decimal"}},
				{{Name: "amount", Type: "Decimal"}, {Name: "pattern", Type: "String"}},
			} {
				if err := machine.RegisterMethod(Method{
					Name: "AmountService.format", ClassName: "AmountService", Access: "public",
					ReturnType: "String", Params: params,
					Program: collisionProgram(t, "return 'real';"),
				}); err != nil {
					t.Fatal(err)
				}
			}
			if err := machine.RegisterMethod(Method{
				Name: "AmountRecorder.handleMethodCall", ClassName: "AmountRecorder", Access: "public",
				ReturnType: "Object", Params: []Param{
					{Name: "stub", Type: "Object"}, {Name: "member", Type: "String"},
					{Name: "returnType", Type: "Type"}, {Name: "types", Type: "List<Type>"},
					{Name: "names", Type: "List<String>"}, {Name: "args", Type: "List<Object>"},
				}, Program: collisionProgram(t, `
System.assertEquals('format', member);
System.assert(returnType.equals(String.class));
System.assert(types[0].equals(Decimal.class));
System.assertEquals('amount', names[0]);
List<Object> expected = new List<Object>{Decimal.valueOf('7')};
if (args.size() == 2) {
 System.assert(types[1].equals(String.class));
 System.assertEquals('pattern', names[1]);
 expected.add('pattern');
}
System.assert(args == expected, 'callback arguments must match declared numeric types');
System.assert(args.equals(expected), 'callback argument list equality');
return 'stub';
`),
			}); err != nil {
				t.Fatal(err)
			}
			program := collisionProgram(t, `
AmountService service = (AmountService)Test.createStub(AmountService.class, new AmountRecorder());
`+source)
			_, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
