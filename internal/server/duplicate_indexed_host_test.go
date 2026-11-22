package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/storage"
)

func TestDuplicateIndexedHostBulk(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"lightning", "visualforce"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				srv := upsertHostServer(t, api)
				object := srv.Org.Objects["Contact"]
				match := storage.MatchingRule{Object: "Contact", Name: "Name", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "LastName", Method: "LastName", BlankBehavior: "NullNotAllowed"}}}
				object.Definition.DuplicateRules = []storage.DuplicateRule{{Name: "Name", Object: "Contact", Active: true, ActionOnInsert: "Block", MatchRules: []storage.MatchingRule{match}}}
				for i := 0; i < 300; i++ {
					id := storage.ID(fmt.Sprintf("003%012d", i+1))
					object.PutRecord(id, storage.Record{ID: id, Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue(fmt.Sprintf("%d", i))}})
				}
				srv.Org.Objects["Contact"] = object
				var output string
				if route == "lightning" {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, "/lightning/apex/AjaxController/lightningIndexedBulk", strings.NewReader(`{}`))
					req.Header.Set("Content-Type", "application/json")
					srv.ServeHTTP(rec, req)
					var response lwcbrowser.WireResponse
					if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || rec.Code != http.StatusOK || response.Error != nil {
						t.Fatalf("response %d: %s (%v)", rec.Code, rec.Body.String(), err)
					}
					var ok bool
					output, ok = response.Data.(string)
					if !ok {
						t.Fatalf("response: %+v", response)
					}
				} else {
					response := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"remoteIndexedBulk","data":[],"type":"rpc","tid":1}]`)
					if len(response) != 1 || !response[0].Status {
						t.Fatalf("response: %+v", response)
					}
					var ok bool
					output, ok = response[0].Result.(string)
					if !ok {
						t.Fatalf("response: %+v", response)
					}
				}
				if !strings.Contains(output, "DUPLICATES_DETECTED") || !strings.Contains(output, `"matchConfidence":100.0`) {
					t.Fatalf("missing expected matches or errors: %s", output)
				}
				// Digest of the entire serialized result on origin/main, including
				// duplicate reports and every save error.
				if got := fmt.Sprintf("%x", sha256.Sum256([]byte(output))); got != "47bb68c84b80daa10800434974038b5b1e711166c148bbcdae32aa87a079b899" {
					t.Fatalf("parent outcome differs (%s): %s", got, output)
				}
			})
		}
	}
}
