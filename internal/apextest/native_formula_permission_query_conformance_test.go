package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeFormulaPermissionQueryConformance(t *testing.T) {
	source := filepath.Join("testdata", "conformance", "native_formula_permission_regex")
	for _, row := range readNativeRows(t, filepath.Join(source, "supplemental", "calculated"), 4) {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			copyDiskRuntimeScopeProject(t, source, root)
			if row[1] == "anonymous:assigned" {
				writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [Probe_Bypass_Access]\n")
			}
			index := loadTestIndex(t, root)
			version := strings.TrimSuffix(row[0], ".0")
			run := Run(index, Options{SelectedClasses: []string{"Q1Query" + version}, SelectedMethod: "capture", Parallelism: 1, NoDiskCache: true})
			if run.Summary().Total != 1 {
				t.Fatalf("observation count: %+v", run.Summary())
			}
			var message string
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					if c.Problem != nil {
						message = c.Problem.Message
					}
				}
			}
			_, got, ok := strings.Cut(message, "Q1CALC|")
			if !ok {
				t.Fatalf("missing capture marker: %s", message)
			}
			if got != row[2] {
				t.Fatalf("native %q local %q", row[2], got)
			}
		})
	}
}
