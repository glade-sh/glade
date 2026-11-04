package playground

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func case339DatabaseOrg(value storage.Value) storage.OrgState {
	return storage.OrgState{Objects: map[string]storage.ObjectState{
		"Sample__c": {Definition: storage.ObjectDefinition{APIName: "Sample__c", Label: "Sample"}, Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Fields: map[string]storage.Value{"Text__c": value}},
		}},
	}}
}

func TestCase339ReviewPublicDatabaseOutputBudget(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value storage.Value
		want  int
	}{
		{"small", storage.StringValue("visible"), http.StatusOK},
		{"near limit", storage.StringValue(strings.Repeat("x", 100000)), http.StatusOK},
		{"large string", storage.StringValue(strings.Repeat("x", 2*maxPublicDatabaseBytes)), http.StatusRequestEntityTooLarge},
		{"unknown value fallback", storage.Value{Kind: "unknown", String: strings.Repeat("x", 2*maxPublicDatabaseBytes)}, http.StatusRequestEntityTooLarge},
		{"escaped string", storage.StringValue(strings.Repeat("<", maxPublicDatabaseBytes/2)), http.StatusRequestEntityTooLarge},
		{"nested list", storage.Value{Kind: storage.ValueList, List: []storage.Value{storage.StringValue(strings.Repeat("z", 2*maxPublicDatabaseBytes))}}, http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			s := NewServer(ws, ServerOptions{Public: true, RatePerMinute: 1000})
			defer s.Close()
			rec := httptest.NewRecorder()
			s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, "/playground/", nil))
			cookies := rec.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatal("missing session")
			}
			org := case339DatabaseOrg(tc.value)
			s.sessions.items[cookies[0].Value].server.runner.lastOrg = &org
			req := httptest.NewRequest(http.MethodGet, "/playground/api/database", nil)
			req.AddCookie(cookies[0])
			rec = httptest.NewRecorder()
			s.ServeHTTP(case339ResponseWriter{rec}, req)
			if rec.Code != tc.want || rec.Body.Len() > maxPublicDatabaseBytes {
				t.Fatalf("status=%d bytes=%d want=%d", rec.Code, rec.Body.Len(), tc.want)
			}
			if tc.name == "small" && !strings.Contains(rec.Body.String(), "visible") {
				t.Fatalf("normal database missing: %s", rec.Body.String())
			}
		})
	}
}

func TestCase339ReviewPublicDatabaseRateLimit(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RatePerMinute: 1})
	defer s.Close()
	rec := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, "/playground/", nil))
	cookie := rec.Result().Cookies()[0]
	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		req := httptest.NewRequest(http.MethodGet, "/playground/api/database", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		s.ServeHTTP(case339ResponseWriter{rec}, req)
		if rec.Code != want {
			t.Fatalf("request %d: %d want %d", i, rec.Code, want)
		}
	}
}

func TestCase339ReviewLocalDatabaseRemainsUnbounded(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{})
	defer s.Close()
	org := case339DatabaseOrg(storage.StringValue(strings.Repeat("x", 2*maxPublicDatabaseBytes)))
	s.runner.lastOrg = &org
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/playground/api/database", nil))
	if rec.Code != http.StatusOK || rec.Body.Len() <= maxPublicDatabaseBytes {
		t.Fatalf("local snapshot: %d bytes=%d", rec.Code, rec.Body.Len())
	}
}

func TestCase339ReviewDatabaseBudgetRejectsUnknownValueBeforeMaterializing(t *testing.T) {
	org := case339DatabaseOrg(storage.Value{Kind: "unknown", String: strings.Repeat("x", 2*maxPublicDatabaseBytes)})
	if _, ok := boundedDatabaseSnapshot(org); ok {
		t.Fatal("unknown value was materialized before the output budget rejected it")
	}
}

// A run loads the whole standard catalog into the visitor org. The public
// snapshot must still return the visitor's data, omit empty standard objects,
// and stay bounded.
func TestCase339PublicDatabaseAfterRunReturnsVisitorData(t *testing.T) {
	ws, err := OpenWorkspace(WorkspaceOptions{DataRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(ws, ServerOptions{Public: true, RatePerMinute: 1000})
	defer s.Close()
	rec := httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, httptest.NewRequest(http.MethodGet, "/playground/", nil))
	cookie := rec.Result().Cookies()[0]
	body := strings.NewReader(`{"anonymousBody":"insert new Account(Name='AfterRunMarker');"}`)
	req := httptest.NewRequest(http.MethodPost, "/playground/api/run", body)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"pass"`) {
		t.Fatalf("run: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/playground/api/database", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	s.ServeHTTP(case339ResponseWriter{rec}, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "AfterRunMarker") {
		t.Fatalf("database after run: %d %.200s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() > 512*1024 {
		t.Fatalf("database snapshot %d bytes exceeds the public database budget", rec.Body.Len())
	}
	if strings.Contains(rec.Body.String(), `"name":"AccountContactRole"`) {
		t.Fatal("empty standard object listed in the public snapshot")
	}
}
