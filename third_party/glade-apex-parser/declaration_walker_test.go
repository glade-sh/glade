//go:build cgo

package apexast

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestDeclarationWalkerOutputIdentity(t *testing.T) {
	const source = "class Probe { String currency; void run(String integer) { String select; } }"
	const want = `[{"severity":"error","code":"APEXPARSE002","message":"Identifier name is reserved: currency","file":"Probe.cls","range":{"start":{"line":1,"column":22,"offset":21},"end":{"line":1,"column":30,"offset":29}},"excerpt":"class Probe { String currency; void run(String integer) { String select; } }"},{"severity":"error","code":"APEXPARSE002","message":"Identifier name is reserved: integer","file":"Probe.cls","range":{"start":{"line":1,"column":48,"offset":47},"end":{"line":1,"column":55,"offset":54}},"excerpt":"class Probe { String currency; void run(String integer) { String select; } }"},{"severity":"error","code":"APEXPARSE002","message":"Unexpected token 'select'.","file":"Probe.cls","range":{"start":{"line":1,"column":66,"offset":65},"end":{"line":1,"column":72,"offset":71}},"excerpt":"class Probe { String currency; void run(String integer) { String select; } }"}]`
	p := NewParser()
	defer p.Close()
	for _, tc := range []struct{ source, want string }{{source, want}, {"class Probe { String monetary; void run(String count) { String chosen; } }", "null"}} {
		got, err := json.Marshal(p.ParseSource("Probe.cls", tc.source).Diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tc.want {
			t.Fatalf("diagnostics for %q = %s, want %s", tc.source, got, tc.want)
		}
	}
}

func BenchmarkParseProject128(b *testing.B) {
	sources := make([]string, 128)
	for i := range sources {
		var s strings.Builder
		fmt.Fprintf(&s, "public class Probe%d {\n", i)
		for j := 0; j < 16; j++ {
			fmt.Fprintf(&s, "public static Integer method%d(Integer x) { Integer v = 0; for (Integer k = 0; k < x; k++) { v += k; } return v; }\n", j)
		}
		s.WriteString("}\n")
		sources[i] = s.String()
	}
	p := NewParser()
	defer p.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, src := range sources {
			f := p.ParseSource("Probe.cls", src)
			if len(f.Diagnostics) != 0 {
				b.Fatal(f.Diagnostics)
			}
		}
	}
}
