package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestUserManageUsersNativeConformance replays a native capture of a runAs
// Standard User holding Manage Internal Users or Manage Users, without Modify
// All Data, updating another user's record (API 62.0 and 67.0).
func TestUserManageUsersNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "native-capture", "user_manage_users")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 17 || strings.Join(rows[0], ",") != "source_api,route,method,outcome,message" {
		t.Fatalf("incomplete native matrix: %d rows", len(rows))
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyPermsetMutingProject(t, filepath.Join(dir, "project"), api)
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("project diagnostics: %v", index.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"ManageUsersCapture"}, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 {
				t.Fatalf("expected one suite: %+v", run.Summary())
			}
			observed := map[string]string{}
			for _, result := range run.Suites[0].Cases {
				if result.Problem == nil {
					observed[result.MethodName] = ""
					continue
				}
				observed[result.MethodName] = result.Problem.Message
			}
			compared := 0
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				t.Run(row[2], func(t *testing.T) {
					got, ok := observed[row[2]]
					if !ok {
						t.Fatalf("no Glade result for %s", row[2])
					}
					want := userSelfUpdateObservation(t, row[4])
					if have := userSelfUpdateObservation(t, got); !reflect.DeepEqual(have, want) {
						wantJSON, _ := json.Marshal(want)
						haveJSON, _ := json.Marshal(have)
						t.Fatalf("native=%s\nGlade=%s", wantJSON, haveJSON)
					}
				})
				compared++
			}
			if compared != 8 {
				t.Fatalf("compared %d native rows at %s, want 8", compared, api)
			}
		})
	}
}
