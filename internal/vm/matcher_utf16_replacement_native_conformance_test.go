package vm

import (
	"encoding/csv"
	"html"
	"os"
	"strings"
	"testing"
)

// NATIVE testdata/conformance/matcher_utf16_replacement/org.tsv:2-5 anonymous 62.0, 67.0
// NATIVE testdata/conformance/matcher_utf16_replacement/org.tsv:6-9 istest 62.0, 67.0
func TestMatcherUTF16ReplacementNativeConformance(t *testing.T) {
	f, err := os.Open("testdata/conformance/matcher_utf16_replacement/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 9 {
		t.Fatalf("got %d rows, want 8", len(rows)-1)
	}
	source := `String a = Pattern.compile('').matcher('a\uD83D\uDE00b').replaceAll('-');
String b = Pattern.compile('x*').matcher('\uD83D\uDE00').replaceAll('-');
String observation = String.valueOf(a.length()) + '|' + a.replace('\uD83D','H').replace('\uDE00','L') + '|' + String.valueOf(b.length());`
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			want := html.UnescapeString(row[5])
			if row[2] == "anonymous" {
				want = strings.TrimPrefix(want, "OBS=")
			} else {
				_, want, _ = strings.Cut(want, "Actual: ")
			}
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			if row[2] == "istest" {
				machine.EnableTestContext()
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			if got := machine.Globals["observation"].Text; got != want {
				t.Fatalf("got %q, native %q", got, want)
			}
		})
	}
}
