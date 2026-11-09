package vm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReportCatalogRoundTripNative(t *testing.T) {
	dir := "../resource/testdata/report_rows_capture/type-catalog-roundtrip"
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		p := strings.SplitN(line, "\t", 4)
		if len(p) != 4 {
			t.Fatalf("bad native row %q", line)
		}
		name := strings.TrimPrefix(p[2], "catalog-roundtrip-")
		if name == p[2] {
			t.Fatalf("bad case %s", p[2])
		}
		rows++
		t.Run(p[0]+"/"+p[1]+"/"+name, func(t *testing.T) {
			version := strings.TrimSuffix(p[0], ".0")
			before, e := os.ReadFile(filepath.Join(dir, "before-"+name+"-"+version+".json"))
			if e != nil {
				t.Fatal(e)
			}
			after, e := os.ReadFile(filepath.Join(dir, "after-"+name+"-"+version+".json"))
			if e != nil {
				t.Fatal(e)
			}
			input := strings.TrimSpace(string(before))
			literal := strings.ReplaceAll(strings.ReplaceAll(input, `\`, `\\`), `'`, `\'`)
			code := `String raw='` + literal + `';Reports.ReportTypeMetadata b=(Reports.ReportTypeMetadata)JSON.deserialize(raw,Reports.ReportTypeMetadata.class);for(Reports.ReportTypeColumnCategory c:b.getCategories()){for(Reports.ReportTypeColumn f:c.getColumns().values()){if(f.getFilterValues()!=null && !f.getFilterValues().isEmpty()){} }}System.debug('B6JSON|'+JSON.serialize(b));`
			program, e := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: p[0]})
			if e != nil {
				t.Fatal(e)
			}
			m := New(nil)
			if p[1] == "seealldata" {
				m.EnableTestContext()
				m.SetTestSeeAllData(true)
			}
			out, e := m.Execute(program)
			if e != nil {
				t.Fatal(e)
			}
			if len(out.Debug) != 1 || !strings.HasPrefix(out.Debug[0], "B6JSON|") {
				t.Fatalf("unexpected debug result")
			}
			var got, want any
			if e = json.Unmarshal([]byte(strings.TrimPrefix(out.Debug[0], "B6JSON|")), &got); e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(after, &want); e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("normalized catalog differs from captured native output for %s", name)
			}
		})
	}
	if rows != 12 {
		t.Fatalf("got %d native rows", rows)
	}
}
