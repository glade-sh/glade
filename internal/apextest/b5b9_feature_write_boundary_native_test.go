package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// write-boundary retains native direct-assignment results on both routes.
func TestB5B9FeatureWriteBoundaryNativeRoutes(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance/standard_describe_profiles/b5b9_feature_objects/write-boundary/results.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Route, Object, Field, Source string
		Status                            int
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Route != "anonymous" {
			continue
		}
		t.Run(row.API+"/"+row.Object+"/"+row.Field, func(t *testing.T) {
			root := b5b9AccessProject(t, "doc", row.API)
			path := filepath.Join(root, "force-app/main/default/classes/OwnedWriteBoundary.cls")
			writeFile(t, path, "public class OwnedWriteBoundary {public static void probe(){"+row.Source+"}}")
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
