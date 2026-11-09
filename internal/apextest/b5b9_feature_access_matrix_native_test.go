package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

type b5b9AccessRow struct {
	Profile, API, Object, Field string
	Read, Query, Get, Put       bool
}

func b5b9AccessRows(t *testing.T) []b5b9AccessRow {
	t.Helper()
	data, err := os.ReadFile("testdata/conformance/standard_describe_profiles/b5b9_feature_objects/full-access/matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []b5b9AccessRow
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 102 {
		t.Fatalf("captured matrix has %d rows", len(rows))
	}
	return rows
}

func b5b9AccessProject(t *testing.T, profile, api string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	name := "npc-minimal.json"
	if profile == "doc" {
		name = "docgen-minimal.json"
	}
	data, err := os.ReadFile("testdata/conformance/standard_describe_profiles/b5b9_feature_objects/" + name)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(data))
	return root
}

// full-access and access-bisect retain direct read, SOQL and dynamic operations
// at each source version. Both native routes are in the feature org.tsv.
func TestB5B9FeatureAccessMatrixStaticNativeRoutes(t *testing.T) {
	for _, row := range b5b9AccessRows(t) {
		for _, op := range []string{"read", "query"} {
			t.Run(row.Profile+"/"+row.API+"/"+row.Object+"/"+row.Field+"/"+op, func(t *testing.T) {
				source := row.Object + " record=new " + row.Object + "(); Object value=record." + row.Field + ";"
				want := row.Read
				if op == "query" {
					source = "List<" + row.Object + "> rows=[SELECT " + row.Field + " FROM " + row.Object + " LIMIT 1];"
					want = row.Query
				}
				root := b5b9AccessProject(t, row.Profile, row.API)
				path := filepath.Join(root, "force-app/main/default/classes/OwnedMatrixProbe.cls")
				writeFile(t, path, "public class OwnedMatrixProbe { public static void probe(){"+source+"} }")
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>")
				index := loadTestIndex(t, root)
				named := index.HasErrors() || sema.Analyze(index).HasErrors()
				anon := sema.AnalyzeAnonymous(index, source, row.API).HasErrors()
				if named == want || anon == want {
					t.Errorf("native accepted=%t; named rejected=%t anonymous rejected=%t", want, named, anon)
				}
			})
		}
	}
}

func TestB5B9FeatureAccessMatrixDynamicNativeRoutes(t *testing.T) {
	rows := b5b9AccessRows(t)
	for _, profile := range []string{"gift", "doc"} {
		for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
			t.Run(profile+"/"+api, func(t *testing.T) {
				var body strings.Builder
				for _, row := range rows {
					if row.Profile != profile || row.API != api {
						continue
					}
					fmt.Fprintf(&body, `{SObject r=new %s(); Boolean ok=true; try{r.get('%s');}catch(SObjectException e){ok=false;}System.assertEquals(%t,ok,'get %s.%s');`, row.Object, row.Field, row.Get, row.Object, row.Field)
					fmt.Fprintf(&body, `ok=true;try{r.put('%s',null);}catch(SObjectException e){ok=false;}System.assertEquals(%t,ok,'put %s.%s');`, row.Field, row.Put, row.Object, row.Field)
					fmt.Fprintf(&body, `ok=true;try{Database.query('SELECT %s FROM %s LIMIT 1');}catch(QueryException e){ok=false;}System.assertEquals(%t,ok,'query %s.%s');}`, row.Field, row.Object, row.Query, row.Object, row.Field)
				}
				runStandardFieldEvidenceRoutes(t, b5b9AccessProject(t, profile, api), api, body.String())
			})
		}
	}
}
