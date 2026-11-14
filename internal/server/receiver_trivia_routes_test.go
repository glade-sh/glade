package server

import (
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
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

// F1 is a captured initializer diagnostic at line 2, column 8
// (receiver_split/org.tsv:144-145).
func TestReceiverTriviaAnonymousServerPosition(t *testing.T) {
	root := t.TempDir()
	support, err := os.ReadFile(filepath.Join("..", "sema", "testdata", "receiver_split", "CrProbe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("..", "sema", "testdata", "receiver_split", "grammar", "bound_field_read.apex"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "CrProbe.cls")
	writeServerTestFile(t, path, string(support))
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{path}}, schema.Schema{})
	org := testOrg()
	handler := New(&org)
	handler.SetProjectIndex(index)
	query := url.Values{"anonymousBody": []string{string(source)}}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v67.0/tooling/executeAnonymous?"+query.Encode(), nil))
	var got struct {
		Compiled       bool   `json:"compiled"`
		CompileProblem string `json:"compileProblem"`
		Line           int    `json:"line"`
		Column         int    `json:"column"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK || got.Compiled || got.CompileProblem != "Unexpected token 's'." || got.Line != 2 || got.Column != 8 {
		t.Errorf("tooling: status=%d body=%s err=%v", rec.Code, rec.Body.String(), err)
	}
	body := soapEnvelope(`<apex:executeAnonymous><apex:String>` + html.EscapeString(string(source)) + `</apex:String></apex:executeAnonymous>`)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, strings.Replace(soapApexTestPath, "65.0", "67.0", 1), strings.NewReader(body)))
	soap := decodeSOAPResult(t, rec.Body.String())
	if rec.Code != http.StatusOK || soap["compiled"] != "false" || soap["compileProblem"] != "Unexpected token 's'." || soap["line"] != "2" || soap["column"] != "8" {
		t.Errorf("SOAP: status=%d result=%v", rec.Code, soap)
	}
}
