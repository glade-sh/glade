package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func unescapeJunctionTSV(s string) string {
	r := strings.NewReplacer(`\\`, "\x00", `\t`, "\t", `\r`, "\r", `\n`, "\n")
	return strings.ReplaceAll(r.Replace(s), "\x00", `\`)
}

// Native anonymous-route (administrator) rows of the two-parent
// ControlledByParent junction capture: with-sharing and USER_MODE select
// counts, UserRecordAccess rows and the ten USER_MODE mutations; see
// testdata/conformance/junction_anonymous/ORG-RUN.txt.
func TestJunctionAnonymousNativeConformance(t *testing.T) {
	const family = "testdata/conformance/junction_anonymous"
	file, err := os.Open(filepath.Join(family, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	probeBytes, err := os.ReadFile(filepath.Join(family, "anon", "probe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		runConformanceSubtestParallel(t, api, func(t *testing.T) {
			wantSelects := map[string]int{}
			var wantAccess []string
			var wantMutations []map[string]any
			for _, row := range rows[1:] {
				if row[2] != api || row[3] != "anonymous" {
					continue
				}
				var native map[string]any
				if err := json.Unmarshal([]byte(unescapeJunctionTSV(row[8])), &native); err != nil {
					t.Fatalf("bad row %s/%s: %v", row[5], row[4], err)
				}
				switch row[5] {
				case "selectSummary":
					wantSelects[native["probe"].(string)] = int(native["returnedRows"].(float64))
				case "userRecordAccessRow":
					f := native["fullRow"].(map[string]any)
					wantAccess = append(wantAccess, strings.Join([]string{f["RecordId"].(string)[:3], boolText(f["HasReadAccess"]), boolText(f["HasEditAccess"]), boolText(f["HasDeleteAccess"]), boolText(f["HasTransferAccess"]), boolText(f["HasAllAccess"]), f["MaxAccessLevel"].(string)}, ","))
				case "mutation":
					m := map[string]any{"operation": native["operation"], "allOrNone": native["allOrNone"], "outcome": native["outcome"]}
					errs := []any{}
					if native["outcome"] == "SaveResult" {
						m["success"] = native["isSuccess"]
						for _, e := range native["getErrors"].([]any) {
							e := e.(map[string]any)
							errs = append(errs, map[string]any{"status": e["getStatusCode"], "message": e["getMessage"], "fields": e["getFields"]})
						}
					} else {
						ex := native["exception"].(map[string]any)
						m["message"], m["numDml"] = ex["getMessage"], ex["getNumDml"]
						for _, e := range ex["errors"].([]any) {
							e := e.(map[string]any)
							errs = append(errs, map[string]any{"index": e["getDmlIndex"], "type": e["getDmlType"], "message": e["getDmlMessage"], "fields": e["getDmlFieldNames"], "hasId": e["getDmlId"] != nil})
						}
					}
					m["errors"] = errs
					wantMutations = append(wantMutations, m)
				}
			}
			if len(wantSelects) != 6 || len(wantAccess) != 5 || len(wantMutations) != 10 {
				t.Fatalf("native rows selects=%d access=%d mutations=%d, want 6/5/10", len(wantSelects), len(wantAccess), len(wantMutations))
			}
			root := t.TempDir()
			project := filepath.Join(family, "project")
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
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := capturedFixtureOrg(t, index, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			program, err := vm.CompileAnonymousWithOptions(string(probeBytes), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatalf("anonymous compile: %v", err)
			}
			result, err := runner.newMachine().Execute(program)
			if err != nil {
				t.Fatalf("anonymous run: %v", err)
			}
			var observed []map[string]any
			for _, line := range result.Debug {
				if _, payload, ok := strings.Cut(line, "JUNCTION_ANON|"); ok {
					if err := json.Unmarshal([]byte(payload), &observed); err != nil {
						t.Fatalf("bad payload: %v", err)
					}
				}
			}
			if observed == nil {
				t.Fatal("missing anonymous observation")
			}
			gotSelects := map[string]int{}
			var gotAccess []string
			var gotMutations []map[string]any
			for _, o := range observed {
				switch o["kind"] {
				case "select":
					gotSelects[o["probe"].(string)] = int(o["rows"].(float64))
				case "access":
					gotAccess = append(gotAccess, strings.Join([]string{o["record"].(string), boolText(o["read"]), boolText(o["edit"]), boolText(o["delete"]), boolText(o["transfer"]), boolText(o["all"]), o["level"].(string)}, ","))
				case "mutation":
					m := map[string]any{"operation": o["operation"], "allOrNone": o["allOrNone"], "outcome": o["outcome"]}
					if o["outcome"] == "SaveResult" {
						m["success"] = o["success"]
					} else {
						m["message"], m["numDml"] = o["message"], o["numDml"]
					}
					m["errors"] = o["errors"]
					gotMutations = append(gotMutations, m)
				}
			}
			t.Run("selects", func(t *testing.T) {
				if !reflect.DeepEqual(gotSelects, wantSelects) {
					t.Errorf("select counts native=%v glade=%v", wantSelects, gotSelects)
				}
			})
			t.Run("userRecordAccess", func(t *testing.T) {
				sort.Strings(gotAccess)
				sort.Strings(wantAccess)
				if !reflect.DeepEqual(gotAccess, wantAccess) {
					t.Errorf("UserRecordAccess native=%v glade=%v", wantAccess, gotAccess)
				}
			})
			t.Run("mutations", func(t *testing.T) {
				if len(gotMutations) != len(wantMutations) {
					t.Fatalf("mutations native=%d glade=%d", len(wantMutations), len(gotMutations))
				}
				for i := range wantMutations {
					want, _ := json.Marshal(wantMutations[i])
					got, _ := json.Marshal(gotMutations[i])
					var w, g any
					_ = json.Unmarshal(want, &w)
					_ = json.Unmarshal(got, &g)
					if !reflect.DeepEqual(w, g) {
						t.Errorf("mutation %d\n native=%s\n glade =%s", i, want, got)
					}
				}
			})
		})
	}
}

func boolText(v any) string {
	if b, ok := v.(bool); ok && b {
		return "true"
	}
	return "false"
}
