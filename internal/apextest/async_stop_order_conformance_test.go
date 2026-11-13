package apextest

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// TestAsyncStopOrderNativeConformance replays native Test.stopTest ordering
// observations: futures first in submission order, then queueables FIFO, batch
// start in its submission slot after the future with execute/finish after the
// queueable, and a top-level Schedulable last.
func TestAsyncStopOrderNativeConformance(t *testing.T) {
	runAsyncStopOrderRows(t, "async_stop_order", "StopOrderTest", 13)
}

// TestAsyncStopMixNativeConformance replays the mixed batch, future and
// queueable stopTest rows. NATIVE async_stop_mix/org.tsv:2-49 istest 62.0 and
// 67.0: futures first; queueables and batches FIFO with batch start, execute
// and finish as separate queue entries appended to the tail; work enqueued in
// a phase precedes the next phase; top-level Schedulables after batch
// continuations; a throwing Queueable ends the drain.
func TestAsyncStopMixNativeConformance(t *testing.T) {
	runAsyncStopOrderRows(t, "async_stop_mix", "StopOrderMixTest", 24)
}

func runAsyncStopOrderRows(t *testing.T, fixture, class string, perAPI int) {
	dir := filepath.Join("testdata", "conformance", fixture)
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 2*perAPI+1 {
		t.Fatalf("invalid native observations: %v (%d rows)", err, len(rows))
	}
	for _, api := range []string{"62.0", "67.0"} {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
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
					body = []byte(strings.ReplaceAll(string(body), "62.0", api))
				}
				writeFile(t, filepath.Join(root, rel), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			asserted := 0
			for _, row := range rows[1:] {
				if len(row) != 6 {
					t.Fatalf("malformed native row: %v", row)
				}
				method, rowAPI, want := row[0], row[1], row[4]
				if rowAPI != api {
					continue
				}
				asserted++
				t.Run(method, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: method, RuntimeRESTAPIVersion: "68.0", NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("expected selected method: %+v", run.Summary())
					}
					result := run.Suites[0].Cases[0]
					if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
						t.Fatalf("missing observation assertion: %+v", result.Problem)
					}
					got := strings.TrimPrefix(result.Problem.Message, "Assertion Failed: ")
					if got != want {
						t.Fatalf("native=%s\nGlade =%s", want, got)
					}
				})
			}
			if asserted != perAPI {
				t.Fatalf("asserted %d rows for API %s, want %d", asserted, api, perAPI)
			}
		})
	}
}
