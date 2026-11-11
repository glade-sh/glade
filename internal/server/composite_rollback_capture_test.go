package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// assertUpsertRows matches the captured response, normalizing only a newly
// generated ID. The existing-record ID remains an exact assertion.
func assertUpsertRows(t *testing.T, got, want []map[string]any, generatedID bool) {
	t.Helper()
	if generatedID {
		id, ok := got[0]["id"].(string)
		if !ok || id == "" {
			t.Fatalf("generated ID missing: %#v", got[0])
		}
		got[0]["id"] = "<ID>"
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("upsert response = %#v, want %#v", got, want)
	}
}

func nativeUpsertRows(all, newRecord bool) []map[string]any {
	first := map[string]any{"created": newRecord, "success": !all, "errors": []any{}}
	if all {
		first["errors"] = []any{map[string]any{
			"fields": []any{}, "message": "Record rolled back because not all records were valid and the request was using AllOrNone header",
			"statusCode": "ALL_OR_NONE_OPERATION_ROLLED_BACK",
		}}
	}
	if !newRecord {
		first["id"] = "001000000000001"
	} else if !all {
		first["id"] = "<ID>"
	}
	return []map[string]any{first, {
		"created": true, "success": false,
		"errors": []any{map[string]any{"fields": []any{"Name"}, "message": "Required fields are missing: [Name]", "statusCode": "REQUIRED_FIELD_MISSING"}},
	}}
}

// These cases replay synthetic Salesforce REST observations at APIs 62.0 and 67.0.
// Native rows: testdata/conformance/composite_rollback_routes/org.tsv:2-13.
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
					// Native rows: org.tsv:2-3,6-7; the existing update keeps its ID.
					assertUpsertRows(t, rows, nativeUpsertRows(all, false), false)
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

// Native rows: testdata/conformance/composite_rollback_routes/org.tsv:18-19 and :22-23.
func TestCompositeUpsertNewInsertRollbackNative(t *testing.T) {
	for _, all := range []bool{true, false} {
		t.Run(fmt.Sprint(all), func(t *testing.T) {
			org := testOrg()
			body := fmt.Sprintf(`{"allOrNone":%t,"records":[{"attributes":{"type":"Account"},"Name":"Valid","External_Id__c":"good"},{"attributes":{"type":"Account"},"External_Id__c":"bad"}]}`, all)
			rec := httptest.NewRecorder()
			New(&org).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, serverTestDataPath+"/composite/sobjects/Account/External_Id__c", strings.NewReader(body)))
			var rows []map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil || rec.Code != http.StatusOK || len(rows) != 2 {
				t.Fatalf("status %d body %s: %v", rec.Code, rec.Body.String(), err)
			}
			// Native rows: org.tsv:18-19,22-23; only a successful new ID is normalized.
			assertUpsertRows(t, rows, nativeUpsertRows(all, true), !all)
			want := 1
			if all {
				want = 0
			}
			if got := len(org.Objects["Account"].Records); got != want {
				t.Fatalf("stored records = %d, want %d", got, want)
			}
		})
	}
}

// Native rows: testdata/conformance/composite_rollback_routes/org.tsv:20-21 and :24-25.
func TestCompositeTreeMultipleFailuresNative(t *testing.T) {
	for _, good := range []bool{true, false} {
		t.Run(fmt.Sprint(good), func(t *testing.T) {
			org := testOrg()
			first := ""
			if good {
				first = `{"attributes":{"type":"Account","referenceId":"good"},"Name":"Valid"},`
			}
			body := `{"records":[` + first + `{"attributes":{"type":"Account","referenceId":"bad1"}},{"attributes":{"type":"Account","referenceId":"bad2"}}]}`
			rec := httptest.NewRecorder()
			New(&org).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, serverTestDataPath+"/composite/tree/Account", strings.NewReader(body)))
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
			if rec.Code != http.StatusBadRequest || !response.HasErrors || len(response.Results) != 2 {
				t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
			}
			for i, want := range []string{"bad1", "bad2"} {
				if response.Results[i].ReferenceID != want || len(response.Results[i].Errors) != 1 || response.Results[i].Errors[0].StatusCode != "REQUIRED_FIELD_MISSING" {
					t.Fatalf("result %d = %#v", i, response.Results[i])
				}
			}
			if got := len(org.Objects["Account"].Records); got != 0 {
				t.Fatalf("tree left %d accounts", got)
			}
		})
	}
}
