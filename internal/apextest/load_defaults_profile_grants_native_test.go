package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// load_defaults_permissions/org.tsv records describe and stripping before and
// after assigning a permission set, including ungranted neighboring fields.
func TestLoadDefaultsProfileGrantNative(t *testing.T) {
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
					granted       bool
				}{
					{"Contact", "DoNotCall", grant}, {"Contact", "HasOptedOutOfEmail", false}, {"Lead", "HasOptedOutOfFax", grant},
				} {
					want := fmt.Sprintf("%s.%s|read=%t|create=%t|default=true|pop=true", tc.object, tc.field, tc.granted, tc.granted)
					if !strings.Contains(message, want+"\n") {
						t.Errorf("native observation missing: %s; got: %s", want, message)
					}
				}
			})
		}
	}
}
