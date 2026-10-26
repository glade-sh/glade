package vm_test

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestTestEntrySharingNativeConformance(t *testing.T) {
	runExecutionCapture(t, "test_entry_sharing", 11)
}

func TestAsyncMixedDMLNativeConformance(t *testing.T) {
	runExecutionCapture(t, "async_mixed_dml", 6)
}

func TestAsyncMixedDMLAnonymousNativeConformance(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	fixture := filepath.Join("testdata", "conformance", "async_mixed_dml")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/anonymous_queueable_batch_finalizer", func(t *testing.T) {
			native := filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"))
			raw, err := os.ReadFile(filepath.Join(native, "anonymous.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			reader := csv.NewReader(strings.NewReader(string(raw)))
			reader.Comma = '\t'
			rows, err := reader.ReadAll()
			if err != nil || len(rows) != 2 || rows[1][0] != api || rows[1][1] != "anonymous" {
				t.Fatalf("invalid anonymous rows: %v", err)
			}
			var want map[string]int
			if err := json.Unmarshal([]byte(rows[1][4]), &want); err != nil {
				t.Fatal(err)
			}
			if want["completed"] != 4 || want["failed"] != 0 {
				t.Fatal("native async work did not complete")
			}
			facts, err := os.ReadFile(filepath.Join(native, "RUN-FACTS.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(facts), "anonymous_native_complete=true\n") {
				t.Fatal("missing native completion")
			}
			entries, err := os.ReadDir(filepath.Join(fixture, "sources"))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				source, err := os.ReadFile(filepath.Join(fixture, "sources", entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(facts), fmt.Sprintf("%s_sha256=%x\n", entry.Name(), sha256.Sum256(source))) {
					t.Fatal("capture detached from " + entry.Name())
				}
			}
			index := executionCaptureIndex(t, fixture, api)
			runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			machine := vm.New(nil)
			if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
				t.Fatal(err)
			}
			org := apextest.OrgFromIndex(index)
			org.APIVersion = api
			machine.SetOrg(&org)
			execute := func(source string) vm.Result {
				t.Helper()
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			source, err := os.ReadFile(filepath.Join(fixture, "sources", "anonymous_jobs.apex"))
			if err != nil {
				t.Fatal(err)
			}
			execute(string(source))
			if err := machine.DrainAsync(&vm.Result{}); err != nil {
				t.Fatal(err)
			}
			result := execute(`System.debug('OBS|'+JSON.serialize(new Map<String,Integer>{
                'account'=>[SELECT COUNT() FROM Account WHERE Name LIKE 'Boundary%'],
                'permissionSet'=>[SELECT COUNT() FROM PermissionSet WHERE Name LIKE 'Boundary%'],
                'completed'=>[SELECT COUNT() FROM AsyncApexJob WHERE JobType != 'BatchApexWorker' AND Status='Completed'],
                'failed'=>[SELECT COUNT() FROM AsyncApexJob WHERE NumberOfErrors > 0]
            }));`)
			for _, line := range result.Debug {
				if _, payload, ok := strings.Cut(line, "OBS|"); ok {
					var got map[string]int
					if err := json.Unmarshal([]byte(payload), &got); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%v; Glade=%v", want, got)
					}
					return
				}
			}
			t.Fatal("missing async readback")
		})
	}
}

func TestMixedDMLContextsNativeConformance(t *testing.T) {
	runExecutionCapture(t, "mixed_dml_contexts", 8)
}

func TestDMLStatementSequenceNativeConformance(t *testing.T) {
	runExecutionCapture(t, "dml_statement_sequence", 4)
}

func TestDescribePrefixIdentityNativeConformance(t *testing.T) {
	runExecutionCapture(t, "describe_prefix_identity", 2)
}

// A checkout contains files only; fixtures without metadata must reach both the
// capture hash checks and project loading without relying on an empty directory.
func TestExecutionCaptureWithoutMetadataRoutes(t *testing.T) {
	fixture := t.TempDir()
	original := filepath.Join("testdata", "conformance", "dml_statement_sequence")
	for _, dir := range []string{"sources", "native"} {
		if err := os.CopyFS(filepath.Join(fixture, dir), os.DirFS(filepath.Join(original, dir))); err != nil {
			t.Fatal(err)
		}
	}
	table, err := os.ReadFile(filepath.Join(original, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture, "cases.tsv"), table, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(fixture, "metadata")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("metadata must be absent: %v", err)
	}
	t.Run("capture_replay_anonymous_direct_test_project_test", func(t *testing.T) {
		runExecutionCaptureFixture(t, "dml_statement_sequence", 4, fixture)
	})
	for _, api := range []string{"62.0", "67.0"} {
		t.Run("project_index/"+api, func(t *testing.T) {
			executionCaptureIndex(t, fixture, api)
		})
	}
}

type executionCaptureCase struct{ id, context, class, method, source string }

// Both direct VM execution and the project test runner replay the same captured
// declarations. CPU milliseconds are host dependent; only monotonicity is compared.
func runExecutionCapture(t *testing.T, family string, count int) {
	t.Helper()
	runExecutionCaptureFixture(t, family, count, filepath.Join("testdata", "conformance", family))
}

