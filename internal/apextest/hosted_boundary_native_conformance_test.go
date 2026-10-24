package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestHostedBoundaryNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/hosted_boundaries"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Code string }
	if err := json.Unmarshal(raw, &cases); err != nil || len(cases) != 19 {
		t.Fatal("incomplete owned case table")
	}
	sources := map[string]string{}
	for _, row := range cases {
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
	if err != nil || len(rows) != 153 || strings.Join(rows[0], ",") != "source_api,route,method,observation" {
		t.Fatalf("incomplete native matrix: rows=%d error=%v", len(rows), err)
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
			// The language server uses this semantic analysis for named sources.
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native sources compiled; diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			testRun := Run(index, Options{SelectedClasses: []string{"HostedBoundaryTest"}, NoDiskCache: true, Parallelism: 1})
			if testRun.Summary().Total != 19 {
				t.Fatalf("test-runner discovery: %+v", testRun.Summary())
			}
			testMessages := map[string]string{}
			for _, suite := range testRun.Suites {
				for _, result := range suite.Cases {
					if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
						t.Errorf("%s: missing assertion envelope: %+v", result.MethodName, result.Problem)
						continue
					}
					testMessages[result.MethodName] = result.Problem.Message
				}
			}
			seen := map[string]bool{}
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				key := row[1] + "/" + row[2]
				if seen[key] || sources[row[2]] == "" {
					t.Fatalf("invalid native row: %v", row)
				}
				seen[key] = true
				t.Run(key, func(t *testing.T) {
					var want struct{ Result string }
					if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
						t.Fatal(err)
					}
					// Successful hosted delivery stays an explicit local boundary.
					if row[1] != "istest" && (row[2] == "notice_configured" || row[2] == "notice_constructor" || row[2] == "notice_page") {
						if want.Result != "returned" {
							t.Fatal("hosted native return changed")
						}
						want.Result = "System.UnsupportedOperationException|Messaging.CustomNotification.send requires hosted notification delivery"
					}
					var payload string
					if row[1] == "istest" {
						_, payload, _ = strings.Cut(testMessages[row[2]], "OBSERVED|"+row[2]+"|")
					} else {
						var source string
						switch row[1] {
						case "anonymous":
							source = `String typeId=[SELECT Id FROM CustomNotificationType WHERE DeveloperName='SyntheticNotice' LIMIT 1].Id;
String observed; try { ` + sources[row[2]] + ` observed='returned'; }
catch(Exception e) { observed=e.getTypeName()+'|'+e.getMessage(); }
System.debug('OBSERVED|'+JSON.serialize(new Map<String,Object>{'result'=>observed}));`
						case "named", "no_permission":
							source = fmt.Sprintf("System.debug('OBSERVED|'+JSON.serialize(new Map<String,Object>{'result'=>HostedBoundaryProbe.observe('%s')}));", row[2])
						default:
							t.Fatalf("unknown route: %s", row[1])
						}
						analysis := sema.AnalyzeAnonymous(index, source, api)
						if analysis.HasErrors() {
							t.Fatalf("native anonymous source compiled: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						result, err := runner.execute(program, func(machine *vm.VM) {
							if row[1] == "no_permission" {
								for _, profile := range org.Objects["Profile"].Records {
									if profile.Fields["Name"].String == "HostedBoundaryProfile" {
										machine.SetCurrentUser(storage.Record{ID: "005000000000002", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.StringValue(string(profile.ID))}})
										return
									}
								}
								t.Fatal("missing restricted profile metadata")
							}
						})
						if err != nil || len(result.Debug) != 1 {
							t.Fatalf("missing observation: debug=%v err=%v", result.Debug, err)
						}
						payload = strings.TrimPrefix(result.Debug[0], "OBSERVED|")
					}
					var got struct{ Result string }
					if err := json.Unmarshal([]byte(payload), &got); err != nil || got != want {
						t.Fatalf("native=%s; local boundary expectation=%s; Glade=%s", row[3], want.Result, payload)
					}
				})
			}
			if len(seen) != 76 {
				t.Fatalf("incomplete route matrix: %d", len(seen))
			}
		})
	}
}
