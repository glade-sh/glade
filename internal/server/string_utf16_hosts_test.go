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
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// These host routes preserve the VM's existing UTF-16 behavior at both floors.
func TestStringUTF16IndexedHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="Utf16Controller"></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/Utf16Controller.cls"), `public class Utf16Controller { @AuraEnabled @RemoteAction public static String value() { String s='A😀z'; return String.valueOf(s.charAt(1)) + ':' + String.valueOf(s.codePointAt(1)) + ':' + String.valueOf(s.codePointBefore(3)); } }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/Utf16Controller.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
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
			srv := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, srv)
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			t.Run("vf", func(t *testing.T) {
				responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"Utf16Controller","method":"value","data":[],"type":"rpc","tid":3}]`)
				if len(responses) != 1 || !responses[0].Status {
					t.Fatalf("remoting: %+v", responses)
				}
				value, err := json.Marshal(responses[0].Result)
				if err != nil || string(value) != `"55357:128512:128512"` {
					t.Fatalf("remoting result %s: %v", value, err)
				}
			})
			t.Run("lwc", func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"Utf16Controller","method":"value","params":{}}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				var body struct {
					Data  any `json:"data"`
					Error any `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); rec.Code != http.StatusOK || err != nil || body.Error != nil || body.Data != "55357:128512:128512" {
					t.Fatalf("wire status %d: %s, %v", rec.Code, rec.Body.String(), err)
				}
			})
		})
	}
}
