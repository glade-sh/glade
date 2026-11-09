package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// write-matrix captures static assignment acceptance for each field and route.
func TestB5B9FeatureWriteMatrixNativeRoutes(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance/standard_describe_profiles/b5b9_feature_objects/write-matrix/matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Profile, API, Object, Field, Source string
		NamedAccepted, AnonymousAccepted    bool
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 114 {
		t.Fatalf("captured rows=%d", len(rows))
	}
	for _, row := range rows {
		t.Run(row.Profile+"/"+row.API+"/"+row.Object+"/"+row.Field, func(t *testing.T) {
			root := b5b9AccessProject(t, row.Profile, row.API)
			path := filepath.Join(root, "force-app/main/default/classes/OwnedWriteMatrix.cls")
			writeFile(t, path, "@IsTest private class OwnedWriteMatrix {@IsTest static void probe(){"+row.Source+"}}")
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			named := index.HasErrors() || sema.Analyze(index).HasErrors()
			anon := sema.AnalyzeAnonymous(index, row.Source, row.API).HasErrors()
			if named == row.NamedAccepted || anon == row.AnonymousAccepted {
				t.Errorf("native accepted named=%t anonymous=%t; local rejected named=%t anonymous=%t", row.NamedAccepted, row.AnonymousAccepted, named, anon)
			}
		})
	}
}
