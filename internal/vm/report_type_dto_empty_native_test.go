package vm

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestReportTypeDTOEmptyNative(t *testing.T) {
	dir := "../resource/testdata/report_rows_capture/type-dto-empty"
	src, err := os.ReadFile(filepath.Join(dir, "empty.apex"))
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`(?s)try\{(.*?)out\+='([^']+)='\+JSON\.serialize\(x\)\+';';\}catch\(Exception e\)\{out\+='[^']+=EXC~'\+e\.getTypeName\(\)\+'~'\+e\.getMessage\(\)\+';';\}`)
	cases := map[string]string{}
	for _, m := range pattern.FindAllStringSubmatch(string(src), -1) {
		cases["dto-empty-"+m[2]] = m[1] + "v=JSON.serialize(x);"
	}
	if len(cases) != 10 {
		t.Fatalf("got %d captured cases", len(cases))
	}
	cases["dto-empty-column-getter"] = `Reports.ReportTypeColumn x=(Reports.ReportTypeColumn)JSON.deserialize('{"filterValues":null}',Reports.ReportTypeColumn.class);String before=JSON.serialize(x);Integer count=x.getFilterValues().size();v=before+'~'+count+'~'+JSON.serialize(x);`
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
	if rows != 44 {
		t.Fatalf("got %d native rows", rows)
	}
}
