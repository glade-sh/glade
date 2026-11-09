package apextest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Project and @IsTest route twins for the captured anonymous and native-test
// relationship outcomes in platform_relationships/api68_boundary/org.tsv:2-7
// and platform_relationships/org.tsv:1781.
func TestAPI68RelationshipNamedProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			dir := filepath.Join(root, "force-app/main/default/classes")
			source := `public class RelationshipProject { public static String observe() { try { Database.query('SELECT Id,(SELECT Id FROM ContactPointOttSvcs LIMIT 1) FROM Account LIMIT 1'); return 'ok'; } catch (QueryException e) { return 'reject'; } } }`
			want := "reject"
			if api == "68.0" {
				want = "ok"
				source = `public class RelationshipProject { public static String observe() { List<Account> rows=[SELECT Id,(SELECT Id FROM ContactPointOttSvcs LIMIT 1) FROM Account LIMIT 1]; return 'ok'; } }`
			}
			writeFile(t, filepath.Join(dir, "RelationshipProject.cls"), source)
			writeFile(t, filepath.Join(dir, "RelationshipProjectTest.cls"), `@IsTest private class RelationshipProjectTest { @IsTest static void observe() { System.assertEquals('`+want+`', RelationshipProject.observe()); } }`)
			for _, name := range []string{"RelationshipProject", "RelationshipProjectTest"} {
				writeFile(t, filepath.Join(dir, name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("project diagnostics: %v", analysis.Diagnostics)
			}
			for _, warm := range []bool{false, true} {
				result := Run(index, Options{SelectedClasses: []string{"RelationshipProjectTest"}, NoDiskCache: !warm, Parallelism: 1})
				if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem != nil {
					t.Fatalf("warm=%t project run: %+v", warm, result.Summary())
				}
			}
		})
	}
}

// The schema dependency gate must bind the child query only for source API 68.
func TestAPI68RelationshipProjectCompileGate(t *testing.T) {
	source := `public class Probe { static void run() { Object x=[SELECT Id,(SELECT Id FROM ContactPointOttSvcs LIMIT 1) FROM Account LIMIT 1]; } }`
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			index := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: api}}
			got := compileGateReferencesSchemaObject("Probe.cls", source, newCompileGateUnits(index), "ContactPointOttSvc")
			if want := api == "68.0"; got != want {
				t.Fatalf("dependency found=%t, want %t", got, want)
			}
		})
	}
}

// Anonymous accessor and dynamic root query observations are in
// platform_relationships/api68_access/org.tsv:2-7.
func TestAPI68RelationshipAccessorAndRootRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			storage.EnsureStandardObject(&org, "Account")
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			accessor := `insert new Account(Name='Relationship probe'); Account a=[SELECT Id,(SELECT Id FROM ContactPointOttSvcs LIMIT 1) FROM Account LIMIT 1]; System.assertEquals(0,a.ContactPointOttSvcs.size());`
			analysis := sema.AnalyzeAnonymous(index, accessor, api)
			if got, want := analysis.HasErrors(), api != "68.0"; got != want {
				t.Errorf("accessor compile errors=%t want=%t: %v", got, want, analysis.Diagnostics)
			}
			if api == "68.0" {
				program, err := vm.CompileAnonymousWithOptions(accessor, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = runner.newMachine().Execute(program); err != nil {
					t.Errorf("accessor: %v", err)
				}
			}
			rootQuery := `Boolean caught=false;try { Database.query('SELECT Id FROM ContactPointOttSvc LIMIT 1'); } catch (QueryException e) { caught=true; } System.assertEquals(` + fmt.Sprint(api != "68.0") + `,caught);`
			program, err := vm.CompileAnonymousWithOptions(rootQuery, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = runner.newMachine().Execute(program); err != nil {
				t.Errorf("root query: %v", err)
			}
		})
	}
}

func TestAPI68RelationshipCompileGateUsesClassSourceVersion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	dir := filepath.Join(root, "force-app/main/default/classes")
	files := map[string]string{}
	for _, item := range []struct{ name, api string }{{"Probe67", "67.0"}, {"Probe68", "68.0"}} {
		path := filepath.Join(dir, item.name+".cls")
		source := `public class ` + item.name + ` { static void run() { Object x=[SELECT Id,(SELECT Id FROM ContactPointOttSvcs LIMIT 1) FROM Account LIMIT 1]; } }`
		files[path] = source
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+item.api+"</apiVersion></ApexClass>")
	}
	units := newCompileGateUnits(loadTestIndex(t, root))
	for path, source := range files {
		got := compileGateReferencesSchemaObject(path, source, units, "ContactPointOttSvc")
		want := filepath.Base(path) == "Probe68.cls"
		if got != want {
			t.Errorf("%s dependency=%t, want %t", filepath.Base(path), got, want)
		}
	}
}
