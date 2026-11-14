package server

import (
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Native anonymous rows: receiver_split/org.tsv:70-71, 108-109.
func TestReceiverTriviaAnonymousServerRoutes(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"free", "String\n.valueOf('x');", "Variable does not exist: String"},
		{"bound", "System.\ndebug('x');", "Unexpected token 'debug'."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			handler := New(&org)
			query := url.Values{"anonymousBody": []string{tc.source}}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v67.0/tooling/executeAnonymous?"+query.Encode(), nil))
			var got struct {
				Compiled       bool   `json:"compiled"`
				CompileProblem string `json:"compileProblem"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK || got.Compiled || got.CompileProblem != tc.want {
				t.Errorf("tooling: status=%d body=%s err=%v", rec.Code, rec.Body.String(), err)
			}
			body := soapEnvelope(`<apex:executeAnonymous><apex:String>` + html.EscapeString(tc.source) + `</apex:String></apex:executeAnonymous>`)
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, strings.Replace(soapApexTestPath, "65.0", "67.0", 1), strings.NewReader(body)))
			soap := decodeSOAPResult(t, rec.Body.String())
			if rec.Code != http.StatusOK || soap["compiled"] != "false" || soap["compileProblem"] != tc.want {
				t.Errorf("SOAP: status=%d result=%v", rec.Code, soap)
			}
		})
	}
}