func runExecutionCaptureFixture(t *testing.T, family string, count int, fixture string, seedSource ...string) {
	t.Helper()
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	read := func(name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	table := func(name, header string) [][]string {
		t.Helper()
		reader := csv.NewReader(strings.NewReader(read(name)))
		reader.Comma = '\t'
		records, err := reader.ReadAll()
		if err != nil || len(records) == 0 || strings.Join(records[0], "|") != header {
			t.Fatalf("invalid %s: %v", name, err)
		}
		return records[1:]
	}
	var cases []executionCaptureCase
	for _, row := range table("cases.tsv", "id|context|class|method|source") {
		cases = append(cases, executionCaptureCase{row[0], row[1], row[2], row[3], row[4]})
	}
	if len(cases) != count {
		t.Fatalf("cases=%d, want %d", len(cases), count)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			nativeRoot := filepath.Join("native", "api"+strings.TrimSuffix(api, ".0"))
			run := read(filepath.Join(nativeRoot, "RUN-FACTS.txt"))
			for _, fact := range []string{"api=" + api, "native_complete=true", "deployment_success=true", fmt.Sprintf("rows=%d", count)} {
				if !strings.Contains("\n"+run, "\n"+fact+"\n") {
					t.Fatalf("missing capture fact %s", fact)
				}
			}
			for _, dir := range []string{"sources", "metadata"} {
				err := filepath.WalkDir(filepath.Join(fixture, dir), func(path string, entry fs.DirEntry, err error) error {
					if err != nil {
						if dir == "metadata" && path == filepath.Join(fixture, dir) && errors.Is(err, fs.ErrNotExist) {
							return nil
						}
						return err
					}
					if entry.IsDir() {
						return nil
					}
					rel, err := filepath.Rel(fixture, path)
					if err != nil {
						return err
					}
					key := filepath.Base(path)
					if dir == "metadata" {
						key = filepath.ToSlash(rel)
					}
					fact := fmt.Sprintf("%s_sha256=%x", key, sha256.Sum256([]byte(read(rel))))
					if !strings.Contains("\n"+run, "\n"+fact+"\n") {
						t.Fatalf("capture detached from %s", rel)
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if !strings.Contains(run, fmt.Sprintf("cases.tsv_sha256=%x\n", sha256.Sum256([]byte(read("cases.tsv"))))) {
				t.Fatal("capture detached from case table")
			}
			rows := table(filepath.Join(nativeRoot, "org.tsv"), "api|context|id|outcome|value")
			if len(rows) != count {
				t.Fatalf("native rows=%d, want %d", len(rows), count)
			}
			index := executionCaptureIndex(t, fixture, api)
			runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := apextest.OrgFromIndex(index)
			org.APIVersion = api
			base := vm.New(nil)
			if err := apextest.RegisterCompiledProjectRuntimeForRequest(base, runtime); err != nil {
				t.Fatal(err)
			}
			for i, tc := range cases {
				row := rows[i]
				if row[0] != api || row[1] != tc.context || row[2] != tc.id {
					t.Fatalf("native row order changed: %v", row[:3])
				}
				routes := []string{"anonymous"}
				if tc.context == "istest" {
					routes = []string{"direct_test", "project_test"}
				}
				for _, route := range routes {
					t.Run(route+"/"+tc.id, func(t *testing.T) {
						var message, kind string
						var result vm.Result
						if route == "project_test" {
							report := apextest.Run(index, apextest.Options{SelectedClasses: []string{tc.class}, SelectedMethod: tc.method, NoDiskCache: true, Parallelism: 1, LimitMode: vm.LimitModeStrict})
							summary := report.Summary()
							if summary.Total != 1 {
								t.Fatalf("project selected %d cases: %+v", summary.Total, report)
							}
							for _, suite := range report.Suites {
								for _, test := range suite.Cases {
									if test.Problem != nil {
										kind = test.Problem.Type
										message = test.Problem.Message
									}
								}
							}
						} else {
							state := org.Clone()
							machine := base.CloneRuntimeFrozenShared(nil)
							machine.SetOrg(&state)
							machine.SetLimitMode(vm.LimitModeStrict)
							source := ""
							if tc.context == "anonymous" {
								if len(seedSource) > 0 {
									seed, err := vm.CompileAnonymousWithOptions(read(filepath.Join("sources", seedSource[0])), vm.CompileOptions{APIVersion: api})
									if err != nil {
										t.Fatal(err)
									}
									if _, err := machine.Execute(seed); err != nil {
										t.Fatalf("committed anonymous seed: %v", err)
									}
								}
								source = read(filepath.Join("sources", tc.source))
							}
							if tc.context == "istest" {
								source = tc.class + "." + tc.method + "();"
								machine.EnableTestContext()
								if err := apextest.RegisterTestMethodForRequest(machine, index, tc.class, tc.method); err != nil {
									t.Fatal(err)
								}
								for _, typ := range index.Types {
									if typ.Name == tc.class {
										for _, member := range typ.Members {
											isSetup := false
											for _, modifier := range member.Modifiers {
												if strings.EqualFold(strings.TrimPrefix(modifier, "@"), "TestSetup") {
													isSetup = true
												}
											}
											if !isSetup {
												continue
											}
											setup, err := vm.CompileAnonymousWithOptions(tc.class+"."+member.Name+"();", vm.CompileOptions{APIVersion: api})
											if err != nil {
												t.Fatal(err)
											}
											result, err = machine.ExecuteTestInClass(setup, tc.class)
											if err != nil {
												kind, message = executionCaptureError(err)
												break
											}
										}
									}
								}
							}
							if message == "" {
								if tc.context == "istest" && len(seedSource) > 0 {
									// Test setup and the test body are separate native
									// transactions, as in the project runner.
									machine.EnableTestContext()
								}
								if tc.context == "anonymous" {
									analysis := sema.AnalyzeAnonymous(index, source, api)
									if analysis.HasErrors() {
										if row[3] == "compile_reject" {
											return
										}
										t.Fatalf("native accepted; anonymous diagnostics=%v", analysis.Diagnostics)
									}
								}
								program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
								if err != nil {
									if row[3] == "compile_reject" {
										return
									}
									t.Fatal(err)
								}
								if tc.context == "istest" {
									result, err = machine.ExecuteTestInClass(program, tc.class)
								} else {
									result, err = machine.Execute(program)
								}
								if err != nil {
									kind, message = executionCaptureError(err)
								}
							}
							if row[3] == "compile_reject" {
								t.Fatalf("native compile rejection %s; Glade executed: %s %s", row[4], kind, message)
							}
							if tc.context == "anonymous" && message == "" {
								for _, line := range result.Debug {
									if strings.HasPrefix(line, "OBS|") {
										message = line
									}
								}
							}
						}
						var want any
						if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
							t.Fatal(err)
						}
						switch row[3] {
						case "observation":
							_, payload, ok := strings.Cut(message, "OBS|")
							if !ok {
								t.Fatalf("missing observation; native=%s; Glade=%s: %s", row[4], kind, message)
							}
							var got any
							if err := json.NewDecoder(strings.NewReader(payload)).Decode(&got); err != nil {
								t.Fatalf("invalid observation: %v: %s", err, message)
							}
							if family == "dml_statement_sequence" {
								wantRows, gotRows := executionCaptureSequence(t, want), executionCaptureSequence(t, got)
								for i := range wantRows {
									if !reflect.DeepEqual(gotRows[i], wantRows[i]) {
										t.Errorf("sample %d: native=%v; Glade=%v", i, wantRows[i], gotRows[i])
									}
								}
								return
							}
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("native=%v; Glade=%v", want, got)
							}
						case "exception":
							if kind != "" && !strings.Contains(kind, ".") {
								kind = "System." + kind
							}
							got := map[string]any{"type": kind, "message": message}
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("native=%v; Glade=%v", want, got)
							}
						default:
							t.Fatalf("unexpected native outcome %s", row[3])
						}
					})
				}
			}
		})
	}
}

