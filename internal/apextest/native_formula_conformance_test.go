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
	"github.com/glade-sh/glade/internal/vm"
)

func readNativeRows(t *testing.T, root string, want int) [][]string {
	t.Helper()
	file, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != want+1 {
		t.Fatalf("native rows=%d, error=%v; want header and %d observations", len(rows), err, want)
	}
	for _, row := range rows[1:] {
		if len(row) != 3 || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid native row: %v", row)
		}
	}
	return rows[1:]
}

func nativeAnonymousObservation(t *testing.T, root, marker, apiVersion, expression string) string {
	t.Helper()
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	program, err := vm.CompileAnonymousWithOptions("System.debug('"+marker+"'+"+expression+");", vm.CompileOptions{APIVersion: apiVersion})
	if err != nil {
		t.Fatal(err)
	}
	org := orgFromIndex(index)
	// A replayed capture assignment (test.permissionSets) also held on the
	// native anonymous route, which ran as the same administrator.
	if names := configuredTestPermissionSets(t, root); len(names) > 0 {
		assignCapturedPermissionSets(t, &org, names...)
	}
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	result, err := runner.execute(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range result.Debug {
		if strings.HasPrefix(line, marker) {
			return strings.TrimPrefix(line, marker)
		}
	}
	return ""
}

// Native rows: testdata/conformance/native_nospace_generic_decl/org.tsv.
func TestNativeNoSpaceGenericDeclarationConformance(t *testing.T) {
	root := t.TempDir()
	copyDiskRuntimeScopeProject(t, filepath.Join("testdata", "conformance", "native_nospace_generic_decl"), root)
	for _, row := range readNativeRows(t, root, 2) {
		t.Run(row[0], func(t *testing.T) {
			className := "NativeNoSpace" + strings.TrimSuffix(row[0], ".0")
			got := nativeAnonymousObservation(t, root, "NATIVE_NOSPACE|", row[0], "JSON.serialize("+className+".current())")
			var want, have any
			if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(got), &have); err != nil {
				t.Fatalf("missing complete observation: %q (%v)", got, err)
			}
			if !reflect.DeepEqual(want, have) {
				t.Fatalf("native=%s\nlocal=%s", row[2], got)
			}
		})
	}
}

// Native rows: testdata/conformance/native_formula_permission_regex/org.tsv.
// The anonymous route runs as the administrator without the custom
// permission; the isTest routes run each user through the Proof class.
func TestNativeFormulaPermissionRegexConformance(t *testing.T) {
	root := t.TempDir()
	copyDiskRuntimeScopeProject(t, filepath.Join("testdata", "conformance", "native_formula_permission_regex"), root)
	// ORG-RUN.txt: the running administrator was assigned Probe_Row_Access for
	// field access on both APIs (case196 API 67 admin FLS).
	writeCapturedAssignments(t, root, "Probe_Row_Access")
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	for _, row := range readNativeRows(t, root, 8) {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			version := strings.TrimSuffix(row[0], ".0")
			got := ""
			route, user, _ := strings.Cut(row[1], ":")
			if route == "isTest" {
				InvalidateRuntimeCaches()
				run := Run(index, Options{SelectedClasses: []string{"NativeFormulaProbe" + version + "Proof"}, SelectedMethod: user, Parallelism: 1})
				if run.Summary().Total != 1 {
					t.Fatalf("selected observation count: %+v", run.Summary())
				}
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						if c.Problem != nil {
							_, got, _ = strings.Cut(c.Problem.Message, "NATIVE_FORMULA|")
						}
					}
				}
			} else {
				got = nativeAnonymousObservation(t, root, "NATIVE_FORMULA|", row[0], "String.join(NativeFormulaProbe"+version+".current(), ';')")
			}
			wantLines, gotLines := strings.Split(row[2], ";"), strings.Split(strings.TrimSpace(got), ";")
			for i := range wantLines {
				if i >= len(gotLines) || wantLines[i] != gotLines[i] {
					have := "<missing>"
					if i < len(gotLines) {
						have = gotLines[i]
					}
					t.Errorf("native %q\n local %q", wantLines[i], have)
				}
			}
			if len(gotLines) != len(wantLines) {
				t.Errorf("observation count native=%d local=%d", len(wantLines), len(gotLines))
			}
		})
	}
}
