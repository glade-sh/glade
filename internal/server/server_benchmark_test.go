package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func BenchmarkServerQueryRoute(b *testing.B) {
	org := benchmarkServerOrg(1000)
	handler := New(&org)
	request := httptest.NewRequest(http.MethodGet, "/services/data/v"+storage.DefaultRESTAPIVersion+"/query?q=SELECT%20Id,%20Name%20FROM%20Account%20WHERE%20Name%20=%20'Account%200500'", nil)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			b.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
		}
	}
}

func benchmarkServerOrg(records int) storage.OrgState {
	org := testOrg()
	object := org.Objects["Account"]
	for i := 0; i < records; i++ {
		id := storage.ID(fmt.Sprintf("001%012d", i+1))
		object.Records[id] = storage.Record{
			ID:     id,
			Object: "Account",
			Fields: map[string]storage.Value{
				"Name": storage.StringValue(fmt.Sprintf("Account %04d", i)),
			},
		}
	}
	org.Objects["Account"] = object
	return org
}
