package apextest

import (
	"crypto/sha256"
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

// Replay the follow-up captures: CustomNotification validation
// precedence, targetPageRef and Id shapes, an org-only notification type, and
// ObjectPermissions.PermissionsViewAllFields access at source API 62 and 67.
// Org-only rows name a type that no project metadata declares; their local
// answer is pinned in orgOnlyLocal.
func TestCase289FollowupsNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/case289_followups"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ID, Code string
		Routes   []string
	}
	if err := json.Unmarshal(raw, &cases); err != nil || len(cases) != 88 {
		t.Fatal("incomplete owned case table", err)
	}
	sources := map[string]string{}
	routes := map[string][]string{}
	for _, row := range cases {
		if sources[row.ID] != "" || row.Code == "" {
			t.Fatal("duplicate or empty case", row.ID)
		}
		sources[row.ID] = row.Code
		routes[row.ID] = row.Routes
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 345 || strings.Join(rows[0], ",") != "source_api,route,case_id,observation" {
		t.Fatalf("incomplete native matrix: rows=%d err=%v", len(rows), err)
	}
	capture, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
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
				text := strings.ReplaceAll(string(body), "62.0", api)
				captureAPI, captureText := api, text
				if api == "68.0" {
					// The API 68 recapture used the API 67 source template.
					captureAPI = "67.0"
					captureText = strings.ReplaceAll(string(body), "62.0", captureAPI)
				}
				if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s:%s=%x\n", captureAPI, filepath.ToSlash(name), sha256.Sum256([]byte(captureText)))) {
					t.Errorf("source differs from capture: %s", name)
				}
				writeFile(t, filepath.Join(root, name), text)
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
			messages := map[string]string{}
			run := Run(index, Options{SelectedClasses: []string{"Case289ObservationTest"}, NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 84 {
				t.Fatalf("project discovery: %+v", run.Summary())
			}
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
						t.Fatalf("%s: missing observation: %+v", result.MethodName, result.Problem)
					}
					messages[result.MethodName] = result.Problem.Message
				}
			}
			seen := 0
			nativeAPI := api
			if api == "68.0" {
				// The recapture found the same observations at 67 and 68.
				nativeAPI = "67.0"
			}
			for line, row := range rows[1:] {
				if row[0] != nativeAPI {
					continue
				}
				seen++
				key := row[1] + "/" + row[2]
				if sources[row[2]] == "" {
					t.Fatal("invalid native row", key)
				}
				t.Run(key, func(t *testing.T) {
					var got, want any
					if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
						t.Fatal(err)
					}
					if local, ok := localAnswers[nativeAPI+"/"+row[1]+"/"+row[2]]; ok {
						want = local
					}
					var payload string
					switch row[1] {
					case "istest":
						_, payload, _ = strings.Cut(messages[row[2]], "OBS|"+row[2]+"|")
					case "anonymous":
						marker := "OBS|" + row[2] + "|"
						source := sources[row[2]] + fmt.Sprintf("System.debug('%s'+JSON.serialize(o));", marker)
						if strings.HasPrefix(row[2], "OPV") {
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
						if reflect.DeepEqual(want, map[string]any{"returned": true}) {
							// Accepted anonymous sends retain the explicit local hosted-delivery boundary.
							want = map[string]any{"stage": "send", "exceptionType": "System.UnsupportedOperationException", "exceptionMessage": "Messaging.CustomNotification.send requires hosted notification delivery"}
						}
					case "static":
						obs, ok := want.(map[string]any)
						if !ok {
							t.Fatal("invalid static observation")
						}
						compiled := obs["compiled"] == true
						source := sources[row[2]]
						analysis := sema.AnalyzeAnonymous(index, source, api)
						_, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if !compiled {
							// Native rejects the member at API 62.0 on all four static routes.
							if !analysis.HasErrors() {
								t.Fatalf("native rejected %q; Glade accepted %s", obs["problem"], row[2])
							}
							return
						}
						if analysis.HasErrors() || compileErr != nil {
							t.Fatalf("native compiled; Glade diagnostics=%v err=%v", analysis.Diagnostics, compileErr)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						result, err := runner.execute(program)
						if err != nil {
							t.Fatal(err)
						}
						if row[2] != "ST3" {
							marker := "OBS|" + row[2] + "|"
							if len(result.Debug) != 1 || strings.TrimPrefix(result.Debug[0], marker) != obs["debug"] {
								t.Fatalf("native debug %v; Glade %v", obs["debug"], result.Debug)
							}
						}
						return
					default:
						t.Fatal("unobserved route", key)
					}
					if err := json.Unmarshal([]byte(payload), &got); err != nil {
						t.Fatalf("missing JSON: %q: %v", payload, err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("org.tsv:%d native=%s; Glade=%s", line+2, row[3], payload)
					}
				})
			}
			if seen != 172 {
				t.Fatalf("incomplete route matrix: %d", seen)
			}
		})
	}
}

// Pinned local answers where the native org holds state that project metadata
// cannot declare. Each entry names why Glade differs.
var localAnswers = func() map[string]any {
	answers := map[string]any{}
	// OT01, OT02: CustomNotificationType OrgOnlyNotice exists only in the org.
	// A SOQL lookup finds no local row, and a literal Id is indistinguishable
	// from an unknown Id, so the send reports an invalid type. Native returned.
	noRow := map[string]any{"stage": "mutation", "exceptionType": "System.QueryException", "exceptionMessage": "List has no rows for assignment to SObject"}
	unknownType := map[string]any{"stage": "send", "exceptionType": "System.HandledException", "exceptionMessage": "Invalid parameter value for: customNotifTypeId"}
	for _, api := range []string{"62.0", "67.0"} {
		answers[api+"/istest/OT01"] = noRow
		answers[api+"/anonymous/OT01"] = noRow
		answers[api+"/anonymous/OT02"] = unknownType
	}
	return answers
}()
