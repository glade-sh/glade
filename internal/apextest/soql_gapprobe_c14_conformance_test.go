package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// Native rows captured 2026-11-06 for week literals by locale, case-only ORDER
// BY ties and ORDER BY NULLS placement. See testdata/conformance/soql_gapprobe_c14/ORG-RUN.txt.
func TestSOQLGapProbeC14NativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "soql_gapprobe_c14")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 9 {
		t.Fatalf("invalid native observations: %v", err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1]+"/"+row[2], func(t *testing.T) {
			var want map[string]any
			if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			classes := filepath.Join("force-app", "main", "default", "classes")
			for _, rel := range []string{"sfdx-project.json", filepath.Join(classes, row[1]+".cls"), filepath.Join(classes, row[1]+".cls-meta.xml")} {
				body, err := os.ReadFile(filepath.Join(project, rel))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, rel), string(body))
			}
			// The capture ran on Friday 2026-11-06; the default user was en_US, America/Los_Angeles.
			writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  clock:\n    start: '2026-11-06T12:00:00Z'\n  user:\n    timeZone: America/Los_Angeles\n    locale: en_US\n    language: en_US\n")
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{row[1]}, SelectedMethod: row[2], NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("expected selected method: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing capture assertion: %+v", result.Problem)
			}
			got, err := parseCaptureObservation(result.Problem.Message, row[3])
			if err != nil {
				t.Fatal(err)
			}
			switch row[2] {
			case "ties":
				// Native order for case-only ties without a further key is unspecified;
				// only the keyed order is a stable observation.
				if got["ascIdDesc"] != want["ascIdDesc"] {
					t.Fatalf("native=%v; Glade=%v", want["ascIdDesc"], got["ascIdDesc"])
				}
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
		})
	}
}
