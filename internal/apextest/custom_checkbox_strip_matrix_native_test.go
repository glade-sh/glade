package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// Native rows compare fresh, explicit, loaded and unloaded checkboxes under
// both profiles and three permission-set grant levels.
func TestCustomCheckboxStripMatrixNative(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "custom_checkbox_strip_matrix")
	rows := readStripOmittedDefaultsTSV(t, filepath.Join(dir, "org.tsv"), "source_api,profile,grant,route,observation")
	index := loadTestIndex(t, filepath.Join(dir, "project"))
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	methods := map[string]string{"admin/none": "adminNone", "admin/read": "adminRead", "admin/edit": "adminEdit", "standard/none": "standardNone", "standard/read": "standardRead", "standard/edit": "standardEdit"}
	for _, api := range []string{"55.0", "62.0", "67.0"} {
		for group, method := range methods {
			t.Run(api+"/"+group, func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"MatrixV" + strings.TrimSuffix(api, ".0")}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing observation: %+v", run.Summary())
				}
				message := run.Suites[0].Cases[0].Problem.Message
				checked := 0
				for _, row := range rows {
					if row[0] != api || row[1]+"/"+row[2] != group {
						continue
					}
					checked++
					want := row[3] + "|" + row[4]
					if !strings.Contains(message, want+"\n") {
						t.Errorf("native observation missing: %s; got %s", want, message)
					}
				}
				if checked != 5 {
					t.Fatalf("expected five native rows, got %d", checked)
				}
			})
		}
	}
}
