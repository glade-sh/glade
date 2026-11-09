package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// An LWC action may enqueue a Queueable. Like Salesforce, the action commits
// first and the job runs afterwards in its own transaction: its writes are
// committed too, and a failing job does not fail or roll back the action.
func TestLightningApexRouteRunsEnqueuedQueueableAfterCommit(t *testing.T) {
	root := t.TempDir()
	writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/AsyncCtrl.cls"), `public class AsyncCtrl {
    public class Follow implements Queueable {
        String name;
        Boolean fail;
        Follow(String name, Boolean fail) { this.name = name; this.fail = fail; }
        public void execute(QueueableContext context) {
            insert new Account(Name = name + ' async');
            if (fail) { throw new AuraHandledException('async boom'); }
        }
    }
    @AuraEnabled
    public static String save(String name, Boolean failAsync) {
        insert new Account(Name = name);
        System.enqueueJob(new Follow(name, failAsync));
        return 'saved';
    }
}`)
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/AsyncCtrl.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>`)
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
	names := func() []string {
		out := []string{}
		for _, record := range org.Objects["Account"].Records {
			out = append(out, record.Fields["Name"].String)
		}
		sort.Strings(out)
		return out
	}

	if body := call(`{"name":"First","failAsync":false}`); !strings.Contains(body, `"data":"saved"`) {
		t.Fatalf("action body = %s", body)
	}
	if got := strings.Join(names(), ","); got != "First,First async" {
		t.Fatalf("after successful job accounts = %s", got)
	}
	if body := call(`{"name":"Second","failAsync":true}`); !strings.Contains(body, `"data":"saved"`) {
		t.Fatalf("action with failing job body = %s", body)
	}
	if got := strings.Join(names(), ","); got != "First,First async,Second" {
		t.Fatalf("after failing job accounts = %s", got)
	}
}
