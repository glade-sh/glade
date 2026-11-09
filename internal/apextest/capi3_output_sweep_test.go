package apextest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capi3_output_sweep/org.tsv: for every ConnectApi type that instantiates
// and serializes at least one key, JSON.serialize(Type.forName(name).newInstance()) carries the same
// "type" discriminator in glade (value, null, or absent).
func TestCapi3OutputDiscriminatorSweep(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "conformance", "capi3_output_sweep", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	type row struct{ name, typ string }
	var rows []row
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		// Zero keys marks a type of a licensed cloud the sweep org lacks: its
		// properties serialize empty there, so the discriminator is unobserved.
		if len(p) != 3 || p[0] == "name" || p[1] == "X" || p[1] == "0" {
			continue
		}
		rows = append(rows, row{p[0], p[2]})
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if len(rows) < 1200 {
		t.Fatalf("native rows=%d", len(rows))
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			p := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			const batch = 150
			for start, idx := 0, 0; start < len(rows); start, idx = start+batch, idx+1 {
				end := min(start+batch, len(rows))
				var b strings.Builder
				fmt.Fprintf(&b, "@IsTest private class CapiSweep%d { @IsTest static void probe() {\n", idx)
				for _, r := range rows[start:end] {
					want := r.typ
					check := "System.assertEquals('" + want + "',got,'" + r.name + "');"
					if want == "-" {
						// A partly licensed type serializes few keys natively and no
						// "type"; glade emits the unassigned null but never a value.
						check = "System.assert(got=='-' || got=='null',got + ' ' + '" + r.name + "');"
					}
					fmt.Fprintf(&b, "{ Object v=Type.forName('%s').newInstance(); Map<String,Object> m=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(v)); String got=m.containsKey('type')?''+m.get('type'):'-'; %s }\n", r.name, check)
				}
				b.WriteString("} }")
				p.writeClass(fmt.Sprintf("CapiSweep%d", idx), b.String())
			}
			run := p.run()
			s := run.Summary()
			if s.Failed != 0 || s.Passed != s.Total || s.Total == 0 {
				t.Fatalf("summary=%#v problem=%q", s, firstRunProblem(run))
			}
		})
	}
}
