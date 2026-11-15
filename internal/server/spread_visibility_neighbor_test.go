package server

import (
	"encoding/json"
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

func TestSourceSpreadRESTDescribePreservesNeighbor(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
	}{
		{"default", []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape}},
		{"optional", []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.EnableStandardSpreadSource()
			storage.ApplyProjectOrgShape(&org, tc.features)
			storage.EnsureLazyStandardObject(&org, "CareProgram")
			handler := New(&org)
			for i := 0; i < 2; i++ {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/services/data/v68.0/sobjects/CareProgram/describe", nil))
				if response.Code != http.StatusNotFound {
					t.Fatalf("origin/main describe status 404, got %d", response.Code)
				}
			}
		})
	}
}

func TestSourceSpreadRESTDescribePreservesEditedNeighbor(t *testing.T) {
	org := storage.NewOrgState()
	org.EnableStandardSpreadSource()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := storage.EnsureLazyStandardObject(&org, "CareProgram"); !ok {
		t.Fatal("inventory object was not materialized")
	}
	object := org.Objects["CareProgram"]
	original := object.Definition.Fields["Id"]
	defer func() { object.Definition.Fields["Id"] = original }()
	edited := original
	edited.Label = "edited"
	object.Definition.Fields["Id"] = edited
	handler := New(&org)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/services/data/v68.0/sobjects/CareProgram/describe", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("edited definition status %d, want 200", response.Code)
	}
}

func TestSourceSpreadOtherHostRoutesPreserveNeighbor(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
	}{
		{"default", []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape}},
		{"optional", []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.EnableStandardSpreadSource()
			storage.ApplyProjectOrgShape(&org, tc.features)
			storage.EnsureLazyStandardObject(&org, "CareProgram")
			handler := New(&org)
			for _, route := range []struct {
				name, method, path, body string
				status                   int
			}{
				{"composite-retrieve", http.MethodGet, "/services/data/v68.0/composite/sobjects/CareProgram", "", http.StatusNotFound},
				{"composite-tree", http.MethodPost, "/services/data/v68.0/composite/tree/CareProgram", `{"records":[]}`, http.StatusNotFound},
				{"bulk", http.MethodPost, bulkV1TestPath + "/job", bulkV1TestJobInfo("insert", "CareProgram", ""), http.StatusNotFound},
			} {
				t.Run(route.name, func(t *testing.T) {
					response := httptest.NewRecorder()
					handler.ServeHTTP(response, httptest.NewRequest(route.method, route.path, strings.NewReader(route.body)))
					if response.Code != route.status {
						t.Fatalf("origin/main status %d, got %d", route.status, response.Code)
					}
				})
			}
			t.Run("partner-soap", func(t *testing.T) {
				response := partnerSOAPPost(handler, partnerSOAPEnvelope(`<urn:describeSObjects><urn:sObjectType>CareProgram</urn:sObjectType></urn:describeSObjects>`))
				if response.Code != http.StatusInternalServerError {
					t.Fatalf("origin/main SOAP status 500, got %d", response.Code)
				}
			})
		})
	}
}

func TestSourceSpreadVisualforceAndLightningPreserveNeighbor(t *testing.T) {
	root := t.TempDir()
	writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"68.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="SpreadVisibilityHost"></apex:page>`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/SpreadVisibilityHost.cls"), `public class SpreadVisibilityHost { @AuraEnabled @RemoteAction public static String observe() { try { Database.query('SELECT Id FROM CareProgram LIMIT 2'); return 'ok'; } catch (QueryException e) { return 'reject'; } } }`)
	writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/SpreadVisibilityHost.cls-meta.xml"), `<ApexClass><apiVersion>68.0</apiVersion></ApexClass>`)
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
	org.APIVersion = "68.0"
	org.EnableStandardSpreadSource()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	storage.EnsureLazyStandardObject(&org, "CareProgram")
	srv := newLightningTestServer(t, &org, source)
	configureVisualforceTestPrincipal(t, srv)
	srv.SetProjectIndex(typesys.Build(p, schema))
	if srv.runtimeErr != nil {
		t.Fatal(srv.runtimeErr)
	}
	t.Run("visualforce", func(t *testing.T) {
		responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"SpreadVisibilityHost","method":"observe","data":[],"type":"rpc","tid":3}]`)
		if len(responses) != 1 || !responses[0].Status || responses[0].Result != "reject" {
			t.Fatalf("origin/main remoting result changed: %+v", responses)
		}
	})
	t.Run("lightning", func(t *testing.T) {
		response := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"SpreadVisibilityHost","method":"observe","params":{}}`))
		req.Header.Set("Content-Type", "application/json")
		srv.ServeHTTP(response, req)
		var body struct {
			Data  any `json:"data"`
			Error any `json:"error"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusOK || body.Error != nil || body.Data != "reject" {
			t.Fatalf("origin/main wire result changed: status %d, body %s", response.Code, response.Body.String())
		}
	})
}
