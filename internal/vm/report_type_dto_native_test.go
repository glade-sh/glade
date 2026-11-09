package vm

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestReportTypeDTONative(t *testing.T) {
	dir := "../resource/testdata/report_rows_capture/type-dto"
	src, err := os.ReadFile(filepath.Join(dir, "dto.apex"))
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`(?s)try\{(.*?)out\+='([^']+)='\+v\+';';\}catch\(Exception e\)\{out\+='[^']+=EXC~'\+e\.getTypeName\(\)\+'~'\+e\.getMessage\(\)\+';';\}`)
	cases := map[string]string{}
	for _, m := range pattern.FindAllStringSubmatch(string(src), -1) {
		cases["dto-"+m[2]] = m[1]
	}
	if len(cases) != 9 {
		t.Fatalf("got %d captured cases", len(cases))
	}
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) != 4 {
			t.Fatalf("bad native row %q", line)
		}
		body, ok := cases[parts[2]]
		if !ok {
			t.Fatalf("missing source for %s", parts[2])
		}
		rows++
		t.Run(parts[0]+"/"+parts[1]+"/"+parts[2], func(t *testing.T) {
			literal := strings.ReplaceAll(strings.ReplaceAll(parts[3], `\`, `\\`), `'`, `\'`)
			code := `String v;try{` + body + `}catch(Exception e){v='EXC~'+e.getTypeName()+'~'+e.getMessage();}System.assertEquals('` + literal + `',v);`
			program, e := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: parts[0]})
			if e != nil {
				t.Fatal(e)
			}
			m := New(nil)
			if parts[1] == "istest" {
				m.EnableTestContext()
			}
			if _, e = m.Execute(program); e != nil {
				t.Fatal(e)
			}
		})
	}
	if rows != 36 {
		t.Fatalf("got %d rows", rows)
	}
}
