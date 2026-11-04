package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

const journalBulkBody = `List<Account> rows = new List<Account>();
for (Integer i = 0; i < 40; i++) rows.add(new Account(Name = 'Before'));
insert rows;
for (Account row : rows) row.Name = 'After';
update rows;`

func TestJournalBulkServerAndLightningRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			t.Run("anonymous", func(t *testing.T) {
				org := testOrg()
				host := New(&org)
				body, err := json.Marshal(map[string]string{"anonymousBody": journalBulkBody})
				if err != nil {
					t.Fatal(err)
				}
				rec := httptest.NewRecorder()
				host.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(body))))
				if api == "62.0" {
					if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "unsupported API version v62.0") {
						t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
					}
					return
				}
				if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"success":true`) {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
				checkJournalBulkAccounts(t, &org, 40)
			})
			t.Run("lightning_cold_warm", func(t *testing.T) {
				root := t.TempDir()
				writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/JournalBulkController.cls"), `public class JournalBulkController { @AuraEnabled public static String run() { `+journalBulkBody+` return String.valueOf(rows.size()); } }`)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				sch, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				source, err := NewSourceMetadataFromProject(p)
				if err != nil {
					t.Fatal(err)
				}
				org := testOrg()
				host := newLightningTestServer(t, &org, source)
				host.SetProjectRuntime(typesys.Build(p, sch), nil, nil)
				for i := 1; i <= 2; i++ {
					rec := httptest.NewRecorder()
					host.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/apex/JournalBulkController/run", strings.NewReader(`{}`)))
					var result lwcbrowser.WireResponse
					if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if rec.Code != http.StatusOK || result.Error != nil || result.Data != "40" {
						t.Fatalf("request %d: status=%d body=%s", i, rec.Code, rec.Body.String())
					}
					checkJournalBulkAccounts(t, &org, 40*i)
				}
			})
		})
	}
}

func checkJournalBulkAccounts(t *testing.T, org *storage.OrgState, count int) {
	t.Helper()
	records := org.Objects["Account"].Records
	if len(records) != count {
		t.Fatalf("accounts = %d, want %d", len(records), count)
	}
	for _, row := range records {
		if row.Fields["Name"].String != "After" {
			t.Fatalf("updated account = %#v", row.Fields["Name"])
		}
	}
}
