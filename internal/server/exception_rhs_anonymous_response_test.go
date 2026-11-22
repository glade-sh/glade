package server

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// NATIVE ../apextest/testdata/exception_rhs_call_frames/org.tsv:2-3,6-7,10-11,14-15,18,20,22,24.
func TestExceptionRHSAnonymousResponse(t *testing.T) {
	dir := filepath.Join("..", "apextest", "testdata", "exception_rhs_call_frames")
	rows, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			suffix := strings.TrimSuffix(api, ".0")
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			name := "FrameProbe_" + suffix
			source, err := os.ReadFile(filepath.Join(dir, "classes", name+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			classDir := filepath.Join(root, "force-app/main/default/classes")
			writeServerTestFile(t, filepath.Join(classDir, name+".cls"), string(source))
			writeServerTestFile(t, filepath.Join(classDir, name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			srv := newLightningTestServer(t, &org, metadata)
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
				row := strings.Split(line, "\t")
				if len(row) != 4 {
					t.Fatal("invalid native row")
				}
				if row[1] != api || row[2] != "anonymous" {
					continue
				}
				t.Run(row[0], func(t *testing.T) {
					file := strings.TrimSuffix(row[0], "_anonymous") + ".apex"
					code, err := os.ReadFile(filepath.Join(dir, "anonymous", file))
					if err != nil {
						t.Fatal(err)
					}
					body, err := json.Marshal(map[string]string{"anonymousBody": strings.ReplaceAll(string(code), "FrameProbe", name)})
					if err != nil {
						t.Fatal(err)
					}
					rec := httptest.NewRecorder()
					srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(body))))
					var got struct {
						Compiled            bool    `json:"compiled"`
						Success             bool    `json:"success"`
						CompileProblem      *string `json:"compileProblem"`
						ExceptionMessage    *string `json:"exceptionMessage"`
						ExceptionStackTrace *string `json:"exceptionStackTrace"`
						Line                int     `json:"line"`
						Column              int     `json:"column"`
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
						t.Fatalf("body %q: %v", rec.Body.String(), err)
					}
					want := strings.ReplaceAll(row[3], `\n`, "\n")
					wantMessage := fmt.Sprintf("%s.FrameException: %s.FrameException:[__lineNumber=2, __stackTrace=Class.%s.fail: line 2, column 1, __thrown=true]", name, name, name)
					if rec.Code != http.StatusOK || !got.Compiled || got.Success || got.CompileProblem != nil || got.ExceptionMessage == nil || *got.ExceptionMessage != wantMessage || got.ExceptionStackTrace == nil || *got.ExceptionStackTrace != want || got.Line != 1 || got.Column != 1 {
						t.Errorf("response: %s, want stack %q", rec.Body.String(), want)
					}
					var escaped bytes.Buffer
					if err := xml.EscapeText(&escaped, []byte(strings.ReplaceAll(string(code), "FrameProbe", name))); err != nil {
						t.Fatal(err)
					}
					soapBody := soapEnvelope("<apex:executeAnonymous><apex:String>" + escaped.String() + "</apex:String></apex:executeAnonymous>")
					soap := httptest.NewRecorder()
					srv.ServeHTTP(soap, httptest.NewRequest(http.MethodPost, "/services/Soap/s/"+api+"/example", strings.NewReader(soapBody)))
					soapResult := decodeSOAPResult(t, soap.Body.String())
					if soap.Code != http.StatusOK || soapResult["compiled"] != "true" || soapResult["success"] != "false" || soapResult["compileProblem"] != "" || soapResult["exceptionMessage"] != wantMessage || soapResult["exceptionStackTrace"] != want || soapResult["line"] != "1" || soapResult["column"] != "1" {
						t.Errorf("SOAP response: %s, want stack %q", soap.Body.String(), want)
					}
				})
			}
		})
	}
}
