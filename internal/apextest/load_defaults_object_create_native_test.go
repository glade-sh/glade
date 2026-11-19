package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// The native shape capture covers two standard profiles, the marketing-user
// flag, and a separate standard object as an access control.
func TestLoadDefaultsObjectCreateNative(t *testing.T) {
	index := loadTestIndex(t, filepath.Join("testdata", "conformance", "load_defaults_shape", "project"))
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	for _, api := range []string{"62", "67"} {
		for _, tc := range []struct{ method, campaign, pricebook, product string }{
			{"admin", "false", "true", "true"},
			{"adminMarketing", "true", "true", "true"},
			{"standard", "false", "false", "false"},
			{"standardMarketing", "false", "false", "false"},
		} {
			t.Run(api+"/"+tc.method, func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"ShapeV" + api}, SelectedMethod: tc.method, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing observation: %+v", run.Summary())
				}
				message := run.Suites[0].Cases[0].Problem.Message
				for _, want := range []string{"Campaign|create=" + tc.campaign, "Pricebook2|create=" + tc.pricebook, "Product2|create=" + tc.product} {
					if !strings.Contains(message, want+"\n") {
						t.Errorf("native observation missing: %s", want)
					}
				}
			})
		}
	}
}
