package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// An LWC Apex action that writes and then throws must leave the server org
// untouched, including in-place updates of records that existed before the
// action. A successful action that writes must still be published.
func TestLightningApexRouteDiscardsWritesFromFailedAction(t *testing.T) {
	root := t.TempDir()
	writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/IsolationCtrl.cls"), `public class IsolationCtrl {
    @AuraEnabled
    public static String save(String name) {
        insert new Account(Name = name);
        return 'saved';
    }
    @AuraEnabled
    public static String renameThenFail(String name) {
        Account existing = [SELECT Id, Name FROM Account LIMIT 1];
        existing.Name = name;
        update existing;
        insert new Account(Name = 'Leaked insert');
        throw new AuraHandledException('boom');
    }
    @AuraEnabled
    public static String rename(String name) {
        Account existing = [SELECT Id, Name FROM Account LIMIT 1];
        existing.Name = name;
        update existing;
        return 'renamed';
    }
}`)
	writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes/IsolationCtrl.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>`)
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
	call := func(method, body string) string {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/lightning/apex/IsolationCtrl/"+method, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body = %s", method, rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	accountNames := func() []string {
		names := []string{}
		for _, record := range org.Objects["Account"].Records {
			names = append(names, record.Fields["Name"].String)
		}
		return names
	}

	if body := call("save", `{"name":"Original"}`); !strings.Contains(body, `"data":"saved"`) {
		t.Fatalf("save body = %s", body)
	}
	if body := call("renameThenFail", `{"name":"Must not leak"}`); !strings.Contains(body, "boom") {
		t.Fatalf("failing action body = %s", body)
	}
	if names := accountNames(); len(names) != 1 || names[0] != "Original" {
		t.Fatalf("failed action leaked writes: accounts = %v", names)
	}
	if body := call("rename", `{"name":"Renamed"}`); !strings.Contains(body, `"data":"renamed"`) {
		t.Fatalf("rename body = %s", body)
	}
	if names := accountNames(); len(names) != 1 || names[0] != "Renamed" {
		t.Fatalf("successful update not published: accounts = %v", names)
	}
}
