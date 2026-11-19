package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// Native rows in strip_omitted_defaults/org.tsv record populated fields before
// stripInaccessible runs. Cover multiple object kinds and both supported APIs.
func TestLoadDefaultsSystemFieldsNative(t *testing.T) {
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
				prefix := object + " populated="
				var fields []string
				for _, line := range strings.Split(message, "\n") {
					if strings.HasPrefix(line, prefix) {
						populated, _, _ := strings.Cut(strings.TrimPrefix(line, prefix), " ")
						fields = strings.Split(populated, ",")
					}
				}
				for _, field := range []string{"IsDeleted", "OwnerId"} {
					found := false
					for _, got := range fields {
						found = found || got == field
					}
					if !found {
						t.Errorf("%s: native populated %s; got %s", object, field, fmt.Sprint(fields))
					}
				}
			}
		})
	}
}
