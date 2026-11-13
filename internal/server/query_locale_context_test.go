package server

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// Local host contract: REST and Bulk query adapters pass the selected user's
// locale into the shared SOQL executor. Native text rows supply the comparator;
// this is not a claim of native HTTP transport capture.
func TestQueryRoutesKeepUserTextLocale(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	const actor storage.ID = "005000000000002"
	addUser(&org, actor, "synthetic@example.invalid", "synthetic@example.invalid", "Synthetic User")
	addAccountForTest(&org, "001000000000001", "_")
	addAccountForTest(&org, "001000000000002", "9")
	handler := New(&org)
	for _, locale := range []string{"en_US", "de_DE", "en_GB", "en_US"} {
		users := org.Objects["User"]
		user := users.Records[actor]
		user.Fields["LocaleSidKey"] = storage.StringValue(locale)
		users.Records[actor] = user
		org.Objects["User"] = users
		for _, query := range []string{"SELECT Name FROM Account ORDER BY Name", "SELECT Name FROM Account WHERE Name > '9' ORDER BY Name"} {
			parsed, err := soql.Parse(query)
			if err != nil {
				t.Fatal(err)
			}
			direct, err := soql.ExecuteWithCacheAndContext(org, parsed, nil, soql.ExecutionContext{Locale: locale, SetupUserID: actor})
			if err != nil {
				t.Fatal(err)
			}
			want := []string{}
			for _, record := range direct.Records {
				want = append(want, record.Fields["Name"].String)
			}
			req := httptest.NewRequest(http.MethodGet, serverTestDataPath+"/query?q="+url.QueryEscape(query), nil)
			req.Header.Set("X-GLADE-User-Id", string(actor))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			var payload struct {
				Records []struct {
					Name string `json:"Name"`
				} `json:"records"`
			}
			if rec.Code != http.StatusOK {
				t.Fatal(rec.Code, rec.Body.String())
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			got := []string{}
			for _, record := range payload.Records {
				got = append(got, record.Name)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("REST %s %s: got %v, shared executor %v", locale, query, got, want)
			}
			body, err := json.Marshal(map[string]string{"operation": "query", "query": query})
			if err != nil {
				t.Fatal(err)
			}
			req = httptest.NewRequest(http.MethodPost, serverTestDataPath+"/jobs/query", strings.NewReader(string(body)))
			req.Header.Set("X-GLADE-User-Id", string(actor))
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			var job struct {
				ID string `json:"id"`
			}
			if rec.Code != http.StatusOK {
				t.Fatal(rec.Code, rec.Body.String())
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
				t.Fatal(err)
			}
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, serverTestDataPath+"/jobs/query/"+job.ID+"/results", nil))
			if rec.Code != http.StatusOK {
				t.Fatal(rec.Code, rec.Body.String())
			}
			rows, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			got = []string{}
			for _, row := range rows[1:] {
				got = append(got, row[0])
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Bulk %s %s: got %v, shared executor %v", locale, query, got, want)
			}
		}
	}
}
