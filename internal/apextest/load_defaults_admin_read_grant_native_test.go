package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// The native profile inventory records a readable, non-editable field for
// both profiles. The test-route describe and stripping captures show that the
// administrator can create it, while the standard user cannot.
func TestLoadDefaultsAdminReadGrantNative(t *testing.T) {
	index := loadTestIndex(t, filepath.Join("testdata", "conformance", "load_defaults_permissions", "project"))
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	for _, api := range []string{"62", "67"} {
		for _, tc := range []struct{ method, expected string }{
			{"admin", "Case.IsEscalated|read=true|create=true|default=true|pop=true"},
			{"standard", "Case.IsEscalated|read=true|create=false|default=true|pop=true"},
		} {
			t.Run(api+"/"+tc.method, func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"ProbeV" + api}, SelectedMethod: tc.method, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing observation: %+v", run.Summary())
				}
				if got := run.Suites[0].Cases[0].Problem.Message; !strings.Contains(got, tc.expected+"\n") {
					t.Errorf("native observation missing: %s", tc.expected)
				}
			})
		}
	}
}
