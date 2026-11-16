package apextest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

const duplicateRuntimeFixture = "testdata/conformance/duplicate_runtime_behavior"

// NATIVE duplicate rule order, gate, options, Exact length, mapping, no-rule and
// FieldDiff rows testdata/conformance/duplicate_runtime_behavior/org.tsv
// Rows are keyed by api, route and Apex method. Each payload is the JSON the
// native run transported in a failed assertion; record and object ids were
// masked when the rows were committed.
type duplicateRuntimeRows map[string]string

var duplicateRuntimeIDs = regexp.MustCompile(`\b(01I|a0[0-9a-zA-Z])[0-9A-Za-z]{12}(?:[0-9A-Za-z]{3})?\b`)

func readDuplicateRuntimeRows(t *testing.T) duplicateRuntimeRows {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(duplicateRuntimeFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if lines[0] != "api\troute\tid\tvalue" {
		t.Fatal("invalid native row header")
	}
	rows := duplicateRuntimeRows{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 {
			t.Fatalf("invalid native row: %q", line)
		}
		key := strings.Join(cells[:3], "/")
		if _, dup := rows[key]; dup {
			t.Fatalf("duplicate native row %s", key)
		}
		rows[key] = cells[3]
	}
	return rows
}

func duplicateRuntimeKeys(rows duplicateRuntimeRows) []string {
	seen := map[string]bool{}
	for key := range rows {
		parts := strings.Split(key, "/")
		if parts[1] == "anonymous" && !strings.HasPrefix(parts[2], "REP") {
			seen[parts[2]] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

func duplicateRuntimeProject(t *testing.T, api string, keys []string) string {
	t.Helper()
	root := t.TempDir()
	base := filepath.Join(root, "force-app/main/default")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for _, name := range []string{"objects", "matchingRules", "duplicateRules", "permissionsets"} {
		if err := os.CopyFS(filepath.Join(base, name), os.DirFS(filepath.Join(duplicateRuntimeFixture, name))); err != nil {
			t.Fatal(err)
		}
	}
	writeCapturedAssignments(t, root, "DupAccess")
	probe, err := os.ReadFile(filepath.Join(duplicateRuntimeFixture, "DupProbe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	var test strings.Builder
	test.WriteString("@IsTest private class DupProbeTest {\n")
	for _, key := range keys {
		fmt.Fprintf(&test, "@IsTest static void %s() { DupProbe.emit('%s'); }\n", key, key)
	}
	test.WriteString("}")
	for name, source := range map[string]string{"DupProbe": string(probe), "DupProbeTest": test.String()} {
		writeFile(t, filepath.Join(base, "classes", name+".cls"), source)
		writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
	}
	return root
}

// normalizeDuplicateRuntime masks platform ids (15 or 18 characters) and nothing
// else. An empty custom-object result reports the object's 01I id as its
// match-level entity natively (ENT rows), so the id mask is all that is needed;
// object names in messages and entity types are compared as they are.
func normalizeDuplicateRuntime(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			out[key] = normalizeDuplicateRuntime(item)
		}
		if _, ok := out["kp"].(string); ok && out["raw"] != nil {
			// ENT rows list the object's key prefix, which the org allocates in
			// creation order like an id; the comparisons against it stay.
			out["kp"] = "<kp>"
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeDuplicateRuntime(item)
		}
		return out
	case string:
		return duplicateRuntimeIDs.ReplaceAllString(v, "<id>")
	}
	return value
}

func compareDuplicateRuntime(t *testing.T, key, want, got string) {
	t.Helper()
	var wantValue, gotValue any
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("%s: native row is not JSON: %v", key, err)
	}
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("%s: Glade observation is not JSON: %v\n%s", key, err, got)
	}
	wantValue, gotValue = normalizeDuplicateRuntime(wantValue), normalizeDuplicateRuntime(gotValue)
	for _, member := range duplicateRuntimeFuzzyScoring[key] {
		// The fuzzy scorer decides confidence and membership for these members;
		// only the field differences of the records both sides return are pinned.
		compareDuplicateRuntimeDiffs(t, key+"."+member, wantValue, gotValue, member)
		deleteDuplicateRuntimePath(wantValue, member)
		deleteDuplicateRuntimePath(gotValue, member)
	}
	if !reflect.DeepEqual(wantValue, gotValue) {
		wantJSON, _ := json.Marshal(wantValue)
		gotJSON, _ := json.Marshal(gotValue)
		t.Fatalf("%s differs\nnative: %s\nglade:  %s", key, wantJSON, gotJSON)
	}
}

// duplicateRuntimeFuzzyScoring lists members whose fuzzy confidence or member
// set is not asserted (owned by the fuzzy scorer, not by this fixture).
var duplicateRuntimeFuzzyScoring = map[string][]string{"FDIFF2": {"fuzzyFirst", "queryHasStoredNull"}}

func duplicateRuntimeDiffsByName(value any) map[string]any {
	out := map[string]any{}
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if name, ok := v["n"].(string); ok {
				out[name] = v["fd"]
			}
			for _, item := range v {
				walk(item)
			}
		case []any:
			for _, item := range v {
				walk(item)
			}
		}
	}
	walk(value)
	return out
}

