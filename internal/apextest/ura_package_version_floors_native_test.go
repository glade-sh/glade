package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestURAPackageVersionFloorsNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/ura_package_version_floors"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, row := range rows[1:] {
		if row[1] != "istest" || row[2] != "package-version" {
			continue
		}
		row := row
		checked++
		t.Run(row[0], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"localprobe","sourceApiVersion":%q}`, row[0]))
			class := "VersionProbe" + strings.TrimSuffix(row[0], ".0")
			for _, suffix := range []string{".cls", ".cls-meta.xml"} {
				b, err := os.ReadFile(filepath.Join(dir, "sources", class+suffix))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+suffix), string(b))
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("project diagnostics: %v", index.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "pv", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing assertion transport: %+v", run.Summary())
			}
			msg := run.Suites[0].Cases[0].Problem.Message
			_, payload, ok := strings.Cut(msg, "LC2PV|")
			if !ok {
				t.Fatalf("missing observation: %s", msg)
			}
			var got, want any
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("native=%v Glade=%v", want, got)
			}
		})
	}
	if checked != 6 {
		t.Fatalf("named native rows=%d, want 6", checked)
	}
}
