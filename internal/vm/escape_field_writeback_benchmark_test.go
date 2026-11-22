package vm

import (
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

func escapeFieldWritebackProgram(tb testing.TB) ir.Program {
	tb.Helper()
	source, err := os.ReadFile("testdata/escape_field_writeback/raster.apex")
	if err != nil {
		tb.Fatal(err)
	}
	program, err := CompileAnonymous(string(source))
	if err != nil {
		tb.Fatal(err)
	}
	return program
}

func runEscapeFieldWriteback(tb testing.TB, program ir.Program) {
	tb.Helper()
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "RasterBuffer",
		Fields: map[string]Field{
			"pixels": {Name: "pixels", Type: "List<Integer>", InitialValue: List()},
		},
	}); err != nil {
		tb.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		tb.Fatal(err)
	}
}

func TestEscapeFieldWritebackSynthetic(t *testing.T) {
	runEscapeFieldWriteback(t, escapeFieldWritebackProgram(t))
}

func BenchmarkEscapeFieldWritebackSynthetic(b *testing.B) {
	program := escapeFieldWritebackProgram(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		runEscapeFieldWriteback(b, program)
	}
}
