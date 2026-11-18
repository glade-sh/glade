package apextest_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// These local HTTP hosts replay captured anonymous observations. The capture
// itself does not claim Salesforce Visualforce or LWC transport coverage.
func TestAccountCaptureHTTPHostRoutes(t *testing.T) {
	for _, tc := range []struct{ name, fixture, expression, prefix string }{
		{"delete", "account_delete_children", "LpDeleteProbe.run()", "D01 "},
		{"recordType", "account_object_settings_record_type", "LpRtProbe.run()", "R0"},
	} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(tc.name+"/"+api, func(t *testing.T) {
				fixture := filepath.Join("testdata/native-capture", tc.fixture)
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
				want := map[string]string{}
				for _, row := range rows[1:] {
					if row[0] == api && row[1] == "anonymous" && strings.HasPrefix(row[2], tc.prefix) {
						want[row[2]] = row[3]
					}
				}
				root := t.TempDir()
				if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
					t.Fatal(err)
				}
				err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return err
					}
					b, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					return os.WriteFile(path, []byte(strings.ReplaceAll(string(b), "67.0", api)), 0644)
				})
				if err != nil {
					t.Fatal(err)
				}
				write := func(name, contents string) {
					t.Helper()
					path := filepath.Join(root, name)
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
						t.Fatal(err)
					}
				}
				base := "force-app/main/default/"
				write(base+"classes/CaptureHost.cls", `public class CaptureHost {
 @AuraEnabled(cacheable=true) public static String observe() {
  List<String> rows = `+tc.expression+`;
  Schema.DescribeFieldResult d = EmailMessage.Status.getDescribe();
  rows.add('picklist|' + d.getType() + '|' + d.isRestrictedPicklist() + '|' + d.getPicklistValues().size() + '|' + d.getPicklistValues()[3].getLabel());
  return JSON.serialize(rows);
 }
 public String getObservation() { return observe(); }
}`)
				write(base+"classes/CaptureHost.cls-meta.xml", `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
				write(base+"pages/CaptureHost.page", `<apex:page controller="CaptureHost" showHeader="false" sidebar="false"><pre id="observation"><apex:outputText value="{!observation}"/></pre></apex:page>`)
				write(base+"pages/CaptureHost.page-meta.xml", `<ApexPage><apiVersion>`+api+`</apiVersion><label>Capture host</label></ApexPage>`)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				sch, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, sch)
				org := apextest.OrgFromIndex(index)
				org.APIVersion = api
				source, err := server.NewSourceMetadataFromProject(p)
				if err != nil {
					t.Fatal(err)
				}
				const userID storage.ID = "005000000000091"
				storage.EnsureStandardObject(&org, "User")
				users := org.Objects["User"]
				users.Records[userID] = storage.Record{ID: userID, Object: "User", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true)}}
				org.Objects["User"] = users
				handler := server.NewWithSource(&org, source)
				handler.VisualforceHTMLUserID = userID
				handler.SetProjectIndex(index)
				for _, route := range []string{"vf", "lwc", "wire"} {
					t.Run(route, func(t *testing.T) {
						for _, state := range []string{"cold", "warm"} {
							method, path, body := http.MethodGet, "/apex/CaptureHost", ""
							if route == "lwc" {
								method, path, body = http.MethodPost, "/lightning/apex/CaptureHost/observe", "{}"
							}
							if route == "wire" {
								method, path, body = http.MethodPost, "/lightning/wire/apex", `{"className":"CaptureHost","method":"observe","params":{}}`
							}
							req := httptest.NewRequest(method, path, strings.NewReader(body))
							req.Header.Set("Content-Type", "application/json")
							rec := httptest.NewRecorder()
							handler.ServeHTTP(rec, req)
							if rec.Code != http.StatusOK {
								t.Fatalf("%s status=%d body=%s", state, rec.Code, rec.Body.String())
							}
							var observed string
							if route == "vf" {
								_, after, ok := strings.Cut(rec.Body.String(), `<pre id="observation">`)
								value, _, closed := strings.Cut(after, "</pre>")
								if !ok || !closed {
									t.Fatalf("missing observation: %s", rec.Body.String())
								}
								observed = html.UnescapeString(value)
							} else {
								var response lwcbrowser.WireResponse
								if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
									t.Fatal(err)
								}
								if response.Error != nil {
									t.Fatalf("%s bridge: %+v", state, response.Error)
								}
								observed, _ = response.Data.(string)
							}
							var entries []string
							if err := json.Unmarshal([]byte(observed), &entries); err != nil {
								t.Fatalf("%s decode %v: %s", state, err, observed)
							}
							got := map[string]string{}
							for _, entry := range entries {
								id, value, ok := strings.Cut(entry, "|")
								if ok {
									got[id] = strings.TrimRight(value, " \n")
								}
							}
							if got["picklist"] != "PICKLIST|true|6|Sent" {
								t.Errorf("%s picklist: %q", state, got["picklist"])
							}
							for id, expected := range want {
								if got[id] != expected {
									t.Errorf("%s %s: got %q want %q", state, id, got[id], expected)
								}
							}
							if len(want) == 0 {
								t.Fatal(fmt.Sprintf("no capture rows for %s", tc.name))
							}
						}
					})
				}
			})
		}
	}
}
