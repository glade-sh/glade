package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCase340FixtureIdentityReplacementRevokesTokens(t *testing.T) {
	for _, change := range []string{"org", "identical-user"} {
		t.Run(change, func(t *testing.T) {
			t.Setenv("GLADE_SERVER_CAPABILITY", "")
			org := testOrg()
			storage.EnsureDeterministicPlatformData(&org)
			id := storage.ID("005000000000123")
			users := org.Objects["User"]
			users.Records[id] = storage.Record{ID: id, Object: "User", Fields: map[string]storage.Value{"LastName": storage.StringValue("Synthetic")}, ExplicitNulls: map[string]bool{}}
			org.Objects["User"] = users
			s := New(&org)
			other := case340OwnedToken(t, s)
			token := case340OwnedTokenFor(t, s, string(id))
			fixture := storage.Fixture{Version: storage.FixtureVersion}
			if change == "org" {
				fixture.Org.OrgID = "synthetic-other-org"
			} else {
				fixture.Objects = []storage.FixtureObject{{Name: "User", Records: []storage.FixtureRecord{{ID: id, Fields: users.Records[id].Fields}}}}
			}
			body, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			h := GuardHTTPAuthority(s, "127.0.0.1:1789")
			req := httptest.NewRequest("POST", "http://localhost:1789/services/data/v67.0/glade/fixture?mode=merge", strings.NewReader(string(body)))
			req.Header.Set("X-Glade-Client", "1")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("fixture status %d: %s", rec.Code, rec.Body.String())
			}
			if _, ok := issuedTokenUser(token); ok {
				t.Error("identity replacement retained issued token")
			}
			if _, ok := issuedTokenUser(other); ok != (change == "identical-user") {
				t.Errorf("unrelated user token retained = %t, change = %s", ok, change)
			}
		})
	}
}
