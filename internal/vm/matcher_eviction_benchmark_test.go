package vm

import (
	"fmt"
	"strings"
	"testing"
)

// The same uncaptured optional-pattern outcome is required on both trees.
func BenchmarkMatcherShortLivedLargeSubject(b *testing.B) {
	input := "😀" + strings.Repeat("é", 4096)
	source := fmt.Sprintf(`String subject = '%s'; Integer positions = 0;
for (Integer i = 0; i < 32; i++) {
 Matcher m = Pattern.compile('z?').matcher(subject);
 if (m.find() && m.find()) positions += m.start();
}`, input)
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		b.Fatal(err)
	}
	machine := New(nil)
	machine.SetTraceEnabled(false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clear(machine.Globals)
		clear(machine.VarTypes)
		machine.ResetLimits()
		result, err := machine.Execute(program)
		if err != nil {
			b.Fatal(err)
		}
		if got := result.Vars["positions"]; got.Kind != ValueInt || got.Int != 64 {
			b.Fatalf("positions = %v, want 64", got)
		}
	}
}
