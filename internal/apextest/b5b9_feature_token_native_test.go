package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type b5b9TokenRow struct{ API, Route, Object, Profile, Result, Source string }

// A source-67 provider passes tokens into each caller version. Native rows
// capture both the anonymous and named paths, including read-only fields.
func TestB5B9FeatureTokenNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/token-overloads/"
	data, err := os.ReadFile(dir + "fulltoken-observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []b5b9TokenRow
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 60 {
		t.Fatalf("native token rows=%d", len(rows))
	}
	provider, err := os.ReadFile(dir + "OwnedFullTokenSource.cls")
	if err != nil {
		t.Fatal(err)
	}
	for _, anon := range rows {
		if anon.Route != "anonymous" {
			continue
		}
		var named *b5b9TokenRow
		for i := range rows {
			if rows[i].Route == "istest" && rows[i].API == anon.API && rows[i].Object == anon.Object && rows[i].Profile == anon.Profile {
				named = &rows[i]
				break
			}
		}
		if named == nil {
			t.Fatalf("missing named token row for %+v", anon)
		}
		t.Run(anon.Profile+"/"+anon.API+"/"+anon.Object, func(t *testing.T) {
			root := b5b9AccessProject(t, anon.Profile, anon.API)
			path := filepath.Join(root, "force-app/main/default/classes/OwnedFullTokenSource.cls")
			writeFile(t, path, string(provider))
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
			body, _, ok := strings.Cut(anon.Source, "System.debug('B5B9FULLTOKEN:'")
			if !ok {
				t.Fatal("native token source marker missing")
			}
			literal := func(s string) string { return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'" }
			anonymous := body + "System.assertEquals(" + literal(anon.Result) + ",observation);"
			namedBody := body + "System.assertEquals(" + literal(named.Result) + ",observation);"
			b5b9RunShapeRoutes(t, root, anon.API, anonymous, namedBody)
		})
	}
}
