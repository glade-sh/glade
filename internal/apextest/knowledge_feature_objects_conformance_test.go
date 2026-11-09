package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The TSV rows supply expected describe visibility for each isolated shape.
func TestKnowledgeFeatureOnlyObjectsConformance(t *testing.T) {
	runKnowledgeObjectsShape(t, "feature_only", "feature_enabled_setting_omitted.json")
}

func TestKnowledgeSettingOnlyObjectsConformance(t *testing.T) {
	runKnowledgeObjectsShape(t, "setting_only", "setting_enabled_feature_omitted.json")
}

func TestKnowledgeDisabledSettingObjectsConformance(t *testing.T) {
	runKnowledgeObjectsShape(t, "setting_disabled", "feature_enabled_setting_disabled.json")
}

func runKnowledgeObjectsShape(t *testing.T, prefix, shapeFile string) {
	t.Helper()
	dir := filepath.Join("testdata", "conformance", "knowledge_publishing")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	cells := map[string]map[string]string{}
	for _, row := range rows[1:] {
		if row[2] != prefix+"_ka" && row[2] != prefix+"_kav" {
			continue
		}
		key := row[0] + "/" + row[1]
		if cells[key] == nil {
			cells[key] = map[string]string{}
		}
		cells[key][row[2]] = row[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				cell := cells[api+"/"+route]
				ka, kav := cell[prefix+"_ka"], cell[prefix+"_kav"]
				if len(cell) != 2 || (ka != "true" && ka != "false") || (kav != "true" && kav != "false") {
					t.Fatalf("native cells: %v", cell)
				}
				source := `Map<String,Schema.SObjectType> types=Schema.getGlobalDescribe(); System.assertEquals(` + ka + `,types.containsKey('Knowledge__ka')); System.assertEquals(` + kav + `,types.containsKey('Knowledge__kav'));`
				root := t.TempDir()
				shape, err := os.ReadFile(filepath.Join(dir, shapeFile))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(shape))
				p := project.Project{Root: root, SourceAPIVersion: api}
				if route == "isTest" {
					path := filepath.Join(root, "B4FeatureObjects62.cls")
					writeFile(t, path, "@IsTest private class B4FeatureObjects62 { @IsTest static void observe() { "+source+" } }")
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					p.ApexFiles = []string{path}
				}
				index := typesys.Build(p, gladeschema.Schema{})
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				if route == "isTest" {
					if a := sema.Analyze(index); a.HasErrors() {
						t.Fatal(a.Diagnostics)
					}
					run := Run(index, Options{SelectedClasses: []string{"B4FeatureObjects62"}, NoDiskCache: true, Parallelism: 1})
					if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
						t.Fatalf("native passed: %+v %s", s, firstRunProblem(run))
					}
					return
				}
				if a := sema.AnalyzeAnonymous(index, source, api); a.HasErrors() {
					t.Fatal(a.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
				if _, err = runner.execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
