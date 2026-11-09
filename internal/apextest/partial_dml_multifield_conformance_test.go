package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// partialDMLMultifieldCarries pins Glade's current output for rows that differ
// from native (key: api/route/probe-id). A carry fails when Glade changes.
var partialDMLMultifieldCarries = map[string]string{}

var partialDMLIDPattern = regexp.MustCompile(`\b(?:[A-Za-z0-9]*[0-9][A-Za-z0-9]*)\b`)

// normalizePartialDMLText replaces record ids (15 or 18 characters with a
// digit) so native and Glade messages compare by shape.
func normalizePartialDMLText(s string) string {
	return partialDMLIDPattern.ReplaceAllStringFunc(s, func(word string) string {
		if len(word) == 15 || len(word) == 18 {
			return "<ID>"
		}
		return word
	})
}

// normalizePartialDMLCase keeps everything the native row records except ids
// and run keys, which are replaced by presence flags.
func normalizePartialDMLCase(t *testing.T, raw map[string]any) map[string]any {
	t.Helper()
	out := map[string]any{}
	for _, key := range []string{"probeId", "operation", "mode", "style", "outcome"} {
		out[key] = raw[key]
	}
	present := func(v any) bool { return v != nil }
	if ids, ok := raw["postRecordIds"].([]any); ok {
		flags := []any{}
		for _, id := range ids {
			flags = append(flags, present(id))
		}
		out["postRecordIdsPresent"] = flags
	}
	if inputs, ok := raw["inputs"].([]any); ok {
		list := []any{}
		for _, item := range inputs {
			in := item.(map[string]any)
			list = append(list, map[string]any{"index": in["index"], "assignmentOrder": in["assignmentOrder"], "hasId": present(in["inputRecordId"]), "hasExternalKey": present(in["inputExternalKey"])})
		}
		out["inputs"] = list
	}
	if results, ok := raw["results"].([]any); ok {
		list := []any{}
		for _, item := range results {
			r := item.(map[string]any)
			row := map[string]any{"index": r["index"], "isSuccess": r["isSuccess"], "hasId": present(r["getId"])}
			if created, ok := r["isCreated"]; ok {
				row["isCreated"] = created
			}
			errs := []any{}
			for _, e := range r["errors"].([]any) {
				m := e.(map[string]any)
				errs = append(errs, map[string]any{"getStatusCode": m["getStatusCode"], "getMessage": normalizePartialDMLText(m["getMessage"].(string)), "getFields": m["getFields"]})
			}
			row["errors"] = errs
			list = append(list, row)
		}
		out["results"] = list
	}
	if ex, ok := raw["exception"].(map[string]any); ok {
		e := map[string]any{"typeName": ex["typeName"], "getNumDml": ex["getNumDml"]}
		if message, ok := ex["message"].(string); ok {
			e["message"] = normalizePartialDMLText(message)
		}
		if stack, ok := ex["stackTrace"]; ok {
			e["stackTrace"] = stack
		}
		rows := []any{}
		for _, item := range ex["rows"].([]any) {
			r := item.(map[string]any)
			rows = append(rows, map[string]any{"ordinal": r["ordinal"], "getDmlIndex": r["getDmlIndex"], "getDmlMessage": normalizePartialDMLText(r["getDmlMessage"].(string)), "getDmlType": r["getDmlType"], "hasDmlId": present(r["getDmlId"]), "getDmlFieldNames": r["getDmlFieldNames"]})
		}
		e["rows"] = rows
		out["exception"] = e
	}
	return out
}

func partialDMLCases(t *testing.T, payload string) map[string]map[string]any {
	t.Helper()
	var raw []string
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		t.Fatalf("invalid transport: %v", err)
	}
	cases := map[string]map[string]any{}
	for _, item := range raw {
		var c map[string]any
		if err := json.Unmarshal([]byte(item), &c); err != nil {
			t.Fatalf("invalid case: %v", err)
		}
		if c["kind"] == "case" {
			cases[c["probeId"].(string)] = normalizePartialDMLCase(t, c)
		}
	}
	return cases
}

// Native multi-field partial DML: field order, row messages, result ids and
// DmlException accessors for insert, update and both upsert forms; see
// testdata/conformance/partial_dml_multifield/ORG-RUN.txt.
func TestPartialDMLMultifieldNativeConformance(t *testing.T) {
	const family = "testdata/conformance/partial_dml_multifield"
	raw, err := os.Open(filepath.Join(family, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 97 || strings.Join(rows[0], ",") != "probe-id,api,route,outcome_or_value,exact_message_text" {
		t.Fatal("incomplete native matrix")
	}
	native := map[string]map[string]any{}
	for _, row := range rows[1:] {
		var c map[string]any
		if err := json.Unmarshal([]byte(row[3]), &c); err != nil {
			t.Fatalf("bad native row %v: %v", row[:3], err)
		}
		native[row[1]+"/"+row[2]+"/"+row[0]] = normalizePartialDMLCase(t, c)
	}
	probe, err := os.ReadFile(filepath.Join(family, "anon", "probe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		api := api
		runConformanceSubtestParallel(t, api, func(t *testing.T) {
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
			writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [CapPartial]\n")
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			compare := func(t *testing.T, route string, got map[string]map[string]any) {
				t.Helper()
				if len(got) != 16 {
					t.Fatalf("%s produced %d cases, want 16", route, len(got))
				}
				for _, prefix := range []string{"insert", "update", "upsertId", "upsertExternal"} {
					for _, mode := range []string{"default", "user"} {
						for _, style := range []string{"partial", "throw"} {
							id := prefix + "-" + mode + "-" + style
							t.Run(id, func(t *testing.T) {
								want := native[api+"/"+route+"/"+id]
								if want == nil {
									t.Fatal("native row missing")
								}
								have := got[id]
								if reflect.DeepEqual(have, want) {
									if _, ok := partialDMLMultifieldCarries[api+"/"+route+"/"+id]; ok {
										t.Fatal("now matches native; remove its carry")
									}
									return
								}
								wantJSON, _ := json.Marshal(want)
								haveJSON, _ := json.Marshal(have)
								carryKey := api + "/" + route + "/" + id
								if pinned, ok := partialDMLMultifieldCarries[carryKey]; ok && pinned == string(haveJSON) {
									t.Logf("CARRY native=%s; Glade=%s", wantJSON, haveJSON)
									return
								}
								t.Fatalf("CELL %s\nnative=%s\nGlade =%s", carryKey, wantJSON, haveJSON)
							})
						}
					}
				}
			}
			t.Run("anonymous", func(t *testing.T) {
				org := capturedFixtureOrg(t, index, api)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(string(probe), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatalf("anonymous compile: %v", err)
				}
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatalf("anonymous run: %v", err)
				}
				for _, line := range result.Debug {
					if _, payload, ok := strings.Cut(line, "SYNTH_PDML_CASES|"); ok {
						compare(t, "anonymous", partialDMLCases(t, payload))
						return
					}
				}
				t.Fatal("missing anonymous observation")
			})
			t.Run("test", func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"SynthPartialCapture"}, SelectedMethod: "capture_matrix", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing runner observation %+v", run.Summary())
				}
				_, payload, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "SYNTH_PDML_CASES|")
				if !ok {
					t.Fatalf("invalid transport: %s", run.Suites[0].Cases[0].Problem.Message)
				}
				compare(t, "test", partialDMLCases(t, payload))
			})
		})
	}
}
