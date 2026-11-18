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

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Replay the captured anonymous observation through local VF and Lightning hosts.
func TestStandardExtUpsertVisualforceAndLightningRoutes(t *testing.T) {
	fixture := filepath.Join("testdata", "native-capture", "standard_ext_upsert_anonymous")
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		f := strings.SplitN(line, "\t", 3)
		if len(f) != 3 {
			t.Fatal("bad native row")
		}
		if rows[f[0]] == nil {
			rows[f[0]] = map[string]string{}
		}
		rows[f[0]][f[1]] = f[2]
	}
	write := func(root, path, body string) {
		t.Helper()
		p := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			write(root, "sfdx-project.json", fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for _, name := range []string{"ExtProbe", "ExtProbeTest", "ExtHost"} {
				write(root, "force-app/main/default/classes/"+name+".cls-meta.xml", `<ApexClass><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			write(root, "force-app/main/default/classes/ExtHost.cls", `public class ExtHost { @AuraEnabled(cacheable=true) public static String observe() { return JSON.serialize(ExtProbe.run()); } public String getObservation() { return observe(); } }`)
			write(root, "force-app/main/default/pages/ExtHost.page", `<apex:page controller="ExtHost" showHeader="false" sidebar="false"><pre id="observation"><apex:outputText value="{!observation}"/></pre></apex:page>`)
			write(root, "force-app/main/default/pages/ExtHost.page-meta.xml", `<ApexPage><apiVersion>59.0</apiVersion><label>ExtHost</label></ApexPage>`)
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
			index := typesys.Build(p, schema)
			org := apextest.OrgFromIndex(index)
			org.APIVersion = api
			for _, route := range []string{"vf", "lightning", "wire"} {
				t.Run(route, func(t *testing.T) {
					// Each native observation starts in a fresh transaction; isolate
					// stored keys as well because the local random seed repeats.
					routeOrg := org.Clone()
					h := server.NewWithSource(&routeOrg, source)
					h.VisualforceHTMLUserID = storage.DefaultUserID
					h.SetProjectIndex(index)
					method, path, body := http.MethodGet, "/apex/ExtHost", ""
					if route == "lightning" {
						method, path, body = http.MethodPost, "/lightning/apex/ExtHost/observe", "{}"
					} else if route == "wire" {
						method, path, body = http.MethodPost, "/lightning/wire/apex", `{"className":"ExtHost","method":"observe","params":{}}`
					}
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(method, path, strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("X-GLADE-User-Id", string(storage.DefaultUserID))
					h.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK {
						t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
					}
					var observed string
					if route == "vf" {
						_, s, ok := strings.Cut(rec.Body.String(), `<pre id="observation">`)
						v, _, end := strings.Cut(s, "</pre>")
						if !ok || !end {
							t.Fatalf("missing observation: %s", rec.Body.String())
						}
						observed = html.UnescapeString(v)
					} else {
						var out lwcbrowser.WireResponse
						if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
							t.Fatal(err)
						}
						if out.Error != nil {
							t.Fatalf("bridge error: %+v", out.Error)
						}
						var ok bool
						observed, ok = out.Data.(string)
						if !ok {
							t.Fatalf("unexpected data %#v", out.Data)
						}
					}
					var values []string
					if err := json.Unmarshal([]byte(observed), &values); err != nil {
						t.Fatalf("invalid observation %q: %v", observed, err)
					}
					got := map[string]string{}
					for _, v := range values {
						k, val, ok := strings.Cut(v, "=")
						if ok {
							got[k] = val
						}
					}
					if len(rows[api]) != 48 {
						t.Fatal("native row count")
					}
					for k, want := range rows[api] {
						if got[k] != want {
							t.Errorf("%s got %q want %q", k, got[k], want)
						}
					}
				})
			}
		})
	}
}