func executionCaptureError(err error) (string, string) {
	var runtimeErr *vm.RuntimeError
	if errors.As(err, &runtimeErr) {
		return runtimeErr.Type, runtimeErr.ExceptionMessage()
	}
	return "error", err.Error()
}

func executionCaptureSequence(t *testing.T, value any) []any {
	t.Helper()
	rows, ok := value.([]any)
	if !ok || len(rows) != 151 {
		t.Fatalf("sequence has %T / %v rows; want 151", value, len(rows))
	}
	result := make([]any, 0, len(rows))
	previous := float64(0)
	for _, value := range rows {
		row, ok := value.([]any)
		if !ok || len(row) != 7 {
			t.Fatalf("invalid statement sample %v", value)
		}
		before, okBefore := row[5].(float64)
		after, okAfter := row[6].(float64)
		if !okBefore || !okAfter {
			t.Fatalf("invalid CPU sample %v", row)
		}
		result = append(result, []any{row[0], row[1], row[2], row[3], row[4], before >= previous && after >= before})
		previous = after
	}
	return result
}

func executionCaptureIndex(t *testing.T, fixture, api string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	write := func(name string, body []byte) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("sfdx-project.json", []byte(fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api)))
	if shape, err := os.ReadFile(filepath.Join(fixture, "project-scratch-def.json")); err == nil {
		write("config/project-scratch-def.json", shape)
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	for _, dir := range []string{"sources", "metadata"} {
		err := filepath.WalkDir(filepath.Join(fixture, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				if dir == "metadata" && path == filepath.Join(fixture, dir) && errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(filepath.Join(fixture, dir), path)
			if err != nil {
				return err
			}
			if dir == "sources" && filepath.Ext(path) != ".cls" {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			target := filepath.Join("force-app/main/default", rel)
			if dir == "sources" {
				target = filepath.Join("force-app/main/default/classes", rel)
				write(target+"-meta.xml", []byte("<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>"))
			}
			write(target, body)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, s)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native declarations accepted; Glade=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	return index
}
