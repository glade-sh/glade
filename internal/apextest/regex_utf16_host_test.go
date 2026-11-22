package apextest_test

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Host adapters replay the anonymous observations in regex_java_divergence/
// org.tsv:757-758,770,2504-2505,2517; this is not a native host capture.
func TestRegexUTF16HostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
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
			write("force-app/main/default/classes/RegexHostController.cls", `public class RegexHostController {
 @AuraEnabled(cacheable=true) public static String observe() {
  Matcher m=Pattern.compile('').matcher('a\uD83D\uDE00b'); String positions='';
  while(m.find()) { positions += m.start()+','; }
  Matcher r=Pattern.compile('.').matcher('\uD83D\uDE00'); r.region(1,2);
  return positions+'|'+String.valueOf('a\uD83D\uDE00b'.replaceAll('','-').length())+'|'+String.valueOf(r.find());
 }
 public String getObservation() { return observe(); }
}`)
			write("force-app/main/default/classes/RegexHostController.cls-meta.xml", `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			write("force-app/main/default/pages/RegexHost.page", `<apex:page controller="RegexHostController" showHeader="false" sidebar="false"><pre id="observation"><apex:outputText value="{!observation}"/></pre></apex:page>`)
			write("force-app/main/default/pages/RegexHost.page-meta.xml", `<ApexPage><apiVersion>`+api+`</apiVersion><label>Regex host</label></ApexPage>`)
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
			storage.EnsureDeterministicPlatformData(&org)
			handler := server.NewWithSource(&org, source)
			handler.VisualforceHTMLUserID = storage.DefaultUserID
			handler.SetProjectIndex(typesys.Build(p, schema))
			for _, route := range []string{"Visualforce", "imperative", "wire"} {
				t.Run(route, func(t *testing.T) {
					method, path, body := http.MethodGet, "/apex/RegexHost", ""
					if route == "imperative" {
						method, path, body = http.MethodPost, "/lightning/apex/RegexHostController/observe", `{}`
					} else if route == "wire" {
						method, path, body = http.MethodPost, "/lightning/wire/apex", `{"className":"RegexHostController","method":"observe","params":{}}`
					}
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(method, path, strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					handler.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK {
						t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
					}
					var observation string
					if route == "Visualforce" {
						_, after, ok := strings.Cut(rec.Body.String(), `<pre id="observation">`)
						value, _, closed := strings.Cut(after, "</pre>")
						if !ok || !closed {
							t.Fatalf("missing observation: %s", rec.Body.String())
						}
						observation = html.UnescapeString(value)
					} else {
						var response lwcbrowser.WireResponse
						if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || response.Error != nil {
							t.Fatalf("response %s, err %v", rec.Body.String(), err)
						}
						var ok bool
						observation, ok = response.Data.(string)
						if !ok {
							t.Fatalf("observation %#v", response.Data)
						}
					}
					if observation != "0,1,2,3,4,|9|true" {
						t.Fatalf("got %q", observation)
					}
				})
			}
		})
	}
}
