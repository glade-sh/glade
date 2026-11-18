package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// The native strip capture records hidden fields as removed for loaded
// defaults, but not for ordinary fresh constructors, on both profiles.
func TestLoadDefaultsHiddenRemovedNative(t *testing.T) {
	index := loadTestIndex(t, filepath.Join("testdata", "conformance", "strip_omitted_defaults", "project"))
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	for _, api := range []string{"62", "67"} {
		for _, profile := range []string{"Admin", "Standard"} {
			for _, loaded := range []bool{false, true} {
				method := "freshCreatable" + profile
				if loaded {
					method = "loadDefaultsCreatable" + profile
				}
				t.Run(api+"/"+method, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"StripDefaultProbe" + api}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("missing observation: %+v", run.Summary())
					}
					message := run.Suites[0].Cases[0].Problem.Message
					for _, tc := range []struct{ object, field string }{
						{"Case", "ClosedOnCreate"}, {"Task", "IsRecurrence2"}, {"Task", "IsRecurrence2Exception"}, {"Task", "IsRecurrence2Exclusion"},
					} {
						found, removed := false, false
						for _, line := range strings.Split(message, "\n") {
							if strings.HasPrefix(line, tc.object+" populated=") {
								found = true
								_, rest, ok := strings.Cut(line, " removed=")
								if ok {
									list, _, _ := strings.Cut(rest, " ")
									for _, name := range strings.Split(list, ",") {
										removed = removed || name == tc.field
									}
								}
							}
						}
						if !found || removed != loaded {
							t.Errorf("%s.%s: native removed=%t; got removed=%t (line found=%t)", tc.object, tc.field, loaded, removed, found)
						}
					}
				})
			}
		}
	}
}
