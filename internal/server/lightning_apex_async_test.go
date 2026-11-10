package server

import (
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

const lightningAsyncNativeFixture = "testdata/native_conformance/lightning_async"

// The captured rows: a Queueable enqueued by a synchronous transaction runs
// after that transaction commits, in its own transaction; a transaction that
// fails after enqueueing creates no job; a failing job does not fail the
// transaction that enqueued it.
func readLightningAsyncNativeRows(t *testing.T) map[string]map[string]string {
	t.Helper()
	file, err := os.Open(filepath.Join(lightningAsyncNativeFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 || strings.Join(rows[0], ",") != "api,case,route,outcome" {
		t.Fatalf("native header = %v", rows[0])
	}
	byAPI := map[string]map[string]string{}
	for _, row := range rows[1:] {
		if len(row) != 4 || row[2] != "anonymous" {
			t.Fatalf("invalid native row: %v", row)
		}
		if byAPI[row[0]] == nil {
			byAPI[row[0]] = map[string]string{}
		}
		byAPI[row[0]][row[1]] = row[3]
	}
	return byAPI
}

// An LWC action is a synchronous transaction on the server route. The same
// rules apply to the Queueables it enqueues.
func TestLightningApexRouteRunsEnqueuedQueueableAfterCommit(t *testing.T) {
	rows := readLightningAsyncNativeRows(t)
	if len(rows) != 3 {
		t.Fatalf("native API matrix = %v", rows)
	}
	for api, cases := range rows {
		for _, name := range []string{"enqueue_runs_after_commit", "rollback_discards_job", "job_failure_isolated"} {
			if cases[name] == "" {
				t.Fatalf("%s: missing native row %s", api, name)
			}
		}
		if !strings.Contains(cases["enqueue_runs_after_commit"], "job=Completed;afterCount=1") ||
			!strings.Contains(cases["rollback_discards_job"], "action=failed;newJobs=0;afterCount=0") ||
			!strings.Contains(cases["job_failure_isolated"], "action=success;job=Failed") {
			t.Fatalf("%s: unexpected native outcomes %v", api, cases)
		}
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/AsyncCtrl.cls"), `public class AsyncCtrl {
    public class Follow implements Queueable {
        String name;
        Boolean fail;
        Follow(String name, Boolean fail) { this.name = name; this.fail = fail; }
        public void execute(QueueableContext context) {
            insert new Account(Name = name + ' async');
            if (fail) { throw new IllegalArgumentException('async boom'); }
        }
    }
    @AuraEnabled
    public static String save(String name, Boolean failAsync, Boolean failAction) {
        insert new Account(Name = name);
        System.enqueueJob(new Follow(name, failAsync));
        if (failAction) { throw new AuraHandledException('action boom'); }
        return 'saved';
    }
}`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/AsyncCtrl.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "glade.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			org := testOrg()
			handler := NewWithStoreAndSource(&org, store, source)
			handler.SetProjectIndex(typesys.Build(p, schema))
			call := func(body string) string {
				t.Helper()
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AsyncCtrl/save", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
				}
				return rec.Body.String()
			}
			names := func() string {
				out := []string{}
				for _, record := range org.Objects["Account"].Records {
					out = append(out, record.Fields["Name"].String)
				}
				sort.Strings(out)
				return strings.Join(out, ",")
			}

			// enqueue_runs_after_commit: the action commits, then the job commits.
			if body := call(`{"name":"First","failAsync":false,"failAction":false}`); !strings.Contains(body, `"data":"saved"`) {
				t.Fatalf("action body = %s", body)
			}
			if got := names(); got != "First,First async" {
				t.Fatalf("after successful job accounts = %s", got)
			}
			// rollback_discards_job: a failed action leaves no record and no job.
			if body := call(`{"name":"Rolled","failAsync":false,"failAction":true}`); !strings.Contains(body, "action boom") {
				t.Fatalf("failed action body = %s", body)
			}
			if got := names(); got != "First,First async" {
				t.Fatalf("after failed action accounts = %s", got)
			}
			// job_failure_isolated: the action succeeds; only the job rolls back.
			if body := call(`{"name":"Second","failAsync":true,"failAction":false}`); !strings.Contains(body, `"data":"saved"`) {
				t.Fatalf("action with failing job body = %s", body)
			}
			if got := names(); got != "First,First async,Second" {
				t.Fatalf("after failing job accounts = %s", got)
			}
			// A later action starts with an empty queue.
			if body := call(`{"name":"Third","failAsync":false,"failAction":false}`); !strings.Contains(body, `"data":"saved"`) {
				t.Fatalf("later action body = %s", body)
			}
			if got := names(); got != "First,First async,Second,Third,Third async" {
				t.Fatalf("after later action accounts = %s", got)
			}
		})
	}
}
