package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPermissionGroupRecalculationRolesNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/psg_recalculation_roles"
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
	if len(rows) != 3 {
		t.Fatalf("native rows %d, want two", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0], func(t *testing.T) {
			root := copyPermsetMutingProject(t, filepath.Join(dir, "project"), row[0])
			meta := filepath.Join(root, "force-app/main/default/classes/PsgRecalcProbe.cls-meta.xml")
			b, err := os.ReadFile(meta)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, meta, strings.ReplaceAll(string(b), "67.0", row[0]))
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("project diagnostics: %v", index.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"PsgRecalcProbe"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("missing test result: %+v", run.Summary())
			}
			problem := run.Suites[0].Cases[0].Problem
			if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
				t.Fatalf("missing assertion transport: %+v", problem)
			}
			_, payload, ok := strings.Cut(problem.Message, "CAP|")
			if !ok {
				t.Fatalf("missing payload: %v", problem)
			}
			var got, want any
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(row[1]), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("native=%v Glade=%v", want, got)
			}
		})
	}
}
