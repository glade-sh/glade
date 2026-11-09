package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Each row supplies the expected value to the executed Apex assertion.
func TestKnowledgeNativeRowsDriven(t *testing.T) {
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
	setup := `Knowledge__kav k=new Knowledge__kav(Title='Synthetic row',UrlName='synthetic-row',Language='en_US');insert k;k=[SELECT KnowledgeArticleId,PublishStatus,VersionNumber,Language FROM Knowledge__kav WHERE Id=:k.Id];Id parent=k.KnowledgeArticleId;`
	bodies := map[string]string{
		"draft":         setup + `String observed=k.PublishStatus+':'+k.VersionNumber+':'+k.Language;`,
		"published":     setup + `KbManagement.PublishingService.publishArticle(parent,false);k=[SELECT PublishStatus,VersionNumber FROM Knowledge__kav WHERE Id=:k.Id];String observed=k.PublishStatus+':'+k.VersionNumber;`,
		"edit_draft":    setup + `KbManagement.PublishingService.publishArticle(parent,false);Id draft=KbManagement.PublishingService.editOnlineArticle(parent,false);k=[SELECT PublishStatus,VersionNumber,Language FROM Knowledge__kav WHERE Id=:draft];String observed=k.PublishStatus+':'+k.VersionNumber+':'+k.Language;`,
		"archive_clean": setup + `KbManagement.PublishingService.publishArticle(parent,true);KbManagement.PublishingService.archiveOnlineArticle(parent,null);k=[SELECT PublishStatus,VersionNumber,Language FROM Knowledge__kav WHERE Id=:k.Id];String observed=k.PublishStatus+':'+k.VersionNumber+':'+k.Language;`,
	}
	count := 0
	for _, row := range rows[1:] {
		body, ok := bodies[row[2]]
		if !ok {
			continue
		}
		count++
		t.Run(row[0]+"/"+row[1]+"/"+row[2], func(t *testing.T) {
			if strings.ContainsAny(row[3], "'\\\n\r") {
				t.Fatal("unexpected oracle value")
			}
			source := body + "System.assertEquals('" + row[3] + "',observed);"
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["Knowledge"],"settings":{"knowledgeSettings":{"enableKnowledge":true}}}`)
			p := project.Project{Root: root, SourceAPIVersion: row[0]}
			if row[1] == "isTest" {
				path := filepath.Join(root, "B4DataRow.cls")
				class := licenseKnowledgeTestClass("@IsTest private class B4DataRow { @IsTest static void observe() {" + source + "} }")
				writeFile(t, path, class)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
				p.ApexFiles = []string{path}
			}
			index := typesys.Build(p, gladeschema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if row[1] == "isTest" {
				if a := sema.Analyze(index); a.HasErrors() {
					t.Fatal(a.Diagnostics)
				}
				run := Run(index, Options{SelectedClasses: []string{"B4DataRow"}, NoDiskCache: true, Parallelism: 1})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
					t.Fatalf("native %s: %+v %s", row[3], s, firstRunProblem(run))
				}
				return
			}
			if a := sema.AnalyzeAnonymous(index, source, row[0]); a.HasErrors() {
				t.Fatal(a.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = row[0]
			licenseKnowledgeTestOrg(&org)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			if _, err = runner.execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	if count != 16 {
		t.Fatalf("expected 16 native cells, got %d", count)
	}
}
