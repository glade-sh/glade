package apextest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapi2CapturedOutputSubtypeFamily(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "conformance", "capi2_output_subtypes", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows := map[string][][2]string{}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		p := strings.SplitN(scan.Text(), "\t", 4)
		if len(p) != 4 || p[0] == "api" || p[3] == "null" || strings.HasPrefix(p[3], "EX:") {
			continue
		}
		rows[p[0]] = append(rows[p[0]], [2]string{p[2], p[3]})
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			if len(rows[api]) != 89 {
				t.Fatalf("native discriminator rows=%d", len(rows[api]))
			}
			p := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			var b strings.Builder
			b.WriteString(`@IsTest private class CapiOutputSubtypeTest { @IsTest static void probe() {`)
			for _, row := range rows[api] {
				fmt.Fprintf(&b, `{ Object v=Type.forName('%s').newInstance(); Map<String,Object> m=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(v)); System.assertEquals('%s',m.get('type'),'%s'); }`, row[0], row[1], row[0])
			}
			b.WriteString(`} }`)
			p.writeClass("CapiOutputSubtypeTest", b.String())
			run := p.run()
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
				t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
			}
		})
	}
}
