package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// The Apex body uses the captured named Date-null and valid-Date observations.
func TestJSONDateAPI68HostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="DateProbeHost"></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/DateProbeHost.cls"), `public class DateProbeHost { @AuraEnabled @RemoteAction public static String run() { String s='returned'; try { JSON.deserialize('null',Date.class); } catch(Exception e) { s=e.getTypeName()+'|'+e.getMessage(); } return s+'|'+String.valueOf((Date)JSON.deserialize('"2026-11-03"',Date.class)); } }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/DateProbeHost.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			srv := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, srv)
			srv.SetProjectIndex(typesys.Build(p, sch))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			const want = `System.JSONException|Invalid format: "null"|2026-11-03`
			for i := 0; i < 2; i++ {
				responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"DateProbeHost","method":"run","data":[],"type":"rpc","tid":3}]`)
				if len(responses) != 1 || !responses[0].Status || responses[0].Result != want {
					t.Fatalf("VF request %d: %+v", i, responses)
				}
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"DateProbeHost","method":"run","params":{}}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				var body struct {
					Data  any `json:"data"`
					Error any `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK || body.Error != nil || body.Data != want {
					t.Fatalf("LWC request %d: %s: %v", i, rec.Body.String(), err)
				}
				if api != "62.0" {
					payload, err := json.Marshal(map[string]string{"anonymousBody": `String s='returned'; try { JSON.deserialize('null', Date.class); } catch(Exception e) { s=e.getTypeName()+'|'+e.getMessage(); } System.assertEquals('System.JSONException|Invalid format: "null"',s); System.assertEquals('2026-11-03',String.valueOf((Date)JSON.deserialize('"2026-11-03"',Date.class)));`})
					if err != nil {
						t.Fatal(err)
					}
					resp := httptest.NewRecorder()
					srv.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(payload))))
					if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"success":true`) {
						t.Fatalf("server request %d: %s", i, resp.Body.String())
					}
				}
			}
		})
	}
}
