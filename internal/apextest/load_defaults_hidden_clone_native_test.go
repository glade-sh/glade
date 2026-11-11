package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// Native controls distinguish an original loaded record and a modified one
// from ordinary, false-overload, and cloned records.
func TestLoadDefaultsHiddenCloneNative(t *testing.T) {
	index := loadTestIndex(t, filepath.Join("testdata", "conformance", "load_defaults_hidden_routes", "project"))
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	for _, api := range []string{"62", "67"} {
		for _, profile := range []string{"admin", "standard"} {
			t.Run(api+"/"+profile, func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"HiddenV" + api}, SelectedMethod: profile, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing observation: %+v", run.Summary())
				}
				message := run.Suites[0].Cases[0].Problem.Message
				for _, tc := range []struct{ object, field string }{{"Case", "ClosedOnCreate"}, {"Task", "IsRecurrence2"}, {"Task", "IsRecurrence2Exception"}, {"Task", "IsRecurrence2Exclusion"}} {
					for _, route := range []string{"plain", "false", "true", "changed", "cloned"} {
						found, removed := false, false
						for _, line := range strings.Split(message, "\n") {
							if strings.HasPrefix(line, tc.object+"."+route+"|removed=") {
								found = true
								list := strings.TrimPrefix(line, tc.object+"."+route+"|removed=")
								for _, field := range strings.Split(list, ",") {
									removed = removed || field == tc.field
								}
							}
						}
						want := route == "true" || route == "changed"
						if !found || removed != want {
							t.Errorf("%s.%s %s: native removed=%t, got removed=%t (line found=%t)", tc.object, tc.field, route, want, removed, found)
						}
					}
				}
			})
		}
	}
}
