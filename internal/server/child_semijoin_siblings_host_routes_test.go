package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// NATIVE anonymous observations: ../apextest/testdata/conformance/child_semijoin_siblings/org.tsv:2-3.
func TestChildSemiJoinSiblingVisualforceAndLightningHosts(t *testing.T) {
	source, err := os.ReadFile("../apextest/testdata/conformance/child_semijoin_siblings/seeded.apex")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="JoinHost"></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/JoinHost.cls"), `public class JoinHost { @AuraEnabled @RemoteAction public static String observe() { `+string(source)+` return 'ok'; } }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/JoinHost.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			for _, name := range []string{"Account", "Contact", "Opportunity", "OpportunityContactRole", "Case"} {
				storage.EnsureStandardObject(&org, name)
			}
			srv := newLightningTestServer(t, &org, metadata)
			configureVisualforceTestPrincipal(t, srv)
			srv.SetProjectIndex(typesys.Build(p, s))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			t.Run("vf", func(t *testing.T) {
				responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"JoinHost","method":"observe","data":[],"type":"rpc","tid":3}]`)
				if len(responses) != 1 || !responses[0].Status || responses[0].Result != "ok" {
					t.Fatalf("remoting: %+v", responses)
				}
			})
			t.Run("lwc", func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"JoinHost","method":"observe","params":{}}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				var body struct {
					Data  any `json:"data"`
					Error any `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK || body.Error != nil || body.Data != "ok" {
					t.Fatalf("wire: %d %s; error %v", rec.Code, rec.Body.String(), err)
				}
			})
		})
	}
}
