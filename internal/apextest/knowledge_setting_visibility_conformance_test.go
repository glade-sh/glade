package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestKnowledgeDisabledSettingVisibilityConformance(t *testing.T) {
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
		if row[2] != "setting_disabled_visibility" && row[2] != "setting_omitted_visibility" {
			continue
		}
		count++
		t.Run(row[0]+"/"+row[1]+"/"+row[2], func(t *testing.T) {
			root := t.TempDir()
			shapeFile := "feature_enabled_setting_disabled.json"
			if row[2] == "setting_omitted_visibility" {
				shapeFile = "feature_enabled_setting_omitted.json"
			}
			shape, err := os.ReadFile(filepath.Join(dir, shapeFile))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(shape))
			p := project.Project{Root: root, SourceAPIVersion: row[0]}
			filename := "disabled.apex"
			if row[1] == "isTest" {
				filename = "disabled_named.apex"
			}
			source, err := os.ReadFile(filepath.Join(dir, filename))
			if err != nil {
				t.Fatal(err)
			}
			if row[1] == "isTest" {
				path := filepath.Join(root, "B4NoKnowledge62.cls")
				writeFile(t, path, string(source))
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
				p.ApexFiles = []string{path}
			}
			index := typesys.Build(p, gladeschema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			var result sema.Result
			if row[1] == "anonymous" {
				result = sema.AnalyzeAnonymous(index, string(source), row[0])
			} else {
				result = sema.Analyze(index)
			}
			for _, d := range result.Diagnostics {
				if d.Severity == diagnostic.Error && d.Message == row[3] {
					return
				}
			}
			t.Fatalf("native rejection %q absent: %v", row[3], result.Diagnostics)
		})
	}
	if count != 8 {
		t.Fatalf("expected eight native rows, got %d", count)
	}
}
