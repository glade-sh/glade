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

func TestKnowledgeUnlicensedUserConformance(t *testing.T) {
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
	count := 0
	for _, row := range rows[1:] {
		if row[2] != "setting_only_unlicensed" {
			continue
		}
		count++
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			shape, err := os.ReadFile(filepath.Join(dir, "setting_enabled_feature_omitted.json"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(shape))
			p := project.Project{Root: root, SourceAPIVersion: row[0]}
			filename := "disabled.apex"
			if row[1] == "isTest" {
				filename = "setting_only_named.apex"
			}
			source, err := os.ReadFile(filepath.Join(dir, filename))
			if err != nil {
				t.Fatal(err)
			}
			if row[1] == "isTest" {
				path := filepath.Join(root, "B4SettingOnly62.cls")
				writeFile(t, path, string(source))
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
				p.ApexFiles = []string{path}
			}
			index := typesys.Build(p, gladeschema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if row[1] == "isTest" {
				if a := sema.Analyze(index); a.HasErrors() {
					t.Fatalf("native compiled: %v", a.Diagnostics)
				}
				run := Run(index, Options{SelectedClasses: []string{"B4SettingOnly62"}, NoDiskCache: true, Parallelism: 1})
				if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
					t.Fatalf("native passed: %+v problem=%s", summary, firstRunProblem(run))
				}
				return
			}
			if a := sema.AnalyzeAnonymous(index, string(source), row[0]); a.HasErrors() {
				t.Fatalf("native compiled: %v", a.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = row[0]
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range result.Debug {
				if line == row[3] {
					return
				}
			}
			t.Fatalf("native observed %s, debug=%v", row[3], result.Debug)
		})
	}
	if count != 4 {
		t.Fatalf("expected four native rows, got %d", count)
	}
}
