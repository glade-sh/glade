package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// query-boundary captures WHERE and ORDER BY in static and dynamic SOQL.
func TestB5B9FeatureQueryBoundaryNativeRoutes(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance/standard_describe_profiles/b5b9_feature_objects/query-boundary/results.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Object, Field, Clause, Route, Source string
		Status                                    int
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 16 {
		t.Fatalf("captured rows=%d", len(rows))
	}
	for _, row := range rows {
		label := row.API + "/" + row.Object + "/" + strings.Fields(row.Clause)[0] + "/" + row.Route
		t.Run(label, func(t *testing.T) {
			root := b5b9AccessProject(t, "doc", row.API)
			if row.Route == "anonymous-dynamic" {
				want := "false"
				if row.Status != 0 {
					want = "true"
				}
				body := `Boolean denied=false;try{` + row.Source + `}catch(QueryException e){denied=true;}System.assertEquals(` + want + `,denied);`
				runStandardFieldEvidenceRoutes(t, root, row.API, body)
				return
			}
			path := filepath.Join(root, "force-app/main/default/classes/OwnedQueryBoundary.cls")
			writeFile(t, path, "public class OwnedQueryBoundary {public static void probe(){"+row.Source+"}}")
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			named := index.HasErrors() || sema.Analyze(index).HasErrors()
			anon := sema.AnalyzeAnonymous(index, row.Source, row.API).HasErrors()
			rejected := row.Status != 0
			if named != rejected || anon != rejected {
				t.Errorf("native rejected=%t; named=%t anonymous=%t", rejected, named, anon)
			}
		})
	}
}