func compareDuplicateRuntimeDiffs(t *testing.T, label string, want, got any, member string) {
	t.Helper()
	wantMember, _ := want.(map[string]any)[member]
	gotMember, _ := got.(map[string]any)[member]
	wantDiffs, gotDiffs := duplicateRuntimeDiffsByName(wantMember), duplicateRuntimeDiffsByName(gotMember)
	shared := 0
	for name, diff := range wantDiffs {
		other, ok := gotDiffs[name]
		if !ok {
			continue
		}
		shared++
		if !reflect.DeepEqual(diff, other) {
			t.Fatalf("%s field differences for %s: native %v, glade %v", label, name, diff, other)
		}
	}
	if shared == 0 {
		t.Fatalf("%s: no shared matched record to compare field differences", label)
	}
}

func deleteDuplicateRuntimePath(value any, path string) {
	if m, ok := value.(map[string]any); ok {
		delete(m, path)
	}
}

func duplicateRuntimeMessage(message, key string) (string, bool) {
	_, payload, ok := strings.Cut(message, "DUPOBS|"+key+"|")
	return payload, ok
}

func TestDuplicateRuntimeBehaviorNativeRoutes(t *testing.T) {
	// The native rows ran with the standard duplicate rules active.
	t.Setenv("GLADE_STANDARD_DUPLICATE_RULES", "on")
	rows := readDuplicateRuntimeRows(t)
	keys := duplicateRuntimeKeys(rows)
	if len(keys) == 0 {
		t.Fatal("no native rows")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := loadTestIndex(t, duplicateRuntimeProject(t, api, keys))
			org := capturedFixtureOrg(t, index, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, key := range keys {
				t.Run("anonymous/"+key, func(t *testing.T) {
					want, ok := rows[api+"/anonymous/"+key]
					if !ok {
						t.Fatal("missing native row")
					}
					program, err := vm.CompileAnonymousWithOptions("DupProbe.emit('"+key+"');", vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = runner.newMachine().Execute(program)
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("missing Apex observation: %v", err)
					}
					got, ok := duplicateRuntimeMessage(fault.ExceptionMessage(), key)
					if !ok {
						t.Fatalf("missing observation marker: %s", fault.ExceptionMessage())
					}
					compareDuplicateRuntime(t, key, want, got)
				})
			}
			options := Options{SelectedClasses: []string{"DupProbeTest"}, NoDiskCache: true, Parallelism: 1}
			selected := Discover(index, options)
			if len(selected) != len(keys) {
				t.Fatalf("discovered %d of %d methods", len(selected), len(keys))
			}
			report := RunCasesContext(context.Background(), index, options, selected)
			count := 0
			for _, suite := range report.Suites {
				for _, result := range suite.Cases {
					count++
					t.Run("istest/"+result.MethodName, func(t *testing.T) {
						want, ok := rows[api+"/istest/"+result.MethodName]
						if !ok {
							t.Fatal("missing native row")
						}
						if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
							t.Fatalf("missing Apex observation: %+v", result.Problem)
						}
						got, ok := duplicateRuntimeMessage(result.Problem.Message, result.MethodName)
						if !ok {
							t.Fatalf("missing observation marker: %s", result.Problem.Message)
						}
						compareDuplicateRuntime(t, result.MethodName, want, got)
					})
				}
			}
			if count != len(keys) {
				t.Fatalf("ran %d of %d methods", count, len(keys))
			}
		})
	}
}

