package vm

import "testing"

// BenchmarkSuperMethodCall measures a loop of super.method() calls, the path
// that carries the CartExtension super-dispatch guard.
func BenchmarkSuperMethodCall(b *testing.B) {
	parentProgram, err := CompileAnonymous("return 1;")
	if err != nil {
		b.Fatal(err)
	}
	childProgram, err := CompileAnonymous("return super.score() + 1;")
	if err != nil {
		b.Fatal(err)
	}
	program, err := CompileAnonymous(`
Child c = new Child();
Integer total = 0;
for (Integer i = 0; i < 2000; i++) { total += c.score(); }
System.assertEquals(4000, total);
`)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		machine := New(nil)
		if err := machine.RegisterClass(Class{
			Name:    "Parent",
			Methods: map[string]Method{"score": {Name: "Parent.score", ClassName: "Parent", ReturnType: "Integer", Program: parentProgram}},
		}); err != nil {
			b.Fatal(err)
		}
		if err := machine.RegisterClass(Class{
			Name:       "Child",
			SuperClass: "Parent",
			Methods:    map[string]Method{"score": {Name: "Child.score", ClassName: "Child", ReturnType: "Integer", Program: childProgram}},
		}); err != nil {
			b.Fatal(err)
		}
		if _, err := machine.Execute(program); err != nil {
			b.Fatal(err)
		}
	}
}
