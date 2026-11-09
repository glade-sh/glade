package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDefaultUserSharingCapture replays the captured test-route private-record controls.
func TestDefaultUserSharingCapture(t *testing.T) {
	const fixture = "testdata/conformance/default_user_sharing"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyPermsetMutingProject(t, fixture, api)
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("project diagnostics: %v", index.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"ProbeSharingTest"}, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("missing observation: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil {
				t.Fatal("missing captured assertion")
			}
			want := ""
			for _, row := range rows[1:] {
				if row[0] == api && row[1] == "istest" {
					_, want, _ = strings.Cut(row[3], "CAP|")
				}
			}
			_, got, ok := strings.Cut(result.Problem.Message, "CAP|")
			if !ok || want == "" || got != want {
				t.Fatalf("native=%q Glade=%q (%+v)", want, got, result.Problem)
			}
		})
	}
}

// TestDefaultUserAnonymousCapture compares the successful fresh-org upserts.
func TestDefaultUserAnonymousCapture(t *testing.T) {
	const fixture = "testdata/conformance/default_user_review"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	for _, tc := range []struct{ api, name, source string }{{"62.0", "default-profile-and-upsert", "default.apex"}, {"67.0", "default-profile-and-upsert-fresh", "default_fresh.apex"}} {
		tc := tc
		t.Run(tc.api, func(t *testing.T) {
			root := copyPermsetMutingProject(t, filepath.Join(fixture, "project"), tc.api)
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("project diagnostics: %v", index.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = tc.api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			raw, err := os.ReadFile(filepath.Join(fixture, "anon", tc.source))
			if err != nil {
				t.Fatal(err)
			}
			source := strings.Replace(string(raw), "System.debug(LoggingLevel.ERROR,", "System.assert(false,", 1)
			_, err = runAnonymousObservation(t, runner, index, source, tc.api)
			got := anonymousAssertionMessage(t, err)
			want := ""
			for _, row := range rows[1:] {
				if row[0] == tc.api && row[1] == "anonymous" && row[2] == tc.name {
					want = row[3]
				}
			}
			_, wantJSON, ok := strings.Cut(want, "CAP|")
			if !ok {
				t.Fatal("missing native payload")
			}
			compareMethodObservation(t, wantJSON, strings.Replace(got, "CAP|", "OBS|", 1))
		})
	}
}

// TestUserManagementCapture replays the test-route grant and rejection cases.
func TestUserManagementCapture(t *testing.T) {
	const fixture = "testdata/conformance/default_user_review"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	for _, api := range []string{"62.0", "67.0"} {
		for _, grant := range []string{"modify-all", "modify-all-and-manage-users", "modify-all-and-manage-internal-users", "manage-internal-users"} {
			api, grant := api, grant
			t.Run(api+"/"+grant, func(t *testing.T) {
				root := copyPermsetMutingProject(t, filepath.Join(fixture, "project"), api)
				raw, err := os.ReadFile(filepath.Join(fixture, "permission-sets", grant+".xml"))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/ProbeGrant.permissionset-meta.xml"), string(raw))
				index := loadTestIndex(t, root)
				if index.HasErrors() {
					t.Fatalf("project diagnostics: %v", index.Diagnostics)
				}
				run := Run(index, Options{SelectedClasses: []string{"ProbeUsers"}, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 {
					t.Fatalf("missing suite: %+v", run.Summary())
				}
				compared := 0
				for _, result := range run.Suites[0].Cases {
					if result.Problem == nil {
						t.Fatalf("%s: missing assertion", result.MethodName)
					}
					want := ""
					for _, row := range rows[1:] {
						if row[0] == api && row[1] == "istest" && row[2] == grant+"/"+result.MethodName {
							_, want, _ = strings.Cut(row[3], "CAP|")
						}
					}
					_, got, ok := strings.Cut(result.Problem.Message, "CAP|")
					if !ok || want == "" || got != want {
						t.Errorf("%s native=%q Glade=%q (%+v)", result.MethodName, want, got, result.Problem)
					}
					compared++
				}
				if compared != 2 {
					t.Fatalf("compared %d cases", compared)
				}
			})
		}
	}
}
