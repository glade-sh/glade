package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// API 62 and 67 capture loaded values and populated membership alongside the
// false-overload control, for both profiles and the marketing-user flag.
func TestLoadDefaultsValuesNative(t *testing.T) {
	index := loadTestIndex(t, filepath.Join("testdata", "conformance", "load_defaults_values", "project"))
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	for _, api := range []string{"62", "67"} {
		for _, method := range []string{"admin", "adminMarketing", "standard"} {
			t.Run(api+"/"+method, func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"ValueV" + api}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing observation: %+v", run.Summary())
				}
				message := run.Suites[0].Cases[0].Problem.Message
				for _, want := range []string{
					`Product2.IsSerialized|loaded=false|loadedPop=true|blank=null|blankPop=false`,
					`Product2.TransferRecordMode|loaded="SendAndReceive"|loadedPop=true|blank=null|blankPop=false`,
					`Campaign.ExpectedResponse|loaded=0.00|loadedPop=true|blank=null|blankPop=false`,
					`Campaign.NumberSent|loaded=0|loadedPop=true|blank=null|blankPop=false`,
				} {
					if !strings.Contains(message, want+"\n") {
						t.Errorf("native observation missing: %s; got %s", want, message)
					}
				}
			})
		}
	}
}
