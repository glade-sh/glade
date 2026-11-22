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
	runStdlibTypeDepthNativeConformance(t, "testdata/conformance/stdlib_type_depth", 12)
}

// NATIVE variable receiver and depth boundary: testdata/conformance/stdlib_type_depth_neighbors/org.tsv:2-11
func TestStdlibTypeDepthNeighborsNativeConformance(t *testing.T) {
	runStdlibTypeDepthNativeConformance(t, "testdata/conformance/stdlib_type_depth_neighbors", 10)
}

// NATIVE anonymous depth boundary and test control: testdata/conformance/stdlib_method_depth_boundary/org.tsv:2-15
func TestStdlibMethodDepthBoundaryNativeConformance(t *testing.T) {
	runStdlibTypeDepthNativeConformance(t, "testdata/conformance/stdlib_method_depth_boundary", 14)
}

// NATIVE test depth boundary: testdata/conformance/stdlib_method_depth_test_boundary/org.tsv:2-9
func TestStdlibMethodDepthTestBoundaryNativeConformance(t *testing.T) {
	runStdlibTypeDepthNativeConformance(t, "testdata/conformance/stdlib_method_depth_test_boundary", 8)
}

// NATIVE test depth limit: testdata/conformance/stdlib_method_depth_test_limit/org.tsv:2-5
func TestStdlibMethodDepthTestLimitNativeConformance(t *testing.T) {
	runStdlibTypeDepthNativeConformance(t, "testdata/conformance/stdlib_method_depth_test_limit", 4)
}

// NATIVE Comparable interface controls: testdata/conformance/stdlib_comparable_interface/org.tsv:2-13
func TestStdlibComparableInterfaceNativeConformance(t *testing.T) {
	runStdlibTypeDepthNativeConformance(t, "testdata/conformance/stdlib_comparable_interface", 12)
}

func runStdlibTypeDepthNativeConformance(t *testing.T, fixture string, expectedRows int) {
	t.Helper()
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
			classDir := "force-app/main/default/classes"
			entries, err := os.ReadDir(filepath.Join(fixture, classDir))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.IsDir() {
					t.Fatalf("unexpected directory %s", entry.Name())
				}
				b, err := os.ReadFile(filepath.Join(fixture, classDir, entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, classDir, entry.Name()), strings.ReplaceAll(string(b), "__API__", api))
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
				got := ""
				for _, suite := range run.Suites {
					for _, testCase := range suite.Cases {
						if testCase.Problem != nil {
							got = testCase.Problem.Type + ": " + testCase.Problem.Message
						}
					}
				}
				if !strings.Contains(got, want) {
					t.Fatalf("native %q; Glade %q", want, got)
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
	if len(seen) != expectedRows {
		t.Fatalf("expected %d native rows, got %d", expectedRows, len(seen))
	}
}
