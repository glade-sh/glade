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

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPendingNotificationPermissionsNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/pending_notification_permissions"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ID, Code string
		Routes   []string
	}
	if err := json.Unmarshal(raw, &cases); err != nil || len(cases) != 29 {
		t.Fatal("incomplete owned case table", err)
	}
	sources := map[string]string{}
	for _, row := range cases {
		if sources[row.ID] != "" || row.Code == "" {
			t.Fatal("duplicate or empty case", row.ID)
		}
		sources[row.ID] = row.Code
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 109 || strings.Join(rows[0], ",") != "source_api,route,case_id,observation" {
		t.Fatalf("incomplete native matrix: rows=%d err=%v", len(rows), err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native setup compiled: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, phase := range []string{"first", "repeat"} {
				t.Run(phase, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"PendingRowsObservationTest"}, Parallelism: 1})
					if run.Summary().Total != 29 {
						t.Fatalf("project discovery: %+v", run.Summary())
					}
					messages := map[string]string{}
					for _, suite := range run.Suites {
						for _, result := range suite.Cases {
							if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("%s: missing observation: %+v", result.MethodName, result.Problem)
							}
							messages[result.MethodName] = result.Problem.Message
						}
					}
					seen := map[string]bool{}
					for line, row := range rows[1:] {
						if row[0] != api {
							continue
						}
						key := row[1] + "/" + row[2]
						if seen[key] || sources[row[2]] == "" {
							t.Fatal("invalid native row", key)
						}
						seen[key] = true
						t.Run(key, func(t *testing.T) {
							var payload string
							marker := "OBS|" + row[2] + "|"
							if row[1] == "istest" {
								_, payload, _ = strings.Cut(messages[row[2]], marker)
							} else if row[1] == "anonymous" {
								source := sources[row[2]] + fmt.Sprintf("System.debug('%s'+JSON.serialize(o));", marker)
								if strings.HasPrefix(row[2], "OP_") {
									source = "Savepoint point=Database.setSavepoint();\n" + sources[row[2]] +
										fmt.Sprintf("Database.rollback(point);System.debug('%s'+JSON.serialize(o));", marker)
								}
								if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
									t.Fatal(analysis.Diagnostics)
								}
								program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								result, err := runner.execute(program)
								if err != nil || len(result.Debug) != 1 {
									t.Fatalf("debug=%v err=%v", result.Debug, err)
								}
								payload = strings.TrimPrefix(result.Debug[0], marker)
							} else {
								t.Fatal("unobserved route", key)
							}
							var got, want any
							if err := json.Unmarshal([]byte(payload), &got); err != nil {
								t.Fatalf("missing JSON: %q: %v", payload, err)
							}
							if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
								t.Fatal(err)
							}
							if row[1] == "anonymous" && (row[2] == "NV007" || row[2] == "NV010") {
								if !reflect.DeepEqual(want, map[string]any{"returned": true}) {
									t.Fatal("native overlong text acceptance changed")
								}
								// Accepted sends retain the explicit local hosted-delivery boundary.
								want = map[string]any{"stage": "send", "exceptionType": "System.UnsupportedOperationException", "exceptionMessage": "Messaging.CustomNotification.send requires hosted notification delivery"}
							}
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("org.tsv:%d native=%s; Glade=%s", line+2, row[3], payload)
							}
						})
					}
					if len(seen) != 54 {
						t.Fatalf("incomplete route matrix: %d", len(seen))
					}
				})
			}
		})
	}
}
