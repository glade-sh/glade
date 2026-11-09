package apextest

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Replays captured rows from both native routes at both source versions.
func TestKnowledgePublishingB4LifecycleConformance(t *testing.T) {
	runKnowledgeB4Rows(t, "", "B4Lifecycle", map[string]string{
		"draft": "Draft:0:en_US", "published": "Online:1", "edit_parent": "false",
		"edit_draft": "Draft:0:en_US", "edit_again": "System.HandledException",
		"assign": "returned", "archive_with_draft": "System.HandledException",
		"archive_clean": "Archived:1:en_US", "archive_again": "System.HandledException",
	})
}
func TestKnowledgePublishingB4CategoryConformance(t *testing.T) {
	runKnowledgeB4Rows(t, "category_", "B4CategoryProbe", map[string]string{"category_insert": "returned", "category_count": "1", "category_query": "1", "category_invalid": "INVALID_DATA_CATEGORY_GROUP_REFERENCE"})
}
func TestKnowledgePublishingB4CategoryDeleteConformance(t *testing.T) {
	runKnowledgeB4Rows(t, "category_delete", "B4CategoryDelete", map[string]string{"category_delete_before": "1", "category_delete": "returned", "category_delete_after": "0", "category_delete_allrows": "0"})
}
func TestKnowledgePublishingB4CategoryBoundaryConformance(t *testing.T) {
	runKnowledgeB4Rows(t, "cat_", "B4CategoryBoundary", map[string]string{"cat_at": "1", "cat_above": "1", "cat_below": "1", "cat_both": "1", "cat_root_at": "0", "cat_root_above": "0", "cat_root_below": "1", "cat_root_both": "1"})
}
func TestKnowledgePublishingB4AssignmentConformance(t *testing.T) {
	runKnowledgeB4Rows(t, "assignment_", "B4AssignState", map[string]string{"assignment_before": "assignee-present;note-null;due-null", "assignment_after": "assignee-user;assigned-by-user;date-present;due-second;note-synthetic"})
}
func TestKnowledgePublishingB4RepublishConformance(t *testing.T) {
	runKnowledgeB4Rows(t, "republish_", "B4Republish", map[string]string{"republish_old_count": "0", "republish_status": "Online:1"})
}
func TestKnowledgePublishingB4VersionStateConformance(t *testing.T) {
	runKnowledgeB4Rows(t, "version_", "B4VersionState", map[string]string{"version_unpublish_original": "Archived:1:false", "version_archived_status_query": "0", "version_replaced_allrows": "Online:true", "version_major_original": "Archived:1:false", "version_major_current": "Online:2"})
}
func TestKnowledgePublishingB4HistoryConformance(t *testing.T) {
	runKnowledgeB4Rows(t, "history_", "B4History", map[string]string{"history_id_status": "1", "history_id_latest": "1", "history_status_latest": "1", "history_base_id_status": "1", "history_third_major": "3", "history_minor_after_major": "3"})
}
func TestKnowledgePublishingB4FieldConformance(t *testing.T) {
	runKnowledgeB4Rows(t, "field_", "B4FieldProbe", map[string]string{"field_draft": "owned", "field_online": "owned"})
}
func runKnowledgeB4Rows(t *testing.T, prefix, className string, want map[string]string) {
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
	seen := map[string]map[string]string{}
	for _, row := range rows[1:] {
		if _, included := want[row[2]]; included && strings.HasPrefix(row[2], prefix) {
			key := row[0] + "/" + row[1]
			if seen[key] == nil {
				seen[key] = map[string]string{}
			}
			seen[key][row[2]] = row[3]
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				expected := seen[api+"/"+route]
				if !reflect.DeepEqual(expected, want) {
					t.Fatalf("missing native rows: %v", expected)
				}
				root := t.TempDir()
				src := filepath.Join(dir, "project")
				err := filepath.WalkDir(src, func(path string, e fs.DirEntry, err error) error {
					if err != nil || e.IsDir() {
						return err
					}
					if filepath.Base(filepath.Dir(path)) == "classes" && filepath.Base(path) != className+".cls" && filepath.Base(path) != className+".cls-meta.xml" {
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
					content := strings.ReplaceAll(string(body), "62.0", api)
					if filepath.Base(path) == className+".cls" {
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
				if route == "isTest" {
					run := Run(index, Options{SelectedClasses: []string{className}, NoDiskCache: true, Parallelism: 1})
					if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
						t.Fatalf("native passed, Glade summary=%+v problem=%s", summary, firstRunProblem(run))
					}
					return
				}
				// Reuse the native named assertions as an anonymous body so each route
				// verifies the same observed values, not merely compilation.
				source, err := os.ReadFile(filepath.Join(src, "force-app/main/default/classes", className+".cls"))
				if err != nil {
					t.Fatal(err)
				}
				body := strings.TrimSpace(string(source))
				body = strings.TrimPrefix(body, "@IsTest private class "+className+" { @IsTest static void observe() {")
				body = strings.TrimSuffix(body, "} }")
				if a := sema.AnalyzeAnonymous(index, body, api); a.HasErrors() {
					t.Fatal(a.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				licenseKnowledgeTestOrg(&org)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
				if _, err = runner.execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// The captured positive org had the scratch admin's Knowledge User flag set
// before either route ran. Recreate that setup in the offline project runner.
func licenseKnowledgeTestClass(source string) string {
	return strings.Replace(source, "{ @IsTest static", "{ @TestSetup static void licenseActor() { User u=[SELECT Id,UserPermissionsKnowledgeUser FROM User WHERE Id=:UserInfo.getUserId()]; u.UserPermissionsKnowledgeUser=true; update u; } @IsTest static", 1)
}

func licenseKnowledgeTestOrg(org *storage.OrgState) {
	users, _ := storage.EnsureMutableObjectRecords(org, "User")
	id, user, ok := storage.LookupRecordByID(users.Records, storage.DefaultUserID)
	if ok {
		user = user.Clone()
		user.Fields["UserPermissionsKnowledgeUser"] = storage.BooleanValue(true)
		users.Records[id] = user
		users.Indexes = nil
		org.Objects["User"] = *users
	}
}
