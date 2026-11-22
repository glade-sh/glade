package server

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE ../vm/testdata/conformance/hybrid_locale_tokens/fr/org.tsv:3 anonymous 67.0
// NATIVE ../vm/testdata/conformance/hybrid_locale_tokens/de/org.tsv:3 anonymous 67.0
func TestHybridLocaleAnonymousHost(t *testing.T) {
	for _, tc := range []struct{ fixture, locale, language string }{{"fr", "fr_FR", "fr"}, {"de", "de_DE", "de"}} {
		for _, api := range []string{"67.0"} {
			t.Run(tc.fixture+"/"+api, func(t *testing.T) {
				dir := filepath.Join("..", "vm", "testdata", "conformance", "hybrid_locale_tokens", tc.fixture)
				f, err := os.Open(filepath.Join(dir, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				r := csv.NewReader(f)
				r.Comma = '\t'
				rows, err := r.ReadAll()
				if err != nil {
					t.Fatal(err)
				}
				want := ""
				for _, row := range rows[1:] {
					if row[1] == api && row[2] == "anonymous" {
						want = strings.TrimPrefix(html.UnescapeString(row[5]), "OBS=")
					}
				}
				if want == "" {
					t.Fatal("missing native observation")
				}
				source, err := os.ReadFile(filepath.Join(dir, "observe.apex"))
				if err != nil {
					t.Fatal(err)
				}
				body := strings.Replace(string(source), "System.debug('OBS=' + ", "System.assertEquals('"+want+"', ", 1)
				org := testOrg()
				storage.EnsureDeterministicPlatformData(&org)
				users := org.Objects["User"]
				user := users.Records[storage.DefaultUserID]
				user.Fields["LocaleSidKey"] = storage.StringValue(tc.locale)
				user.Fields["LanguageLocaleKey"] = storage.StringValue(tc.language)
				user.Fields["TimeZoneSidKey"] = storage.StringValue("GMT")
				users.Records[storage.DefaultUserID] = user
				org.Objects["User"] = users
				s := New(&org)
				token := case340OwnedTokenFor(t, s, string(storage.DefaultUserID))
				payload, err := json.Marshal(map[string]string{"anonymousBody": body})
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", bytes.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)
				rec := httptest.NewRecorder()
				s.ServeHTTP(rec, req)
				var result struct {
					Compiled bool `json:"compiled"`
					Success  bool `json:"success"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || rec.Code != http.StatusOK || !result.Compiled || !result.Success {
					t.Fatalf("status %d, response %s, error %v", rec.Code, rec.Body.String(), err)
				}
			})
		}
	}
}
