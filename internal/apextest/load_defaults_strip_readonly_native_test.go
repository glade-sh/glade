package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// The native rows for these objects record that the constructor-populated
// deletion field is removed by CREATABLE stripping for an administrator.
func TestLoadDefaultsStripReadonlyNative(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "strip_omitted_defaults")
	index := loadTestIndex(t, filepath.Join(dir, "project"))
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			run := Run(index, Options{SelectedClasses: []string{"StripDefaultProbe" + api}, SelectedMethod: "loadDefaultsCreatableAdmin", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing observation: %+v", run.Summary())
			}
			message := run.Suites[0].Cases[0].Problem.Message
			for _, object := range []string{"Account", "Lead", "StripProbe__c"} {
				var removed []string
				for _, line := range strings.Split(message, "\n") {
					if strings.HasPrefix(line, object+" populated=") {
						_, fields, found := strings.Cut(line, " removed=")
						if found {
							removed = strings.Split(fields, ",")
						}
					}
				}
				found := false
				for _, field := range removed {
					found = found || field == "IsDeleted"
				}
				if !found {
					t.Errorf("%s: native removed IsDeleted; got %v", object, removed)
				}
			}
		})
	}
}
