package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// named-shapes/doc-only-access.json and doc-only-cases.json capture named
// read/put, anonymous rejection, and the API-specific named query boundary.
func TestB5B9NamedOnlyFieldStaticNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/"
	data, err := os.ReadFile(dir + "named-shapes/doc-only-access.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Route, Source string
		Status             int
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Route != "anonymous" {
			continue
		}
		t.Run(row.API+"/"+row.Source, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row.API+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			profile, err := os.ReadFile(dir + "docgen-minimal.json")
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(profile))
			path := filepath.Join(root, "force-app/main/default/classes/OwnedNamedField.cls")
			writeFile(t, path, "public class OwnedNamedField {public static void probe(){"+row.Source+"}}")
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			named := index.HasErrors() || sema.Analyze(index).HasErrors()
			anonymous := sema.AnalyzeAnonymous(index, row.Source, row.API).HasErrors()
			if named || !anonymous {
				t.Errorf("native named compiled, anonymous rejected; named rejected=%t anonymous rejected=%t", named, anonymous)
			}
		})
	}
}

func TestB5B9NamedOnlyFieldDynamicNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/"
	for _, v := range []int{62, 67} {
		api := strconv.Itoa(v) + ".0"
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			profile, err := os.ReadFile(dir + "docgen-minimal.json")
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(profile))
			anonymous, named := "", ""
			for _, pair := range []struct{ object, field string }{{"DocumentGenerationProcess", "DocGenerationBatchProcessId"}, {"DocumentTemplateSection", "DocumentTemplateClauseId"}} {
				q := `SELECT ` + pair.field + ` FROM ` + pair.object + ` LIMIT 1`
				anonymous += `{SObject r=new ` + pair.object + `();Boolean denied=false;try{r.get('` + pair.field + `');}catch(SObjectException ex){denied=true;}System.assert(denied);denied=false;try{r.put('` + pair.field + `',null);}catch(SObjectException ex){denied=true;}System.assert(denied);denied=false;try{Database.query('` + q + `');}catch(QueryException ex){denied=true;}System.assert(denied);}`
				want := "false"
				if v == 67 {
					want = "true"
				}
				named += `{SObject r=new ` + pair.object + `();System.assertEquals(null,r.get('` + pair.field + `'));r.put('` + pair.field + `',null);Boolean denied=false;try{Database.query('` + q + `');}catch(QueryException ex){denied=true;}System.assertEquals(` + want + `,denied);}`
			}
			b5b9RunShapeRoutes(t, root, api, anonymous, named)
		})
	}
}

// named-query-bisect/results.json retains deliberately failed assertion
// messages showing named static and dynamic queries accepted at APIs 63-66.
func TestB5B9NamedOnlyFieldQueryIntermediateVersions(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/"
	for _, v := range []int{63, 64, 65, 66} {
		api := strconv.Itoa(v) + ".0"
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			profile, err := os.ReadFile(dir + "docgen-minimal.json")
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(profile))
			named := `Database.query('SELECT DocGenerationBatchProcessId FROM DocumentGenerationProcess LIMIT 1');` +
				`Database.query('SELECT DocumentTemplateClauseId FROM DocumentTemplateSection LIMIT 1');` +
				`List<DocumentGenerationProcess> a=[SELECT DocGenerationBatchProcessId FROM DocumentGenerationProcess LIMIT 1];` +
				`List<DocumentTemplateSection> b=[SELECT DocumentTemplateClauseId FROM DocumentTemplateSection LIMIT 1];`
			b5b9RunShapeRoutes(t, root, api, `System.assert(true);`, named)
		})
	}
}
