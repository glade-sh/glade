package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeFormulaPermissionReviewConformance(t *testing.T) {
	base := filepath.Join("testdata", "conformance", "native_formula_permission_regex", "supplemental", "review")
	for _, row := range readNativeRows(t, base, 4) {
		if row[1] != "isTest:runAs" {
			continue
		}
		t.Run(row[0], func(t *testing.T) {
			root := t.TempDir()
			copyDiskRuntimeScopeProject(t, filepath.Join(base, "project"), root)
			index := loadTestIndex(t, root)
			version := strings.TrimSuffix(row[0], ".0")
			run := Run(index, Options{SelectedClasses: []string{"Q1NativeRoutes" + version}, SelectedMethod: "capture", Parallelism: 1, NoDiskCache: true})
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
			_, got, ok := strings.Cut(message, "Q1ROUTES|")
			if !ok {
				t.Fatalf("missing capture marker: %s", message)
			}
			if got != row[2] {
				t.Fatalf("native %q local %q", row[2], got)
			}
		})
	}
}