// NATIVE Allow+Report committed sets for batch peers and an update testdata/conformance/duplicate_runtime_behavior/org.tsv committed_anonymous 62.0
// Two matching records inserted by one statement, a seed with two matching
// records, and an update into a match each commit one set holding all members.
func TestDuplicateRuntimeBehaviorCommittedReports(t *testing.T) {
	t.Setenv("GLADE_STANDARD_DUPLICATE_RULES", "on")
	rows := readDuplicateRuntimeRows(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := api[:2]
			index := loadTestIndex(t, duplicateRuntimeProject(t, api, []string{"NOREG"}))
			org := capturedFixtureOrg(t, index, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			run := func(source string) string {
				t.Helper()
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := runner.newMachine()
				machine.SetOrg(&org)
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				org = *machine.Org
				for _, message := range result.Debug {
					if _, payload, ok := strings.Cut(message, "REPROBS|"+version+"|"); ok {
						return payload
					}
				}
				return ""
			}
			run("DupProbe.repWrite('" + version + "');")
			for _, id := range []string{"REPR", "REPR2"} {
				want, ok := rows[api+"/committed_anonymous/"+id]
				if !ok {
					t.Fatalf("missing native row %s", id)
				}
				got := run("DupProbe.repRead('" + version + "');")
				compareDuplicateRuntime(t, id, want, got)
			}
		})
	}
}

// NATIVE repeat update of a record inserted as an allowed duplicate, one
// transaction per step testdata/conformance/duplicate_runtime_behavior/org.tsv committed_anonymous 62.0
// RPT_* rows run a ladder in one transaction; RPTM_*, RPTN_* and RPTO_* rows run
// the same shapes with every step in its own request against persisted records.
// Natively a record inserted in the transaction runs the update gate against
// its insert-time values, so moving back to them saves; a later transaction
// compares with the stored value and blocks.
func TestDuplicateRuntimeBehaviorSeparateTransactions(t *testing.T) {
	t.Setenv("GLADE_STANDARD_DUPLICATE_RULES", "on")
	rows := readDuplicateRuntimeRows(t)
	phases := []string{"RPTM_1", "RPTM_2", "RPTM_3", "RPTM_4", "RPTM_5", "RPTN_1", "RPTN_2", "RPTN_3", "RPTN_4", "RPTO_1", "RPTO_2", "RPTO_3", "RPTO_4", "RPTO_5"}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := loadTestIndex(t, duplicateRuntimeProject(t, api, []string{"NOREG"}))
			org := capturedFixtureOrg(t, index, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, phase := range phases {
				want, ok := rows[api+"/committed_anonymous/"+phase]
				if !ok {
					t.Fatalf("missing native row %s", phase)
				}
				program, err := vm.CompileAnonymousWithOptions("DupProbe.phase('"+phase+"');", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := runner.newMachine()
				machine.SetOrg(&org)
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatalf("%s: %v", phase, err)
				}
				org = *machine.Org
				got := ""
				for _, message := range result.Debug {
					if _, payload, ok := strings.Cut(message, "RPTOBS|"+phase+"|"); ok {
						got = payload
					}
				}
				if got == "" {
					t.Fatalf("%s: missing observation", phase)
				}
				compareDuplicateRuntime(t, phase, want, got)
			}
		})
	}
}
