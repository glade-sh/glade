package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestBatchLimitC09NativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "batch_limit_c09")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 7 {
		t.Fatalf("invalid native rows: %v", err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			api := strings.TrimSuffix(row[0], ".0")
			phase := strings.TrimPrefix(row[1], "Batch.")
			phaseCode := map[string]string{"start": "3", "execute": "4", "finish": "5"}[phase]
			if phaseCode == "" {
				t.Fatal("unknown batch phase")
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+row[0]+`"}`)
			entries, err := os.ReadDir(filepath.Join(dir, api+"-"+phase))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				body, err := os.ReadFile(filepath.Join(dir, api+"-"+phase, entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app", "main", "default", "classes", entry.Name()), string(body))
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"DootGT" + api + "C" + phaseCode + "C09"}, SelectedMethod: row[2], NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("expected selected method: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing capture assertion: %+v", result.Problem)
			}
			got, err := parseCaptureObservation(result.Problem.Message, "C09_"+api+"_"+phaseCode+"_0")
			if err != nil {
				t.Fatal(err)
			}
			id, ok := got["org"].(string)
			if !ok || !regexp.MustCompile(`^00D[A-Za-z0-9]{12}(?:[A-Za-z0-9]{3})?$`).MatchString(id) {
				t.Fatal("invalid organization ID shape")
			}
			delete(got, "org")
			var want map[string]any
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
		})
	}
}
