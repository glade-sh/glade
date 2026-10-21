package vm

import "testing"

func TestExecDoubleValueOfSelectsCapturedOverload(t *testing.T) {
	machine := New(nil)
	methods := map[string]Method{}
	for _, typeName := range []string{"Object", "Integer", "Long", "Decimal", "Double", "String"} {
		body, err := CompileAnonymous("return '" + typeName + "|' + String.valueOf(glyph);")
		if err != nil {
			t.Fatal(err)
		}
		methods[typeName] = Method{Name: "framework_MethodVerifier.selectGlyph", ClassName: "framework_MethodVerifier", IsStatic: true, ReturnType: "String", Params: []Param{{Name: "glyph", Type: typeName}}, Program: body}
	}
	if err := machine.RegisterClass(Class{Name: "framework_MethodVerifier", Methods: methods}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("System.assertEquals('Double|1.0', framework_MethodVerifier.selectGlyph(Double.valueOf('1.0'))); System.assertEquals('Object|1.0', framework_MethodVerifier.selectGlyph((Object)Double.valueOf('1.0'))); System.assertEquals('Decimal|1.0', framework_MethodVerifier.selectGlyph(Decimal.valueOf('1.0')));")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
