package apextest

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestKnowledgeAssignmentOtherConformance(t *testing.T) {
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
		if row[2] != "other_assignment" && row[2] != "other_assignment_unlicensed" {
			continue
		}
		count++
		t.Run(row[0]+"/"+row[1]+"/"+row[2], func(t *testing.T) {
			allowed := row[2] == "other_assignment"
			if allowed && row[3] != "before-current;after-target;by-current;date-present;due-null;note-Review" || !allowed && row[3] != "System.InvalidParameterValueException" {
				t.Fatal(row)
			}
			class := "B4AssignOther"
			if !allowed {
				class = "B4AssignReject"
			}
			root := t.TempDir()
			src := filepath.Join(dir, "project")
			err := filepath.WalkDir(src, func(path string, e fs.DirEntry, err error) error {
				if err != nil || e.IsDir() {
					return err
				}
				if filepath.Base(filepath.Dir(path)) == "classes" && filepath.Base(path) != class+".cls" && filepath.Base(path) != class+".cls-meta.xml" {
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(src, path)
				if err != nil {
					return err
				}
				content := strings.ReplaceAll(string(body), "62.0", row[0])
				if filepath.Base(path) == class+".cls" {
					content = licenseKnowledgeTestClass(content)
				}
				writeFile(t, filepath.Join(root, rel), content)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if a := sema.Analyze(index); index.HasErrors() || a.HasErrors() {
				t.Fatalf("native source compiled: %v %v", index.Diagnostics, a.Diagnostics)
			}
			if row[1] == "isTest" {
				run := Run(index, Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
				if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
					t.Fatalf("native passed, Glade summary=%+v problem=%s", summary, firstRunProblem(run))
				}
				return
			}
			source := `User target=[SELECT Id FROM User WHERE Alias='b4test' LIMIT 1]; Knowledge__kav k=new Knowledge__kav(Title='Other Test',UrlName='other-test',Language='en_US'); insert k; k=[SELECT KnowledgeArticleId,AssignedToId FROM Knowledge__kav WHERE Id=:k.Id]; System.assertEquals(UserInfo.getUserId(),k.AssignedToId);`
			if allowed {
				source += `KbManagement.PublishingService.assignDraftArticleTask(k.KnowledgeArticleId,target.Id,'Review',null,false); k=[SELECT AssignedToId,AssignedById,AssignmentDate,AssignmentDueDate,AssignmentNote FROM Knowledge__kav WHERE Id=:k.Id]; System.assertEquals(target.Id,k.AssignedToId); System.assertEquals(UserInfo.getUserId(),k.AssignedById); System.assertNotEquals(null,k.AssignmentDate); System.assertEquals(null,k.AssignmentDueDate); System.assertEquals('Review',k.AssignmentNote);`
			} else {
				source += `try { KbManagement.PublishingService.assignDraftArticleTask(k.KnowledgeArticleId,target.Id,'Review',null,false); System.assert(false); } catch(System.InvalidParameterValueException expected) {}`
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
			storage.EnsureStandardObject(&org, "User")
			user, _ := storage.EnsureMutableObjectRecords(&org, "User")
			id := storage.ID("005000000000082AAA")
			user.Records[id] = storage.Record{ID: id, Object: "User", Fields: map[string]storage.Value{"Alias": storage.StringValue("b4test"), "UserPermissionsKnowledgeUser": storage.BooleanValue(allowed), "IsActive": storage.BooleanValue(true)}}
			user.Indexes = nil
			org.Objects["User"] = *user
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			if _, err = runner.execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	if count != 8 {
		t.Fatalf("expected eight native rows, got %d", count)
	}
}
