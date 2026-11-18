package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// The captured plain Developer org explicitly omits these fields and exposes
// the two Product2 defaults. Object permission and hidden-field behavior are
// covered independently.
func TestLoadDefaultsPlainFieldShapeNative(t *testing.T) {
	index := loadTestIndex(t, filepath.Join("testdata", "conformance", "load_defaults_shape", "project"))
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			run := Run(index, Options{SelectedClasses: []string{"ShapeV" + api}, SelectedMethod: "admin", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing observation: %+v", run.Summary())
			}
			message := run.Suites[0].Cases[0].Problem.Message
			for _, want := range []string{
				"Contact.IsPriorityRecord|exists=false", "Lead.IsPriorityRecord|exists=false", "Product2.ProductClass|exists=false",
				"Product2.IsSerialized|exists=true|read=true|create=true|default=true|pop=true",
				"Product2.TransferRecordMode|exists=true|read=false|create=false|default=true|pop=true",
			} {
				if !strings.Contains(message, want+"\n") {
					t.Errorf("native observation missing: %s; got %s", want, message)
				}
			}
		})
	}
}
