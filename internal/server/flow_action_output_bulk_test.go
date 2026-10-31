package server

import (
	"encoding/json"
	"encoding/xml"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestFlowActionOutputBulkSOAPRoute(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/flow_action_output_bulk"
	flow, err := os.ReadFile(filepath.Join(fixture, "OwnedOutputProbe.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(fixture, "probe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "force-app/main/default/flows")
			if err := os.MkdirAll(path, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(path, "OwnedOutputProbe.flow-meta.xml"), []byte(strings.ReplaceAll(string(flow), "{{API}}", api)), 0644); err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			a, err := automation.LoadProject(p)
			if err != nil || len(a.Diagnostics) != 0 {
				t.Fatalf("flow load: %v %v", err, a.Diagnostics)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			automation.ApplyToOrg(&org, a)
			handler := New(&org)
			var escaped strings.Builder
			if err := xml.EscapeText(&escaped, body); err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/Soap/s/"+api+"/00DLOCAL000000001", strings.NewReader(soapEnvelope("<apex:executeAnonymous><apex:String>"+escaped.String()+"</apex:String></apex:executeAnonymous>"))))
			// SOAP uses the REST endpoint version inventory; 62.0 is not a
			// supported endpoint even though 62.0 Apex is supported elsewhere.
			if api == "62.0" {
				if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "sf:INVALID_VERSION") || !strings.Contains(rec.Body.String(), "unsupported API version 62.0") {
					t.Fatalf("API 62 SOAP boundary: %d %s", rec.Code, rec.Body.String())
				}
				return
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			result := decodeSOAPResult(t, rec.Body.String())
			if result["compiled"] != "true" {
				t.Fatalf("compile: %v", result)
			}
			_, payload, ok := strings.Cut(result["exceptionMessage"], "ROAD195_RESULT|")
			if !ok {
				t.Fatalf("missing observation: %v", result)
			}
			var got, want []any
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatal(err)
			}
			for _, item := range got {
				if m, ok := item.(map[string]any); ok {
					if outputs, ok := m["outputs"].(map[string]any); ok {
						delete(outputs, "Flow__InterviewGuid")
					}
				}
			}
			for _, line := range strings.Split(string(rows), "\n") {
				f := strings.SplitN(line, "\t", 3)
				if len(f) == 3 && f[0] == api && f[1] == "anonymous" {
					if err := json.Unmarshal([]byte(f[2]), &want); err != nil {
						t.Fatal(err)
					}
				}
			}
			if len(want) != 2 || !reflect.DeepEqual(got, want) {
				t.Fatalf("native %v; server %v", want, got)
			}
		})
	}
}

// Test the same captured operation through the Visualforce and Lightning Apex hosts.
func TestFlowActionOutputBulkHostRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/flow_action_output_bulk"
	flow, err := os.ReadFile(filepath.Join(fixture, "OwnedOutputProbe.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(fixture, "probe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	operation := strings.Replace(string(body), "System.assert(false,'ROAD195_RESULT|'+JSON.serialize(out));", "return JSON.serialize(out);", 1)
	if operation == string(body) {
		t.Fatal("capture marker missing")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/flows/OwnedOutputProbe.flow-meta.xml"), strings.ReplaceAll(string(flow), "{{API}}", api))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/OutputHost.cls"), "public class OutputHost { @AuraEnabled public static String observe(){"+operation+"} public String getOutput(){return observe();} }")
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/OutputHost.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/OutputHost.page"), `<apex:page controller="OutputHost"><apex:outputText value="{!output}"/></apex:page>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			flows, err := automation.LoadProject(p)
			if err != nil || len(flows.Diagnostics) != 0 {
				t.Fatalf("flows: %v %v", err, flows.Diagnostics)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			automation.ApplyToOrg(&org, flows)
			srv := newVisualforceHTMLTestServer(t, &org, source)
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			t.Run("vf", func(t *testing.T) {
				rec := httptest.NewRecorder()
				srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/OutputHost", nil))
				text := html.UnescapeString(rec.Body.String())
				if rec.Code != http.StatusOK || !strings.Contains(text, `"Echo":"alpha"`) || !strings.Contains(text, `"Echo":"beta"`) || !strings.Contains(text, `"Items":["first","second"]`) {
					t.Fatalf("VF %d: %s", rec.Code, text)
				}
			})
			t.Run("lwc", func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/apex/OutputHost/observe", strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				text := rec.Body.String()
				if rec.Code != http.StatusOK || !strings.Contains(text, "alpha") || !strings.Contains(text, "beta") || !strings.Contains(text, "first") || !strings.Contains(text, "second") {
					t.Fatalf("LWC %d: %s", rec.Code, text)
				}
			})
		})
	}
}
