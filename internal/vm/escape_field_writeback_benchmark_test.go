package vm

import (
	"crypto/sha256"
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
	pixels := machine.Globals["buffer"].Fields["pixels"].List
	bytes := make([]byte, len(pixels))
	for i, pixel := range pixels {
		if pixel.Kind != ValueInt || pixel.Int < 0 || pixel.Int > 255 {
			tb.Fatalf("pixel %d is not a byte: %#v", i, pixel)
		}
		bytes[i] = byte(pixel.Int)
	}
	if len(pixels) != 20000 {
		tb.Fatalf("pixel count = %d", len(pixels))
	}
	want := make([]byte, len(pixels))
	for i := range want {
		want[i] = byte(i % 256)
	}
	if got, exp := sha256.Sum256(bytes), sha256.Sum256(want); got != exp {
		tb.Fatalf("pixel digest = %x, want %x", got, exp)
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
