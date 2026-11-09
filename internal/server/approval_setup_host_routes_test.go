package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// The captured mixed-DML and permission-reference classes run unchanged through
// the Visualforce and Lightning routes, in cold and warm server state.
func TestApprovalSetupAndReferenceHostRoutes(t *testing.T) {
	fixtures := map[string]string{
		"ApprovalSetupCapture":     "../apextest/testdata/conformance/approval_setup_mixed",
		"ApprovalReferenceCapture": "../apextest/testdata/conformance/approval_reference",
	}
	for _, api := range []string{"62.0", "67.0"} {
		for class, fixture := range fixtures {
			t.Run(api+"/"+class, func(t *testing.T) {
				body, err := os.ReadFile(filepath.Join(fixture, "project/force-app/main/default/classes", class+".cls"))
				if err != nil {
					t.Fatal(err)
				}
				rows, err := os.ReadFile(filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), "anonymous/org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				var want strings.Builder
				for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
					key, raw, _ := strings.Cut(line, "\t")
					var value string
					if err := json.Unmarshal([]byte(raw), &value); err != nil {
						t.Fatal(err)
					}
					want.WriteString(key + "=" + value + ";")
				}
				root := t.TempDir()
				writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				meta := `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><status>Active</status></ApexClass>`
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/"+class+".cls"), string(body))
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/"+class+".cls-meta.xml"), meta)
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/SetupHost.cls"), `public class SetupHost {
  @AuraEnabled public static String run() { return `+class+`.observe(); }
  public String getOutcome() { return run(); }
}`)
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/SetupHost.cls-meta.xml"), meta)
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/SetupHost.page"), `<apex:page controller="SetupHost" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/SetupHost.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>59.0</apiVersion><label>Setup host</label></ApexPage>`)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				source, err := NewSourceMetadataFromProject(p)
				if err != nil {
					t.Fatal(err)
				}
				schema, err := gladeschema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, schema)
				org := apextest.OrgFromIndex(index)
				uid := storage.ID("005000000000991AAA")
				var admin storage.ID
				for id, row := range org.Objects["Profile"].Records {
					if row.Fields["Name"].String == "System Administrator" {
						admin = id
						break
					}
				}
				if admin == "" {
					t.Fatal("administrator profile missing")
				}
				users := org.Objects["User"]
				users.Records[uid] = storage.Record{ID: uid, Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(uid), "ProfileId": storage.IDValue(admin), "IsActive": storage.BooleanValue(true)}}
				org.Objects["User"] = users
				handler := newLightningTestServer(t, &org, source)
				handler.VisualforceHTMLUserID = uid
				handler.SetProjectIndex(index)
				if handler.runtimeErr != nil {
					t.Fatal(handler.runtimeErr)
				}
				for _, phase := range []string{"cold", "warm"} {
					t.Run(phase, func(t *testing.T) {
						rec := httptest.NewRecorder()
						handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/SetupHost", nil))
						if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), ">"+want.String()+"<") {
							t.Fatalf("VF status=%d want %s body=%s", rec.Code, want.String(), rec.Body.String())
						}
						rec = httptest.NewRecorder()
						req := httptest.NewRequest(http.MethodPost, "/lightning/apex/SetupHost/run", strings.NewReader(`{}`))
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("X-GLADE-User-Id", string(uid))
						handler.ServeHTTP(rec, req)
						var out lwcbrowser.WireResponse
						if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
							t.Fatal(err)
						}
						if rec.Code != http.StatusOK || out.Error != nil || out.Data != want.String() {
							t.Fatalf("LWC status=%d want %s response=%+v", rec.Code, want.String(), out)
						}
					})
				}
			})
		}
	}
}
