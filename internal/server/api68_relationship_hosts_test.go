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

// Native anonymous outcomes: platform_relationships/api68_boundary/org.tsv:2-7.
// The hosts transport the same versioned Apex query without changing its semantics.
func TestAPI68RelationshipVisualforceAndLightningHosts(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="RelationshipHost"></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/RelationshipHost.cls"), `public class RelationshipHost { @AuraEnabled @RemoteAction public static String observe() { try { Database.query('SELECT Id,(SELECT Id FROM ContactPointOttSvcs LIMIT 1) FROM Account LIMIT 1'); return 'ok'; } catch (QueryException e) { return 'reject'; } } }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/RelationshipHost.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
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
			storage.EnsureStandardObject(&org, "Account")
			srv := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, srv)
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			want := "reject"
			if api == "68.0" {
				want = "ok"
			}
			t.Run("vf", func(t *testing.T) {
				for i := 0; i < 2; i++ {
					responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"RelationshipHost","method":"observe","data":[],"type":"rpc","tid":3}]`)
					if len(responses) != 1 || !responses[0].Status || responses[0].Result != want {
						t.Fatalf("remoting: %+v, want %q", responses, want)
					}
				}
			})
			t.Run("lwc", func(t *testing.T) {
				for i := 0; i < 2; i++ {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"RelationshipHost","method":"observe","params":{}}`))
					req.Header.Set("Content-Type", "application/json")
					srv.ServeHTTP(rec, req)
					var body struct {
						Data  any `json:"data"`
						Error any `json:"error"`
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK || body.Error != nil || body.Data != want {
						t.Fatalf("wire: %d %s, want %q, error %v", rec.Code, rec.Body.String(), want, err)
					}
				}
			})
		})
	}
}
