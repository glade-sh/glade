package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native field facts are in standard_industries_schema/org.tsv:29-31,41-43.
func TestStandardIndustriesSchemaDescribeRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		org := storage.NewOrgState()
		org.APIVersion = api
		org.Features = []string{"ContactsToMultipleAccounts", "B2BCommerce", "Communities", "CommerceEnabled", "MiddleNameEnabled"}
		handler := New(&org)
		for _, row := range []struct{ object, field, typ string }{{"AccountContactRelation", "Roles", "multipicklist"}, {"PricebookEntry", "ProductSellingModelId", "reference"}, {"User", "MiddleName", "string"}} {
			t.Run(api+"/"+row.object+"."+row.field, func(t *testing.T) {
				storage.EnsureStandardObject(&org, row.object)
				for _, attempt := range []string{"cold", "warm"} {
					t.Run(attempt, func(t *testing.T) {
						rec := httptest.NewRecorder()
						handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/sobjects/"+row.object+"/describe", nil))
						if rec.Code != http.StatusOK {
							t.Fatalf("describe status=%d body=%s", rec.Code, rec.Body.String())
						}
						var payload struct {
							Fields []struct {
								Name string `json:"name"`
								Type string `json:"type"`
							} `json:"fields"`
						}
						if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
							t.Fatal(err)
						}
						found := false
						for _, field := range payload.Fields {
							if field.Name == row.field {
								found = true
								if !strings.EqualFold(field.Type, row.typ) {
									t.Errorf("type=%q, native %q", field.Type, row.typ)
								}
							}
						}
						if !found {
							t.Fatalf("field %s missing", row.field)
						}
					})
				}
			})
		}
	}
}

func TestLightningIndustriesSchemaFields(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		org := storage.NewOrgState()
		org.APIVersion = api
		org.Features = []string{"ContactsToMultipleAccounts", "B2BCommerce", "Communities", "CommerceEnabled", "MiddleNameEnabled"}
		handler := New(&org)
		for _, row := range []struct{ object, field, typ string }{{"AccountContactRelation", "Roles", "Multipicklist"}, {"PricebookEntry", "ProductSellingModelId", "Reference"}, {"User", "MiddleName", "String"}} {
			t.Run(api+"/"+row.object, func(t *testing.T) {
				storage.EnsureStandardObject(&org, row.object)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/lightning/wire/getObjectInfo", strings.NewReader(`{"objectApiName":"`+row.object+`"}`)))
				if rec.Code != http.StatusOK {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
				var out struct {
					Data struct {
						Fields map[string]struct {
							DataType string `json:"dataType"`
						} `json:"fields"`
					} `json:"data"`
					Error any `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				if out.Error != nil {
					t.Fatalf("wire error=%v", out.Error)
				}
				f, ok := out.Data.Fields[row.field]
				if !ok || f.DataType != row.typ {
					t.Fatalf("field %s=%+v, present=%v, want %s", row.field, f, ok, row.typ)
				}
			})
		}
	}
}
