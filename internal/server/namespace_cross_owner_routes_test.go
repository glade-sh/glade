package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestCrossNamespaceOwnerServerRoutes(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(root, "LocalShape.cls")
	foreign := filepath.Join(child, "ForeignShape.cls")
	for path, source := range map[string]string{local: "public class LocalShape {}", foreign: "public class ForeignShape {}"} {
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: "67.0", SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{local, foreign}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatal(idx.Diagnostics)
	}
	org := testOrg()
	handler := New(&org)
	handler.Index = &idx
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, wrong := range []string{"host.ForeignShape", "other.LocalShape"} {
			t.Run(api+"/"+wrong, func(t *testing.T) {
				source := wrong + " value;"
				payload, _ := json.Marshal(map[string]string{"anonymousBody": source})
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(payload))))
				if api == "62.0" {
					if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "unsupported API version") {
						t.Fatalf("REST boundary status=%d body=%s", rec.Code, rec.Body.String())
					}
				} else if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"compiled":false`) || !strings.Contains(rec.Body.String(), "Invalid type: "+wrong) {
					t.Fatalf("REST status=%d body=%s", rec.Code, rec.Body.String())
				}
				rec = httptest.NewRecorder()
				body := soapEnvelope("<apex:executeAnonymous><apex:String>" + source + "</apex:String></apex:executeAnonymous>")
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/Soap/s/"+api+"/00DLOCAL000000001", strings.NewReader(body)))
				if api == "62.0" {
					if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "INVALID_VERSION") {
						t.Fatalf("SOAP boundary status=%d body=%s", rec.Code, rec.Body.String())
					}
					return
				}
				result := decodeSOAPResult(t, rec.Body.String())
				if rec.Code != http.StatusOK || result["compiled"] != "false" || !strings.Contains(result["compileProblem"], "Invalid type: "+wrong) {
					t.Fatalf("SOAP status=%d body=%s", rec.Code, rec.Body.String())
				}
			})
		}
	}
}

func TestCrossNamespaceOwnerServerCast(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(root, "LocalShape.cls")
	foreign := filepath.Join(child, "ForeignShape.cls")
	for path, source := range map[string]string{local: "public class LocalShape {}", foreign: "public class ForeignShape {}"} {
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: "67.0", SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{local, foreign}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatal(idx.Diagnostics)
	}
	org := testOrg()
	handler := New(&org)
	handler.Index = &idx
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, wrong := range []string{"host.ForeignShape", "other.LocalShape"} {
			t.Run(api+"/"+wrong, func(t *testing.T) {
				source := "Object source = null; Object value = (" + wrong + ")source;"
				payload, _ := json.Marshal(map[string]string{"anonymousBody": source})
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(payload))))
				if api == "62.0" {
					if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "unsupported API version") {
						t.Fatalf("REST boundary status=%d body=%s", rec.Code, rec.Body.String())
					}
				} else if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"compiled":false`) || !strings.Contains(rec.Body.String(), "Invalid type: "+wrong) {
					t.Fatalf("REST status=%d body=%s", rec.Code, rec.Body.String())
				}
				rec = httptest.NewRecorder()
				body := soapEnvelope("<apex:executeAnonymous><apex:String>" + source + "</apex:String></apex:executeAnonymous>")
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/Soap/s/"+api+"/00DLOCAL000000001", strings.NewReader(body)))
				if api == "62.0" {
					if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "INVALID_VERSION") {
						t.Fatalf("SOAP boundary status=%d body=%s", rec.Code, rec.Body.String())
					}
					return
				}
				result := decodeSOAPResult(t, rec.Body.String())
				if rec.Code != http.StatusOK || result["compiled"] != "false" || !strings.Contains(result["compileProblem"], "Invalid type: "+wrong) {
					t.Fatalf("SOAP status=%d body=%s", rec.Code, rec.Body.String())
				}
			})
		}
	}
}

func TestCrossNamespaceSameNameServerRoutes(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(root, "SharedShape.cls")
	foreign := filepath.Join(child, "SharedShape.cls")
	for _, path := range []string{local, foreign} {
		if err := os.WriteFile(path, []byte("public class SharedShape {}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: "67.0", SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{local, foreign}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatal(idx.Diagnostics)
	}
	org := testOrg()
	handler := New(&org)
	handler.Index = &idx
	for _, api := range []string{"67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			source := "host.SharedShape first; other.SharedShape second;"
			payload, _ := json.Marshal(map[string]string{"anonymousBody": source})
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(payload))))
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"compiled":true`) {
				t.Fatalf("REST status=%d body=%s", rec.Code, rec.Body.String())
			}
			rec = httptest.NewRecorder()
			body := soapEnvelope("<apex:executeAnonymous><apex:String>" + source + "</apex:String></apex:executeAnonymous>")
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/Soap/s/"+api+"/00DLOCAL000000001", strings.NewReader(body)))
			result := decodeSOAPResult(t, rec.Body.String())
			if rec.Code != http.StatusOK || result["compiled"] != "true" {
				t.Fatalf("SOAP status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCrossNamespaceOwnerServerCallerPositions(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(child, "ForeignShape.cls")
	if err := os.WriteFile(foreign, []byte("public class ForeignShape {}"), 0600); err != nil {
		t.Fatal(err)
	}
	idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: "67.0", SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{foreign}}, schema.Schema{})
	if idx.HasErrors() {
		t.Fatal(idx.Diagnostics)
	}
	org := testOrg()
	handler := New(&org)
	handler.Index = &idx
	for _, api := range []string{"67.0", "68.0"} {
		for name, source := range map[string]string{"class-literal": "Type value = host.ForeignShape.class;", "catch": "try { Integer value = 1; } catch (host.ForeignShape e) { }", "foreach": "for (host.ForeignShape item : new List<host.ForeignShape>()) { }"} {
			t.Run(api+"/"+name, func(t *testing.T) {
				payload, _ := json.Marshal(map[string]string{"anonymousBody": source})
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(payload))))
				if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"compiled":false`) || !strings.Contains(rec.Body.String(), "Invalid type: host.ForeignShape") {
					t.Fatalf("REST status=%d body=%s", rec.Code, rec.Body.String())
				}
				rec = httptest.NewRecorder()
				body := soapEnvelope("<apex:executeAnonymous><apex:String>" + strings.ReplaceAll(strings.ReplaceAll(source, "<", "&lt;"), ">", "&gt;") + "</apex:String></apex:executeAnonymous>")
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/Soap/s/"+api+"/00DLOCAL000000001", strings.NewReader(body)))
				result := decodeSOAPResult(t, rec.Body.String())
				if rec.Code != http.StatusOK || result["compiled"] != "false" || !strings.Contains(result["compileProblem"], "Invalid type: host.ForeignShape") {
					t.Fatalf("SOAP status=%d body=%s", rec.Code, rec.Body.String())
				}
			})
		}
	}
}
