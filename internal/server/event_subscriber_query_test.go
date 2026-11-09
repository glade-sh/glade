package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestEventSubscriberRESTQueryIdentity(t *testing.T) {
	// The server's supported REST versions do not include 62.0.
	for _, api := range []string{"65.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = api
			storage.EnsureStandardObject(&org, "EventBusSubscriber")
			state := org.Objects["EventBusSubscriber"]
			if state.Records == nil {
				state.Records = make(map[storage.ID]storage.Record)
			}
			id := storage.ID("0Cd000000000001")
			state.Records[id] = storage.Record{ID: id, Object: "EventBusSubscriber", MetadataSourceUnit: "glade:event-subscriber:01q000000000001", Fields: map[string]storage.Value{"Name": storage.StringValue("SyntheticEventTrigger"), "Topic": storage.StringValue("SyntheticEvent__e"), "Type": storage.StringValue("ApexTrigger")}}
			org.Objects["EventBusSubscriber"] = state
			handler := New(&org)
			query := url.QueryEscape("SELECT Id,Name FROM EventBusSubscriber WHERE Topic = 'SyntheticEvent__e'")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/services/data/v"+api+"/query?q="+query, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("query status %d: %s", response.Code, response.Body.String())
			}
			var body struct {
				Records []map[string]any `json:"records"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Records) != 1 || body.Records[0]["Id"] != "000000000000000AAA" {
				t.Fatalf("record identity: %#v", body.Records)
			}
			attrs, ok := body.Records[0]["attributes"].(map[string]any)
			if !ok || !strings.HasSuffix(fmt.Sprint(attrs["url"]), "/01q000000000001") {
				t.Fatalf("subscriber URL: %#v", body.Records[0]["attributes"])
			}
			byID := url.QueryEscape("SELECT Id FROM EventBusSubscriber WHERE Id = '000000000000000AAA'")
			response = httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/services/data/v"+api+"/query?q="+byID, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("ID query status %d: %s", response.Code, response.Body.String())
			}
			body.Records = nil
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || len(body.Records) != 0 {
				t.Fatalf("ID query: %s", response.Body.String())
			}
		})
	}
}
