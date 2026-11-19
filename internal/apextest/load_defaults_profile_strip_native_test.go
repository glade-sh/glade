package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// These fields are denied without an assigned grant; the captured synthetic
// permission set grants two while a neighboring field remains denied.
func TestLoadDefaultsProfileStripNative(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "load_defaults_permissions", "project")
	index := loadTestIndex(t, dir)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	for _, api := range []string{"62", "67"} {
		for _, method := range []string{"admin", "standard", "adminGrant", "standardGrant"} {
			t.Run(api+"/"+method, func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"ProbeV" + api}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing observation: %+v", run.Summary())
				}
				message := run.Suites[0].Cases[0].Problem.Message
				grant := strings.HasSuffix(method, "Grant")
				for _, tc := range []struct {
					object, field string
					removed       bool
				}{
					{"Contact", "DoNotCall", !grant}, {"Contact", "HasOptedOutOfEmail", true}, {"Lead", "HasOptedOutOfFax", !grant},
				} {
					foundLine, removed := false, false
					for _, line := range strings.Split(message, "\n") {
						if strings.HasPrefix(line, tc.object+"|pop=") {
							foundLine = true
							_, rest, ok := strings.Cut(line, "|remove=")
							if ok {
								list, _, _ := strings.Cut(rest, "|")
								for _, field := range strings.Split(list, ",") {
									removed = removed || field == tc.field
								}
							}
						}
					}
					if !foundLine || removed != tc.removed {
						t.Errorf("%s.%s: native removed=%t; got removed=%t (line found=%t)", tc.object, tc.field, tc.removed, removed, foundLine)
					}
				}
			})
		}
	}
}
