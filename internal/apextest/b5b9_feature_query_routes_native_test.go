package apextest

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Count and query-locator callers replay the named and anonymous native rows.
func TestB5B9FeatureQueryRoutesNative(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/query-routes/"
	data, err := os.ReadFile(dir + "observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct{ API, Route, Result, Source string }
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("native route rows=%d", len(rows))
	}
	for _, anon := range rows {
		if anon.Route != "anonymous" {
			continue
		}
		var named *struct{ API, Route, Result, Source string }
		for i := range rows {
			if rows[i].Route == "istest" && rows[i].API == anon.API {
				named = &rows[i]
				break
			}
		}
		if named == nil {
			t.Fatalf("missing named row for %s", anon.API)
		}
		t.Run(anon.API, func(t *testing.T) {
			body, _, ok := strings.Cut(anon.Source, "System.debug('B5B9QUERYROUTES:'")
			if !ok {
				t.Fatal("native source marker missing")
			}
			literal := func(s string) string { return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'" }
			anonymous := body + "System.assertEquals(" + literal(anon.Result) + ",observation);"
			namedBody := body + "System.assertEquals(" + literal(named.Result) + ",observation);"
			b5b9RunShapeRoutes(t, b5b9AccessProject(t, "doc", anon.API), anon.API, anonymous, namedBody)
		})
	}
}
