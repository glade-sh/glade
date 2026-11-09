package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// The supplemental capture records native group grants and the same user
// before and after System.runAs. The original capture class returns the full
// observation; compare the permission and validation fields in the oracle.
func TestNativeFormulaPermissionGroupConformance(t *testing.T) {
	root := t.TempDir()
	copyDiskRuntimeScopeProject(t, filepath.Join("testdata", "conformance", "native_formula_permission_regex"), root)
	rows := readNativeRows(t, filepath.Join(root, "supplemental"), 6)
	index := loadTestIndex(t, root)
	for _, row := range rows {
		if row[1] != "isTest:group" {
			continue
		}
		t.Run(row[0], func(t *testing.T) {
			InvalidateRuntimeCaches()
			run := Run(index, Options{SelectedClasses: []string{"Q1Group" + strings.TrimSuffix(row[0], ".0")}, SelectedMethod: "capture", Parallelism: 1})
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
			_, message, ok := strings.Cut(message, "Q1GROUP|before=")
			if !ok {
				t.Fatalf("missing capture marker: %s", message)
			}
			before, after, ok := strings.Cut(message, "|after=")
			if !ok {
				t.Fatalf("missing after capture: %s", message)
			}
			extract := func(s string) string {
				var values []string
				for _, x := range strings.Split(s, ";") {
					if strings.HasPrefix(x, "hasBypass=") || strings.HasPrefix(x, "guarded=") || strings.HasPrefix(x, "bypassOnly=") {
						values = append(values, x)
					}
				}
				return strings.Join(values, ";")
			}
			got := "before " + extract(before) + " | after " + extract(after)
			if got != row[2] {
				t.Fatalf("native %q\nlocal  %q", row[2], got)
			}
		})
	}
}
