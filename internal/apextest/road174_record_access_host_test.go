package apextest_test

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Replay the anonymous and named native query outcomes through local host adapters.
func TestRoad174RecordAccessHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			apextest.InvalidateRuntimeCaches()
			t.Cleanup(apextest.InvalidateRuntimeCaches)
			root := t.TempDir()
			write := func(name, body string) {
				t.Helper()
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("sfdx-project.json", fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			write("force-app/main/default/classes/RecordAccessHost.cls", `public class RecordAccessHost {
 @AuraEnabled(cacheable=true) public static String observe(String objectName) {
  try { Database.query('SELECT UserRecordAccess.HasReadAccess FROM '+objectName+' LIMIT 1'); return 'ok'; }
  catch (QueryException e) { return 'reject'; }
 }
 public String getObservation() { return observe(ApexPages.currentPage().getParameters().get('objectName')); }
}`)
			write("force-app/main/default/classes/RecordAccessHost.cls-meta.xml", `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			write("force-app/main/default/pages/RecordAccessHost.page", `<apex:page controller="RecordAccessHost" showHeader="false" sidebar="false"><pre id="observation"><apex:outputText value="{!observation}"/></pre></apex:page>`)
			write("force-app/main/default/pages/RecordAccessHost.page-meta.xml", `<ApexPage><apiVersion>`+api+`</apiVersion><label>Record access</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			source, err := server.NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			const userID storage.ID = "005000000000091"
			for _, name := range []string{"User", "Lead", "Campaign", "Asset", "Task", "Event"} {
				storage.EnsureStandardObject(&org, name)
			}
			users := org.Objects["User"]
			users.Records[userID] = storage.Record{ID: userID, Object: "User", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true)}}
			org.Objects["User"] = users
			handler := server.NewWithSource(&org, source)
			handler.VisualforceHTMLUserID = userID
			handler.SetProjectIndex(typesys.Build(p, schema))
			for _, row := range []struct{ name, want string }{{"Lead", "ok"}, {"Campaign", "ok"}, {"Asset", "ok"}, {"Task", "reject"}, {"Event", "reject"}} {
				for _, route := range []string{"vf", "imperative", "wire"} {
					t.Run(row.name+"/"+route, func(t *testing.T) {
						method, path, body := http.MethodGet, "/apex/RecordAccessHost?"+url.Values{"objectName": {row.name}}.Encode(), []byte(nil)
						if route != "vf" {
							method = http.MethodPost
							path = "/lightning/apex/RecordAccessHost/observe"
							body, _ = json.Marshal(map[string]string{"objectName": row.name})
							if route == "wire" {
								path = "/lightning/wire/apex"
								body, _ = json.Marshal(map[string]any{"className": "RecordAccessHost", "method": "observe", "params": map[string]string{"objectName": row.name}})
							}
						}
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(method, path, strings.NewReader(string(body)))
						req.Header.Set("Content-Type", "application/json")
						handler.ServeHTTP(rec, req)
						if rec.Code != http.StatusOK {
							t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
						}
						var got string
						if route == "vf" {
							_, after, ok := strings.Cut(rec.Body.String(), `<pre id="observation">`)
							val, _, closed := strings.Cut(after, "</pre>")
							if !ok || !closed {
								t.Fatalf("missing observation: %s", rec.Body.String())
							}
							got = html.UnescapeString(val)
						} else {
							var response lwcbrowser.WireResponse
							if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
								t.Fatal(err)
							}
							if response.Error != nil {
								t.Fatalf("bridge error: %+v", response.Error)
							}
							got, _ = response.Data.(string)
						}
						if got != row.want {
							t.Fatalf("got %q, native %q", got, row.want)
						}
					})
				}
			}
		})
	}
}
