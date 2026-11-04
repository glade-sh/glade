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
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestInheritedOverloadVisualforceLightningRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			dir := filepath.Join(root, "force-app/main/default/classes")
			writeServerTestFile(t, filepath.Join(dir, "ScopeBase.cls"), `public virtual class ScopeBase { public String prefix = 'base:'; public String pick(String x) { return prefix + x; } public String pick(Integer x) { return prefix + String.valueOf(x); } }`)
			writeServerTestFile(t, filepath.Join(dir, "ScopeChild.cls"), `public class ScopeChild extends ScopeBase { public String getValue() { return pick('ok') + ':' + pick(7); } @AuraEnabled public static String run() { return new ScopeChild().getValue(); } }`)
			for _, name := range []string{"ScopeBase", "ScopeChild"} {
				writeServerTestFile(t, filepath.Join(dir, name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			pages := filepath.Join(root, "force-app/main/default/pages")
			writeServerTestFile(t, filepath.Join(pages, "Scope.page"), `<apex:page controller="ScopeChild" showHeader="false" sidebar="false"><apex:outputText value="{!value}"/></apex:page>`)
			vfAPI := api
			if api == "62.0" {
				vfAPI = "59.0"
			}
			writeServerTestFile(t, filepath.Join(pages, "Scope.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+vfAPI+`</apiVersion><label>Scope</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			idx := typesys.Build(p, sch)
			if result := sema.Analyze(idx); result.HasErrors() {
				t.Fatalf("semantic diagnostics: %#v", result.Diagnostics)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			host := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, host)
			host.SetProjectIndex(idx)
			for _, route := range []struct{ name, method, path, body string }{{"vf", http.MethodGet, "/apex/Scope", ""}, {"lightning", http.MethodPost, "/lightning/apex/ScopeChild/run", `{}`}} {
				t.Run(route.name, func(t *testing.T) {
					for i := 0; i < 2; i++ {
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
						req.Header.Set("Content-Type", "application/json")
						host.ServeHTTP(rec, req)
						if rec.Code != http.StatusOK {
							t.Fatalf("request %d: status %d: %s", i, rec.Code, rec.Body.String())
						}
						if route.name == "vf" {
							if !strings.Contains(rec.Body.String(), ">base:ok:base:7<") {
								t.Fatalf("request %d: %s", i, rec.Body.String())
							}
						} else {
							var out lwcbrowser.WireResponse
							if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
								t.Fatal(err)
							}
							if out.Error != nil || out.Data != "base:ok:base:7" {
								t.Fatalf("request %d: %#v", i, out)
							}
						}
					}
				})
			}
		})
	}
}
