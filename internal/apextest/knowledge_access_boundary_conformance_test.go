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
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Supplemental replay of the unlicensed_valid_article capture; the original
// failing null-ID guard test preceded the permission fix.
func TestKnowledgeUnlicensedValidArticleConformance(t *testing.T) {
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
		if row[2] != "unlicensed_valid_article" {
			continue
		}
		count++
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["Knowledge"],"settings":{"knowledgeSettings":{"enableKnowledge":true}}}`)
			p := project.Project{Root: root, SourceAPIVersion: row[0]}
			file := "access_boundary_anonymous.apex"
			if row[1] == "isTest" {
				file = "access_boundary_named.apex"
			}
			src, err := os.ReadFile(filepath.Join(dir, file))
			if err != nil {
				t.Fatal(err)
			}
			if row[1] == "isTest" {
				path := filepath.Join(root, "B4AccessBoundary62.cls")
				// The capture's scratch admin had its flag explicitly disabled.
				setup := "{ @TestSetup static void unlicenseActor() { User u=[SELECT Id,UserPermissionsKnowledgeUser FROM User WHERE Id=:UserInfo.getUserId()]; u.UserPermissionsKnowledgeUser=false; update u; } @IsTest static"
				writeFile(t, path, strings.Replace(string(src), "{ @IsTest static", setup, 1))
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
				run := Run(index, Options{SelectedClasses: []string{"B4AccessBoundary62"}, NoDiskCache: true, Parallelism: 1})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
					t.Fatalf("native passed: %+v %s", s, firstRunProblem(run))
				}
				return
			}
			if a := sema.AnalyzeAnonymous(index, string(src), row[0]); a.HasErrors() {
				t.Fatal(a.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(string(src), vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = row[0]
			parent := storage.ID("ka0000000000001")
			draft := storage.ID("ka0000000000002")
			for _, object := range []string{"Knowledge__ka", "Knowledge__kav"} {
				storage.EnsureStandardObject(&org, object)
			}
			parents, _ := storage.EnsureMutableObjectRecords(&org, "Knowledge__ka")
			parents.Records[parent] = storage.Record{ID: parent, Object: "Knowledge__ka"}
			org.Objects["Knowledge__ka"] = *parents
			versions, _ := storage.EnsureMutableObjectRecords(&org, "Knowledge__kav")
			versions.Records[draft] = storage.Record{ID: draft, Object: "Knowledge__kav", Fields: map[string]storage.Value{"KnowledgeArticleId": storage.IDValue(parent), "PublishStatus": storage.StringValue("Draft")}}
			org.Objects["Knowledge__kav"] = *versions
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range result.Debug {
				if line == "B4:unlicensed_valid:"+row[3] {
					return
				}
			}
			t.Fatalf("native %s absent: %v", row[3], result.Debug)
		})
	}
	if count != 4 {
		t.Fatalf("expected four native rows, got %d", count)
	}
}
