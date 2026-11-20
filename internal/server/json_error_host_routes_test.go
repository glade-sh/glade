package server

import (
	"encoding/json"
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

func TestJSONMissingValueHostRoutes(t *testing.T) {
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"68.0"}`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="JSONErrorHost"></apex:page>`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/JSONErrorHost.cls"), `public class JSONErrorHost { @AuraEnabled @RemoteAction public static String run() { try { JSON.deserializeUntyped('{"a":}'); } catch (Exception e) { return e.getTypeName()+'|'+e.getMessage(); } return 'accepted'; } }`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/JSONErrorHost.cls-meta.xml"), `<ApexClass><apiVersion>68.0</apiVersion></ApexClass>`)
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
	org.APIVersion = "68.0"
	srv := newLightningTestServer(t, &org, source)
	configureVisualforceTestPrincipal(t, srv)
	srv.SetProjectIndex(typesys.Build(p, sch))
	if srv.runtimeErr != nil {
		t.Fatal(srv.runtimeErr)
	}
	const want = "System.JSONException|Unexpected character ('}' (code 125)): expected a value at [line:1, column:7]"
	responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"JSONErrorHost","method":"run","data":[],"type":"rpc","tid":3}]`)
	if len(responses) != 1 || !responses[0].Status || responses[0].Result != want {
		t.Fatalf("Visualforce response: %+v", responses)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"JSONErrorHost","method":"run","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	var body struct {
		Data  any `json:"data"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK || body.Error != nil || body.Data != want {
		t.Fatalf("LWC response: %s: %v", rec.Body.String(), err)
	}
	payload, err := json.Marshal(map[string]string{"anonymousBody": `try { JSON.deserializeUntyped('{"a":}'); System.assert(false); } catch (JSONException e) { System.assertEquals('Unexpected character (\'}\' (code 125)): expected a value at [line:1, column:7]', e.getMessage()); }`})
	if err != nil {
		t.Fatal(err)
	}
	resp := httptest.NewRecorder()
	srv.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/services/data/v68.0/tooling/executeAnonymous", strings.NewReader(string(payload))))
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"success":true`) {
		t.Fatalf("anonymous response: %s", resp.Body.String())
	}
}
