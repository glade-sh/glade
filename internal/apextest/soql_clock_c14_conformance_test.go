package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/sema"
)

func TestSOQLClockC14NativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "soql_clock_held_c14")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 13 {
		t.Fatalf("invalid clock rows: %v", err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1]+"/"+row[2], func(t *testing.T) {
			var want map[string]any
			if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
				t.Fatal(err)
			}
			clock := "2026-10-31T12:00:00Z" // Captured Saturday for the week-literal cells.
			if row[2] == "futureToday" {
				before, err := time.Parse(time.RFC3339Nano, want["before"].(string))
				if err != nil {
					t.Fatal(err)
				}
				clock = before.Add(-time.Minute).Format(time.RFC3339Nano)
			} else if row[2] == "formatFunction" {
				rows := want["rows"].([]any)
				first := rows[0].(map[string]any)
				zone, err := time.LoadLocation("America/Los_Angeles")
				if err != nil {
					t.Fatal(err)
				}
				instant, err := time.ParseInLocation("1/2/2006, 3:04 PM", first["CreatedDate"].(string), zone)
				if err != nil {
					t.Fatal(err)
				}
				clock = instant.UTC().Format(time.RFC3339Nano)
			}
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				if strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, "sfdx-project.json") {
					body = []byte(strings.ReplaceAll(string(body), "62.0", row[0]))
				}
				writeFile(t, filepath.Join(root, rel), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			settings := fmt.Sprintf("test:\n  clock:\n    start: '%s'\n  user:\n    timeZone: America/Los_Angeles\n    locale: en_US\n    language: en_US\n", clock)
			writeFile(t, filepath.Join(root, "glade.yml"), settings)
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
			got = normalizeCaptureIdentities(got).(map[string]any)
			if row[2] == "futureToday" {
				got, err = normalizeClockCapture(got)
				if err != nil {
					t.Fatalf("Glade clock: %v", err)
				}
				want, err = normalizeClockCapture(want)
				if err != nil {
					t.Fatalf("native clock: %v", err)
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
		})
	}
}
