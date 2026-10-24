package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestOrganizationGroupDataResetHTTPRoutes(t *testing.T) {
	for _, route := range []string{"path", "query", "body"} {
		t.Run(route, func(t *testing.T) {
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"Communities"})
			storage.EnsureDeterministicPlatformData(&org)
			org.Objects["Group"].Records["00G000000000099"] = storage.Record{ID: "00G000000000099", Object: "Group", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("MeadowReaders"), "Type": storage.StringValue("Regular")}}
			store := &memoryStore{}
			handler := NewWithStore(&org, store)
			path, body := serverTestDataPath+"/glade/reset/data", ""
			if route == "query" {
				path = serverTestDataPath + "/glade/reset?scope=data"
			}
			if route == "body" {
				path, body = serverTestDataPath+"/glade/reset", `{"scope":"data"}`
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
			if response.Code != http.StatusOK {
				t.Fatalf("reset status=%d: %s", response.Code, response.Body.String())
			}
			if store.saves != 1 {
				t.Fatalf("reset saves=%d", store.saves)
			}
			// The persisted reset must retain the same setup rows queried by Apex.
			for _, state := range []storage.OrgState{org, store.last} {
				if len(state.Objects["Group"].Records) != 3 {
					t.Fatalf("groups after data reset=%d, want 3 native organization groups", len(state.Objects["Group"].Records))
				}
				if _, ok := state.Objects["Group"].Records["00G000000000099"]; ok {
					t.Fatal("data reset kept regular group")
				}
			}
		})
	}
}
