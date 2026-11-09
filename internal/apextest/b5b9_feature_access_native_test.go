package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// field-access/results.json and its named twins capture compile and runtime
// boundaries at both API endpoints in isolated Fundraising and DocGen orgs.
func TestB5B9FeatureVersionStaticAccessNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/"
	raw, err := os.ReadFile(dir + "field-access/results.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Profile, API, Case, Source string
		Status                     int
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Case != "static-primary" && row.Case != "static-adjacent" && row.Case != "static-soql" {
			continue
		}
		t.Run(row.API+"/"+row.Profile+"/"+row.Case, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row.API+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			profile := "npc-minimal.json"
			if row.Profile == "doc" {
				profile = "docgen-minimal.json"
			}
			data, err := os.ReadFile(dir + profile)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(data))
			path := filepath.Join(root, "force-app/main/default/classes/OwnedAccess.cls")
			writeFile(t, path, "public class OwnedAccess { public static void probe(){"+row.Source+"} }")
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			namedAnalysis := sema.Analyze(index)
			anonAnalysis := sema.AnalyzeAnonymous(index, row.Source, row.API)
			namedRejected := index.HasErrors() || namedAnalysis.HasErrors()
			anonRejected := anonAnalysis.HasErrors()
			wantRejected := row.Status != 0
			if namedRejected != wantRejected || anonRejected != wantRejected {
				t.Errorf("native rejected=%t; named=%t anonymous=%t; index=%v named=%v anonymous=%v", wantRejected, namedRejected, anonRejected, index.Diagnostics, namedAnalysis.Diagnostics, anonAnalysis.Diagnostics)
			}
		})
	}
}

func TestB5B9FeatureVersionDynamicAccessNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/"
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, object, field, profile string }{
			{"gift", "GiftTransaction", "GenerationalCohort", "npc-minimal.json"},
			{"doc", "DocumentGenerationProcess", "AdditionalMetadata", "docgen-minimal.json"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				data, err := os.ReadFile(dir + tc.profile)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(data))
				want := "false"
				if api == "62.0" {
					want = "true"
				}
				body := `SObject r=new ` + tc.object + `();Boolean denied=false;try{r.get('` + tc.field + `');}catch(SObjectException ex){denied=ex.getMessage().contains('Invalid field ` + tc.field + `');}System.assertEquals(` + want + `,denied,'get');` +
					`denied=false;try{r.put('` + tc.field + `','owned');}catch(SObjectException ex){denied=ex.getMessage().contains('Invalid field ` + tc.field + `');}System.assertEquals(` + want + `,denied,'put');r.isSet('` + tc.field + `');` +
					`denied=false;try{Database.query('SELECT ` + tc.field + ` FROM ` + tc.object + ` LIMIT 1');}catch(QueryException ex){denied=ex.getMessage().contains('No such column');}System.assertEquals(` + want + `,denied,'query');`
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}
