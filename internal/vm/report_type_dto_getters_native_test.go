package vm

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestReportTypeDTOGettersNative(t *testing.T) {
	dir := "../resource/testdata/report_rows_capture/type-dto-getters"
	src, err := os.ReadFile(filepath.Join(dir, "getters.apex"))
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`(?s)try\{(.*?)out\+='([^']+)='\+before\+'~'\+count\+'~'\+JSON\.serialize\(x\)\+';';\}catch\(Exception e\)\{out\+='[^']+=EXC~'\+e\.getTypeName\(\)\+'~'\+e\.getMessage\(\)\+';';\}`)
	cases := map[string]string{}
	for _, m := range pattern.FindAllStringSubmatch(string(src), -1) {
		cases["dto-getter-"+m[2]] = m[1] + "v=before+'~'+count+'~'+JSON.serialize(x);"
	}
	if len(cases) != 6 {
		t.Fatalf("got %d captured cases", len(cases))
	}
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		p := strings.SplitN(line, "\t", 4)
		if len(p) != 4 {
			t.Fatalf("bad row %q", line)
		}
		body, ok := cases[p[2]]
		if !ok {
			t.Fatalf("missing case %s", p[2])
		}
		rows++
		t.Run(p[0]+"/"+p[1]+"/"+p[2], func(t *testing.T) {
			want := strings.ReplaceAll(strings.ReplaceAll(p[3], `\`, `\\`), `'`, `\'`)
			code := `String v;try{` + body + `}catch(Exception e){v='EXC~'+e.getTypeName()+'~'+e.getMessage();}System.assertEquals('` + want + `',v);`
			program, e := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: p[0]})
			if e != nil {
				t.Fatal(e)
			}
			m := New(nil)
			if p[1] == "istest" {
				m.EnableTestContext()
			}
			if _, e = m.Execute(program); e != nil {
				t.Fatal(e)
			}
		})
	}
	if rows != 24 {
		t.Fatalf("got %d native rows", rows)
	}
}
