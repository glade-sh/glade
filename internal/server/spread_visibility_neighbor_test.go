package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
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
