package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The native org reports its own current REST version in record URLs; Glade
// reports its configured one. The number text is what these rows pin.
var doubleJSONRESTVersion = regexp.MustCompile(`/services/data/v[0-9]+\.[0-9]+/`)

func normalizeDoubleJSONRow(value string) string {
	return doubleJSONRESTVersion.ReplaceAllString(value, "/services/data/vNN/")
}

// Native rows: one fresh scratch org, anonymous Apex and isTest at API 62.0,
// 67.0 and 68.0. All six runs wrote identical text for every key, so one table
// covers the three versions and both routes. The probes are the native
// anonymous blocks; here each runs as a test method that reports its rows.
func TestDoubleDecimalJSONSerializeNativeConformance(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "conformance", "double_json_serialization")
	want := map[string]string{}
	var probes []string
	// org.tsv and probes/ are the first native run; org-extra.tsv and
	// probes-extra/ are a second run on the same org for non-finite, boundary
	// and Decimal.format rows.
	for _, set := range [][2]string{{"org.tsv", "probes"}, {"org-extra.tsv", "probes-extra"}} {
		raw, err := os.ReadFile(filepath.Join(fixture, set[0]))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
			if i == 0 {
				if line != "key\tvalue" {
					t.Fatalf("unexpected header %q", line)
				}
				continue
			}
			key, value, ok := strings.Cut(line, "\t")
			if !ok {
				t.Fatalf("invalid native row %q", line)
			}
			want[key] = normalizeDoubleJSONRow(value)
		}
		files, err := filepath.Glob(filepath.Join(fixture, set[1], "*.apex"))
		if err != nil || len(files) == 0 {
			t.Fatalf("missing probes in %s: %v", set[1], err)
		}
		sort.Strings(files)
		probes = append(probes, files...)
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(filepath.Join(fixture, "project"), path)
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "67.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"DblBox", "DecBox", "LngBox", "IntBox", "ObjBox"} {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"),
					`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [JFPS]\n")
			var source strings.Builder
			source.WriteString("@IsTest\nprivate class DoubleJSONProbe {\n")
			for i, probe := range probes {
				body, err := os.ReadFile(probe)
				if err != nil {
					t.Fatal(err)
				}
				source.WriteString("  @IsTest static void m" + fmt.Sprintf("%03d", i) + "() {\n    List<String> RS = new List<String>();\n")
				source.Write(body)
				source.WriteString("    System.assert(false, 'OBS~~' + String.join(RS, '\\nOBS~~'));\n  }\n")
			}
			source.WriteString("}\n")
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/DoubleJSONProbe.cls"), source.String())
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/DoubleJSONProbe.cls-meta.xml"),
				`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			results := Run(index, Options{SelectedClasses: []string{"DoubleJSONProbe"}, NoDiskCache: true, Parallelism: 1})
			if len(results.Suites) != 1 || len(results.Suites[0].Cases) != len(probes) {
				t.Fatalf("probe methods not run: %+v", results.Summary())
			}
			got := map[string]string{}
			for _, result := range results.Suites[0].Cases {
				if result.Problem == nil {
					t.Fatalf("%s: missing row transport", result.MethodName)
				}
				_, rows, found := strings.Cut(result.Problem.Message, "OBS~~")
				if !found {
					t.Fatalf("%s: %s", result.MethodName, result.Problem.Message)
				}
				for _, row := range strings.Split(rows, "\nOBS~~") {
					key, value, _ := strings.Cut(row, "~~")
					got[key] = normalizeDoubleJSONRow(value)
				}
			}
			var mismatches []string
			for key, expected := range want {
				if actual, ok := got[key]; !ok {
					mismatches = append(mismatches, key+"\n   native: "+expected+"\n   local:  <missing>")
				} else if actual != expected {
					mismatches = append(mismatches, key+"\n   native: "+expected+"\n   local:  "+actual)
				}
			}
			for key := range got {
				if _, ok := want[key]; !ok {
					mismatches = append(mismatches, key+"\n   native: <no row>")
				}
			}
			if len(mismatches) != 0 {
				sort.Strings(mismatches)
				limit := len(mismatches)
				if limit > 120 {
					limit = 120
				}
				t.Fatalf("%d of %d native rows differ:\n%s", len(mismatches), len(want), strings.Join(mismatches[:limit], "\n"))
			}
		})
	}
}
