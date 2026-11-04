package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// Explicit same-name assignment rejects in native Account updates:
// case349_mixed/org.tsv:6,13. REST and composite bodies are explicit writes.
func TestCase349PersonNameExplicitWriteRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, field := range []string{"PersonHomePhone", "PersonMobilePhone"} {
			for _, route := range []string{"rest", "composite"} {
				t.Run(api+"/"+field+"/"+route, func(t *testing.T) {
					org := testOrg()
					storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
					storage.EnsureStandardObject(&org, "Contact")
					engine := dml.NewEngine(&org)
					inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"LastName": storage.StringValue("Synthetic")}}})
					if len(inserted) != 1 || !inserted[0].Success {
						t.Fatal(inserted)
					}
					id := string(inserted[0].ID)
					path := "/services/data/v" + api + "/sobjects/Account/" + id
					body := fmt.Sprintf(`{"Name":"Synthetic",%q:"4155550123"}`, field)
					if route == "composite" {
						path = "/services/data/v" + api + "/composite/sobjects"
						body = fmt.Sprintf(`{"allOrNone":false,"records":[{"attributes":{"type":"Account"},"Id":%q,"Name":"Synthetic",%q:"4155550123"}]}`, id, field)
					}
					rec := httptest.NewRecorder()
					New(&org).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body)))
					if api == "62.0" {
						if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "unsupported API version") {
							t.Fatalf("version boundary: status=%d body=%s", rec.Code, rec.Body.String())
						}
						return
					}
					if !strings.Contains(rec.Body.String(), "INVALID_FIELD_FOR_INSERT_UPDATE") {
						t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
					}
				})
			}
		}
	}
}

// Queried-name provenance is transient: clones retain it but JSON does not.
func TestCase349PersonNameRequestProvenanceIsTransient(t *testing.T) {
	record, err := recordFromRawFields("Account", "001000000000101", map[string]json.RawMessage{"Name": json.RawMessage(`"Synthetic"`)})
	if err != nil {
		t.Fatal(err)
	}
	mark, ok := any(&record).(interface{ MarkDMLNameUnassigned() })
	if !ok {
		t.Fatal("queried-name provenance unavailable")
	}
	mark.MarkDMLNameUnassigned()
	marked, ok := any(record.Clone()).(interface{ DMLNameUnassigned() bool })
	if !ok || !marked.DMLNameUnassigned() {
		t.Fatal("queried-name provenance missing from clone")
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "dmlNameUnassigned") {
		t.Fatalf("provenance serialized: %s", data)
	}
}
