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

func TestKnowledgeCategoryRejectionConformance(t *testing.T) {
	runKnowledgeQueryRejection(t, "category_bad_group", "category_bad.apex", "category_bad_named.apex", "B4CategoryBad62")
}
func TestKnowledgeLatestRestrictionConformance(t *testing.T) {
	runKnowledgeQueryRejection(t, "latest_without_archived", "latest_bad.apex", "latest_bad_named.apex", "B4LatestBad62")
}
func runKnowledgeQueryRejection(t *testing.T, caseName, anonymous, named, className string) {
	t.Helper()
	dir := filepath.Join("testdata", "conformance", "knowledge_publishing")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range rows[1:] {
		if row[2] != caseName {
			continue
		}
		count++
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["Knowledge"],"settings":{"knowledgeSettings":{"enableKnowledge":true}}}`)
			proj := project.Project{Root: root, SourceAPIVersion: row[0]}
			filename := anonymous
			if row[1] == "isTest" {
				filename = named
			}
			source, err := os.ReadFile(filepath.Join(dir, filename))
			if err != nil {
				t.Fatal(err)
			}
			if row[1] == "isTest" {
				path := filepath.Join(root, className+".cls")
				writeFile(t, path, string(source))
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
				proj.ApexFiles = []string{path}
			}
			index := typesys.Build(proj, gladeschema.Schema{})
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
	if count != 4 {
		t.Fatalf("expected four native rows, got %d", count)
	}
}
