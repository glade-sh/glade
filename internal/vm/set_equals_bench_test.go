package vm

import (
	"fmt"
	"testing"
)

func benchmarkSetEquals(b *testing.B, decimal bool) {
	items := func(offset int) string {
		out := ""
		for i := 0; i < 3000; i++ {
			if decimal {
				out += fmt.Sprintf("s.add(%d.50);\n", i+offset)
			} else {
				out += fmt.Sprintf("s.add(%d);\n", i+offset)
			}
		}
		return out
	}
	kind := "Integer"
	if decimal {
		kind = "Decimal"
	}
	program, err := CompileAnonymous(fmt.Sprintf("Set<%s> s = new Set<%s>();\n%sSet<%s> a = s; s = new Set<%s>();\n%sSet<%s> b = s;\nBoolean r = false; for (Integer i = 0; i < 5; i++) { r = a == b; }", kind, kind, items(0), kind, kind, items(0), kind))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := New(nil).Execute(program); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSetEqualsInteger3000(b *testing.B) { benchmarkSetEquals(b, false) }
func BenchmarkSetEqualsDecimal3000(b *testing.B) { benchmarkSetEquals(b, true) }
