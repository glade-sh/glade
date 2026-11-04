package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Native compile diagnostics: namespace_diagnostics/qualified_locals/org.tsv
// rows N005-N008 at source API 67. REST API 62 is not served locally.
// The static receiver control is in
// namespace_diagnostics/org.tsv.
func TestQualifiedNamespaceLocalToolingServer(t *testing.T) {
	for _, api := range []string{"67.0"} {
		for _, tc := range []struct{ source, want string }{
			{"System.String x='a';", "Invalid type: System.String"},
			{"System.String x;", "Invalid type: System.String"},
			{"System.Integer x=1;", "Invalid type: System.Integer"},
			{"System.List<String> x=new List<String>();", "Invalid type: System.List"},
			{"System.assertEquals('1', System.String.valueOf(1));", ""},
		} {
			t.Run(api+"/"+tc.source, func(t *testing.T) {
				org := testOrg()
				handler := New(&org)
				for _, warm := range []bool{false, true} {
					body, err := json.Marshal(map[string]string{"anonymousBody": tc.source})
					if err != nil {
						t.Fatal(err)
					}
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/services/data/v%s/tooling/executeAnonymous", api), strings.NewReader(string(body))))
					var got struct {
						Compiled       bool   `json:"compiled"`
						Success        bool   `json:"success"`
						CompileProblem string `json:"compileProblem"`
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
						t.Fatal(err)
					}
					if rec.Code != http.StatusOK || got.Compiled != (tc.want == "") || (tc.want == "" && !got.Success) || got.CompileProblem != tc.want {
						t.Fatalf("warm=%v status=%d body=%s", warm, rec.Code, rec.Body.String())
					}
				}
			})
		}
	}
}
