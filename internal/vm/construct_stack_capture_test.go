package vm

import "testing"

func constructStackCaptureMachine(t testing.TB) *VM {
	t.Helper()
	machine := New(nil)
	for _, class := range []Class{
		{Name: "PlainRow", Fields: map[string]Field{"label": {Name: "label", Type: "String"}}},
		{Name: "RowException", SuperClass: "Exception"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}

// A source-defined exception still records its construction line and stack
// trace; a plain source class is constructed without capturing the frames.
func TestSourceExceptionConstructionKeepsStackAndPlainObjectsSkipIt(t *testing.T) {
	machine := constructStackCaptureMachine(t)
	program, err := CompileAnonymous(`PlainRow row = new PlainRow();
RowException failure = new RowException('row failed');
System.assertEquals(2, failure.getLineNumber());
System.assertEquals('AnonymousBlock: line 2, column 1', failure.getStackTraceString());
System.assertEquals('row failed', failure.getMessage());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	plain, err := CompileAnonymous(`PlainRow row = new PlainRow(); row.label = 'x';`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(plain); err != nil {
		t.Fatal(err)
	}
	withFrames := testing.AllocsPerRun(50, func() {
		if _, err := machine.Execute(plain); err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("allocations per plain construction program: %.0f", withFrames)
	exception, err := CompileAnonymous(`RowException failure = new RowException('row failed'); String trace = failure.getStackTraceString();`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(exception); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatalf("exception program after plain constructions: %v", err)
	}
}

func BenchmarkConstructPlainSourceObject(b *testing.B) {
	machine := constructStackCaptureMachine(b)
	program, err := CompileAnonymous(`for (Integer i = 0; i < 1000; i++) { PlainRow row = new PlainRow(); row.label = 'x'; }`)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := machine.Execute(program); err != nil {
			b.Fatal(err)
		}
	}
}
