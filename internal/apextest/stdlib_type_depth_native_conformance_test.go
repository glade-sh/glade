package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE type argument: testdata/conformance/stdlib_type_depth/org.tsv:2 anonymous 62.0
// NATIVE type argument: testdata/conformance/stdlib_type_depth/org.tsv:3 anonymous 67.0
// NATIVE type argument: testdata/conformance/stdlib_type_depth/org.tsv:4 istest 62.0
// NATIVE type argument: testdata/conformance/stdlib_type_depth/org.tsv:5 istest 67.0
// NATIVE method depth: testdata/conformance/stdlib_type_depth/org.tsv:6-13 anonymous and istest 62.0 and 67.0
func TestStdlibTypeDepthNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/stdlib_type_depth"
	rows := readNativeTSV(t, filepath.Join(fixture, "org.tsv"))
	cases := readNativeTSV(t, filepath.Join(fixture, "cases.tsv"))
	files := make(map[string]string)
	for _, c := range cases {
		files[c["id"]] = c["file"]
	}
	seen := make(map[string]bool)
	for _, row := range rows {
		row := row
		id, api, route := row["id"], row["api"], row["route"]
		key := id + "/" + api
		if seen[key] {
			t.Fatalf("duplicate native row %s", key)
		}
		seen[key] = true
		file, ok := files[id]
		if !ok {
			t.Fatalf("unknown native case %s", id)
		}
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			for _, name := range []string{"StdProbe", "AssignProbeTest", "DepthSafeTest", "DepthLimitTest"} {
				for _, ext := range []string{".cls", ".cls-meta.xml"} {
					b, err := os.ReadFile(filepath.Join(fixture, "force-app/main/default/classes", name+ext))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+ext), strings.ReplaceAll(string(b), "__API__", api))
				}
			}
			index := loadTestIndex(t, root)
			if route == "istest" {
				run := Run(index, Options{SelectedClasses: []string{file}, SelectedMethod: "probe", NoDiskCache: true, Parallelism: 1})
				s := run.Summary()
				if s.Total != 1 {
					t.Fatalf("test selection: %+v; %s", s, firstRunProblem(run))
				}
				if row["success"] == "Passed" {
					if s.Passed != 1 {
						t.Fatalf("native passed; Glade: %s", firstRunProblem(run))
					}
					return
				}
				if s.Passed != 0 {
					t.Fatalf("native failed: %s; Glade passed", row["outcome"])
				}
				want := strings.TrimPrefix(row["outcome"], "probe=Fail ")
				if !strings.Contains(firstRunProblem(run), want) {
					t.Fatalf("native %q; Glade %q", want, firstRunProblem(run))
				}
				return
			}
			if route != "anonymous" {
				t.Fatalf("unexpected route %s", route)
			}
			body, err := os.ReadFile(filepath.Join(fixture, file))
			if err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			result, err := runner.execute(program)
			if row["success"] == "False" {
				if err == nil || !strings.Contains(err.Error(), row["outcome"]) {
					t.Fatalf("native %q; Glade %v", row["outcome"], err)
				}
				return
			}
			if err != nil {
				t.Fatalf("native %q; Glade %v", row["outcome"], err)
			}
			if row["outcome"] != "ok" && (len(result.Debug) != 1 || result.Debug[0] != row["outcome"]) {
				t.Fatalf("native %q; Glade %v", row["outcome"], result.Debug)
			}
		})
	}
	if len(seen) != 12 {
		t.Fatalf("expected 12 native rows, got %d", len(seen))
	}
}
