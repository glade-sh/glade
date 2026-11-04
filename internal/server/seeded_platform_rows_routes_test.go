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
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native API 62/67 counts: ../apextest/testdata/conformance/seeded_platform_rows/org.tsv:6-9.
// Supplemental host coverage; the core regressions preceded the product fix.
func TestSeededPlatformRowsHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeServerTestFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [MultiCurrency]\n")
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/SeededRowsHost.cls"), `public class SeededRowsHost {
                public String getOutcome() { return run(); }
                @AuraEnabled(cacheable=true) public static String run() {
                    Integer dated = [SELECT count() FROM DatedConversionRate];
                    Integer cache = [SELECT count() FROM PlatformCachePartition];
                    return String.valueOf(dated) + ':' + String.valueOf(cache);
                }
            }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/SeededRowsHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/SeededRows.page"), `<apex:page controller="SeededRowsHost" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/SeededRows.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><label>Seeded rows</label></ApexPage>`)
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
			org := storage.NewOrgState()
			org.APIVersion = api
			storage.ApplyOrgShape(&org, []string{"MultiCurrency"})
			handler := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, handler)
			handler.SetProjectIndex(typesys.Build(p, schema))
			if handler.runtimeErr != nil {
				t.Fatal(handler.runtimeErr)
			}
			for _, route := range []struct{ name, method, path, body string }{
				{"vf", http.MethodGet, "/apex/SeededRows", ""},
				{"lwc_wire", http.MethodPost, "/lightning/wire/apex", `{"className":"SeededRowsHost","method":"run","params":{}}`},
				{"lwc_imperative", http.MethodPost, "/lightning/apex/SeededRowsHost/run", `{}`},
				{"server_anonymous", http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", `{"anonymousBody":"System.assertEquals(0, [SELECT count() FROM DatedConversionRate]); System.assertEquals(0, [SELECT count() FROM PlatformCachePartition]);"}`},
			} {
				t.Run(route.name, func(t *testing.T) {
					for pass := 0; pass < 2; pass++ {
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
						req.Header.Set("Content-Type", "application/json")
						handler.ServeHTTP(rec, req)
						if rec.Code != http.StatusOK {
							t.Fatalf("pass %d: status=%d body=%s", pass, rec.Code, rec.Body.String())
						}
						switch route.name {
						case "vf":
							if !strings.Contains(rec.Body.String(), ">0:0<") {
								t.Fatalf("pass %d: VF body=%s", pass, rec.Body.String())
							}
						case "server_anonymous":
							var out struct {
								Success  bool `json:"success"`
								Compiled bool `json:"compiled"`
							}
							if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || !out.Success || !out.Compiled {
								t.Fatalf("pass %d: result=%+v err=%v body=%s", pass, out, err, rec.Body.String())
							}
						default:
							var out lwcbrowser.WireResponse
							if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Error != nil || out.Data != "0:0" {
								t.Fatalf("pass %d: LWC response=%+v err=%v", pass, out, err)
							}
						}
					}
				})
			}
		})
	}
}
