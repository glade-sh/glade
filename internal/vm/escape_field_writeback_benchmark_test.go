package vm

import (
	"crypto/sha256"
	"fmt"
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
	if got := fmt.Sprintf("%x", sha256.Sum256(bytes)); got != "c8f5d0341d54d951a71b136e6e2afcb14d11ed8489a7ae126a8fee0df6ecf193" {
		tb.Fatalf("pixel digest = %s", got)
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
