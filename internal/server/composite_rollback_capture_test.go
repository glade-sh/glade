package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These cases replay synthetic Salesforce REST observations at APIs 62.0 and 67.0.
func TestCompositeRollbackResponseNativeRoutes(t *testing.T) {
	// The public server exposes v67.0; the native 62.0 rows matched 67.0.
	for _, api := range []string{"67.0"} {
		base := "/services/data/v" + api
		t.Run(api+"/delete", func(t *testing.T) {
			for _, all := range []bool{true, false} {
				t.Run(fmt.Sprint(all), func(t *testing.T) {
					org := testOrg()
					addAccountForTest(&org, "001000000000001", "Existing")
					rec := httptest.NewRecorder()
					path := fmt.Sprintf("%s/composite/sobjects?ids=001000000000001,001000000000999&allOrNone=%t", base, all)
					New(&org).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, path, nil))
					if rec.Code != http.StatusOK {
						t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
					}
					var rows []struct {
						Success bool `json:"success"`
						Errors  []struct {
							StatusCode string `json:"statusCode"`
							Message    string `json:"message"`
						} `json:"errors"`
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil || len(rows) != 2 {
						t.Fatalf("rows %s: %v", rec.Body.String(), err)
					}
					if len(rows[1].Errors) != 1 || rows[1].Errors[0].StatusCode != "INVALID_CROSS_REFERENCE_KEY" || rows[1].Errors[0].Message != "invalid cross reference id" {
						t.Fatalf("missing ID = %#v", rows[1])
					}
					if rows[0].Success == all || rows[1].Success {
						t.Fatalf("rows = %#v", rows)
					}
					if all && (len(rows[0].Errors) != 1 || rows[0].Errors[0].StatusCode != "ALL_OR_NONE_OPERATION_ROLLED_BACK" || rows[0].Errors[0].Message != "Record rolled back because not all records were valid and the request was using AllOrNone header") {
						t.Fatalf("rollback = %#v", rows[0])
					}
					if got := org.Objects["Account"].Records["001000000000001"].System.IsDeleted; got == all {
						t.Fatalf("deleted = %t with allOrNone=%t", got, all)
					}
				})
			}
		})
		t.Run(api+"/upsert", func(t *testing.T) {
			for _, all := range []bool{true, false} {
				t.Run(fmt.Sprint(all), func(t *testing.T) {
					org := testOrg()
					addAccountForTest(&org, "001000000000001", "Existing")
					account := org.Objects["Account"].Records["001000000000001"]
					account.Fields["External_Id__c"] = storage.StringValue("existing")
					org.Objects["Account"].Records["001000000000001"] = account
					body := fmt.Sprintf(`{"allOrNone":%t,"records":[{"attributes":{"type":"Account"},"Name":"Updated","External_Id__c":"existing"},{"attributes":{"type":"Account"},"External_Id__c":"missing"}]}`, all)
					rec := httptest.NewRecorder()
					New(&org).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, base+"/composite/sobjects/Account/External_Id__c", strings.NewReader(body)))
					if rec.Code != http.StatusOK {
						t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
					}
					var rows []map[string]any
					if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil || len(rows) != 2 {
						t.Fatalf("rows %s: %v", rec.Body.String(), err)
					}
					if rows[0]["success"] != !all || rows[0]["created"] != false || rows[1]["success"] != false || rows[1]["created"] != true {
						t.Fatalf("rows = %#v", rows)
					}
					if _, ok := rows[1]["id"]; ok {
						t.Fatalf("failed insert contains id: %#v", rows[1])
					}
					want := "Updated"
					if all {
						want = "Existing"
					}
					if got := org.Objects["Account"].Records["001000000000001"].Fields["Name"].String; got != want {
						t.Fatalf("name = %q, want %q", got, want)
					}
				})
			}
		})
		t.Run(api+"/tree", func(t *testing.T) {
			org := testOrg()
			rec := httptest.NewRecorder()
			body := `{"records":[{"attributes":{"type":"Account","referenceId":"one"},"Name":"Valid"},{"attributes":{"type":"Account","referenceId":"two"}}]}`
			New(&org).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, base+"/composite/tree/Account", strings.NewReader(body)))
			var response struct {
				HasErrors bool `json:"hasErrors"`
				Results   []struct {
					ReferenceID string `json:"referenceId"`
					Errors      []struct {
						StatusCode string `json:"statusCode"`
					} `json:"errors"`
				} `json:"results"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusBadRequest || !response.HasErrors || len(response.Results) != 1 || response.Results[0].ReferenceID != "two" || len(response.Results[0].Errors) != 1 || response.Results[0].Errors[0].StatusCode != "REQUIRED_FIELD_MISSING" {
				t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
			}
			if got := len(org.Objects["Account"].Records); got != 0 {
				t.Fatalf("tree left %d accounts", got)
			}
		})
	}
}

func TestCompositeDeleteStoredIDVariantsRemainDeleted(t *testing.T) {
	org := testOrg()
	const original = "001000000000001"
	addAccountForTest(&org, original, "Already Deleted")
	record := org.Objects["Account"].Records[original]
	record.System.IsDeleted = true
	org.Objects["Account"].Records[original] = record
	id18 := storage.ID18(original)
	rec := httptest.NewRecorder()
	New(&org).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, serverTestDataPath+"/composite/sobjects?ids="+id18+"&allOrNone=false", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"statusCode":"ENTITY_IS_DELETED"`) || !strings.Contains(rec.Body.String(), `"message":"entity is deleted"`) {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
}
