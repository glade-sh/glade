package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// The captured plain Developer org's populated maps omit CurrencyIsoCode on
// all three object types, at the Apex API floor and ceiling.
func TestLoadDefaultsPlainCurrencyShapeNative(t *testing.T) {
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
				found := false
				for _, line := range strings.Split(message, "\n") {
					if strings.HasPrefix(line, object+" populated=") {
						found = true
						fields, _, _ := strings.Cut(strings.TrimPrefix(line, object+" populated="), " ")
						for _, field := range strings.Split(fields, ",") {
							if field == "CurrencyIsoCode" {
								org := orgFromIndex(index)
								def := org.Objects[object].Definition.Fields[field]
								t.Errorf("%s: native omits CurrencyIsoCode; got %s (features=%v, default=%q, picklist=%v)", object, fields, org.Features, def.DefaultValue, def.PicklistValues)
							}
						}
					}
				}
				if !found {
					t.Errorf("missing %s observation", object)
				}
			}
		})
	}
}
