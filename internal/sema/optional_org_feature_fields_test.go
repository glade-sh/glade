package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Keep configured standard fields bound even when a project has no object
// metadata. The field availability observations live in the runtime fixtures;
// this checks that enabled fields retain their binding. Static rejection for
// disabled features needs separate native compile observations.
func TestAnalyzeOptionalStandardFeatureFields(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ object, field, feature string }{
			{"User", "DefaultCurrencyIsoCode", "MultiCurrency"},
			{"Opportunity", "SyncedQuoteId", "Quotes"},
		} {
			for _, enabled := range []bool{true} {
				t.Run(fmt.Sprintf("%s/%s.%s/%t", api, tc.object, tc.field, enabled), func(t *testing.T) {
					root := t.TempDir()
					dir := filepath.Join(root, "force-app/main/default/classes")
					if err := os.MkdirAll(dir, 0o755); err != nil {
						t.Fatal(err)
					}
					body := fmt.Sprintf("%s row=new %s(); Object value=row.%s; List<%s> selected=[SELECT %s FROM %s];", tc.object, tc.object, tc.field, tc.object, tc.field, tc.object)
					writeSemaFile(t, filepath.Join(dir, "OptionalFieldProbe.cls"), "public class OptionalFieldProbe { public static void inspect() {"+body+"} }")
					writeSemaFile(t, filepath.Join(dir, "OptionalFieldProbe.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
					writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\""+api+"\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
					features := "[]"
					if enabled {
						features = "[" + tc.feature + "]"
					}
					writeSemaFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: "+features+"\n")
					p, err := project.Load(root)
					if err != nil {
						t.Fatal(err)
					}
					s, err := schema.LoadProject(p)
					if err != nil {
						t.Fatal(err)
					}
					index := typesys.Build(p, s)
					if index.HasErrors() || len(index.Objects) != 0 {
						t.Fatalf("expected clean metadata-free project: %v objects=%d", index.Diagnostics, len(index.Objects))
					}
					named := Analyze(index)
					anonymous := AnalyzeAnonymous(index, body, api)
					if named.HasErrors() == enabled || anonymous.HasErrors() == enabled {
						t.Fatalf("enabled=%t; named=%v; anonymous=%v", enabled, named.Diagnostics, anonymous.Diagnostics)
					}
				})
			}
		}
	}
}
