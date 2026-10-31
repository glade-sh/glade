package apextest_test

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Replay the API 62/67 native Apex observations through local host adapters;
// this does not claim native Visualforce or LWC capture.
func TestMetadataNamespaceRelationshipHostRoutes(t *testing.T) {
	write := func(root, name, body string) {
		t.Helper()
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("testdata/conformance/metadata_field_names/fixture")); err != nil {
				t.Fatal(err)
			}
			write(root, "sfdx-project.json", fmt.Sprintf(`{"namespace":"gladens004","sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			probe, err := os.ReadFile("testdata/conformance/metadata_namespace_relationship/probe.cls")
			if err != nil {
				t.Fatal(err)
			}
			write(root, "force-app/main/default/classes/Road215NamespaceProbe.cls", string(probe))
			for _, name := range []string{"Road215NamespaceProbe", "Road215Host"} {
				write(root, "force-app/main/default/classes/"+name+".cls-meta.xml", `<ApexClass><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			write(root, "force-app/main/default/classes/Road215Host.cls", `global class Road215Host { @AuraEnabled(cacheable=true) global static String observe() { return JSON.serialize(Road215NamespaceProbe.observe()); } global String getObservation() { return observe(); } }`)
			write(root, "force-app/main/default/pages/Road215Host.page", `<apex:page controller="Road215Host" showHeader="false" sidebar="false"><pre id="observation"><apex:outputText value="{!observation}"/></pre></apex:page>`)
			write(root, "force-app/main/default/pages/Road215Host.page-meta.xml", `<ApexPage><apiVersion>`+api+`</apiVersion><label>Host</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			shape, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, shape)
			source, err := server.NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := apextest.OrgFromIndex(index)
			org.APIVersion = api
			const userID storage.ID = "005000000000091"
			storage.EnsureStandardObject(&org, "User")
			users := org.Objects["User"]
			users.Records[userID] = storage.Record{ID: userID, Object: "User", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true)}}
			org.Objects["User"] = users
			handler := server.NewWithSource(&org, source)
			handler.VisualforceHTMLUserID = userID
			handler.SetProjectIndex(index)
			for _, route := range []string{"vf", "lwc", "wire"} {
				t.Run(route, func(t *testing.T) {
					method, path, body := http.MethodGet, "/apex/Road215Host", ""
					if route == "lwc" {
						method, path, body = http.MethodPost, "/lightning/apex/Road215Host/observe", `{}`
					}
					if route == "wire" {
						method, path, body = http.MethodPost, "/lightning/wire/apex", `{"className":"Road215Host","method":"observe","params":{}}`
					}
					for attempt := 0; attempt < 2; attempt++ {
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(method, path, strings.NewReader(body))
						req.Header.Set("Content-Type", "application/json")
						handler.ServeHTTP(rec, req)
						if rec.Code != http.StatusOK {
							t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
						}
						var observed string
						if route == "vf" {
							_, after, ok := strings.Cut(rec.Body.String(), `<pre id="observation">`)
							value, _, closed := strings.Cut(after, "</pre>")
							if !ok || !closed {
								t.Fatalf("missing observation: %s", rec.Body.String())
							}
							observed = html.UnescapeString(value)
						} else {
							var result lwcbrowser.WireResponse
							if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
								t.Fatal(err)
							}
							if result.Error != nil {
								t.Fatalf("bridge error: %+v", result.Error)
							}
							var ok bool
							observed, ok = result.Data.(string)
							if !ok {
								t.Fatalf("observation=%#v", result.Data)
							}
						}
						var got map[string]any
						if err := json.Unmarshal([]byte(observed), &got); err != nil {
							t.Fatal(err)
						}
						for k, want := range map[string]any{"objectRelationship": "gladens004__MeadowEntry__c", "fieldRelationship": "gladens004__State__c", "standardFieldRelationship": "Rating", "matchingGroups": float64(1), "missing": float64(0)} {
							if got[k] != want {
								t.Fatalf("%s=%v want %v", k, got[k], want)
							}
						}
					}
				})
			}
		})
	}
}

// Foreign-reference host tests replay the preserved local fallback, not a native row.
func TestMetadataForeignRelationshipHostRoutes(t *testing.T) {
	write := func(root, name, body string) {
		t.Helper()
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("testdata/conformance/metadata_field_names/fixture")); err != nil {
				t.Fatal(err)
			}
			recordPath := "force-app/main/default/customMetadata/MeadowField.CustomState.md-meta.xml"
			record, err := os.ReadFile(filepath.Join(root, recordPath))
			if err != nil {
				t.Fatal(err)
			}
			foreign := strings.ReplaceAll(string(record), "MeadowEntry__c.State__c", "other__MeadowEntry__c.other__State__c")
			foreign = strings.ReplaceAll(foreign, ">MeadowEntry__c<", ">other__MeadowEntry__c<")
			write(root, recordPath, foreign)
			write(root, "sfdx-project.json", fmt.Sprintf(`{"namespace":"gladens004","sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			probe, err := os.ReadFile("testdata/conformance/metadata_namespace_relationship/probe.cls")
			if err != nil {
				t.Fatal(err)
			}
			write(root, "force-app/main/default/classes/Road215NamespaceProbe.cls", string(probe))
			for _, name := range []string{"Road215NamespaceProbe", "Road215Host"} {
				write(root, "force-app/main/default/classes/"+name+".cls-meta.xml", `<ApexClass><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			write(root, "force-app/main/default/classes/Road215Host.cls", `global class Road215Host { @AuraEnabled(cacheable=true) global static String observe() { return JSON.serialize(Road215NamespaceProbe.observe()); } global String getObservation() { return observe(); } }`)
			write(root, "force-app/main/default/pages/Road215Host.page", `<apex:page controller="Road215Host" showHeader="false" sidebar="false"><pre id="observation"><apex:outputText value="{!observation}"/></pre></apex:page>`)
			write(root, "force-app/main/default/pages/Road215Host.page-meta.xml", `<ApexPage><apiVersion>`+api+`</apiVersion><label>Host</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			shape, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, shape)
			source, err := server.NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := apextest.OrgFromIndex(index)
			org.APIVersion = api
			const userID storage.ID = "005000000000091"
			storage.EnsureStandardObject(&org, "User")
			users := org.Objects["User"]
			users.Records[userID] = storage.Record{ID: userID, Object: "User", Fields: map[string]storage.Value{"IsActive": storage.BooleanValue(true)}}
			org.Objects["User"] = users
			handler := server.NewWithSource(&org, source)
			handler.VisualforceHTMLUserID = userID
			handler.SetProjectIndex(index)
			for _, route := range []string{"vf", "lwc", "wire"} {
				t.Run(route, func(t *testing.T) {
					method, path, body := http.MethodGet, "/apex/Road215Host", ""
					if route == "lwc" {
						method, path, body = http.MethodPost, "/lightning/apex/Road215Host/observe", `{}`
					}
					if route == "wire" {
						method, path, body = http.MethodPost, "/lightning/wire/apex", `{"className":"Road215Host","method":"observe","params":{}}`
					}
					for attempt := 0; attempt < 2; attempt++ {
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(method, path, strings.NewReader(body))
						req.Header.Set("Content-Type", "application/json")
						handler.ServeHTTP(rec, req)
						if rec.Code != http.StatusOK {
							t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
						}
						var observed string
						if route == "vf" {
							_, after, ok := strings.Cut(rec.Body.String(), `<pre id="observation">`)
							value, _, closed := strings.Cut(after, "</pre>")
							if !ok || !closed {
								t.Fatalf("missing observation: %s", rec.Body.String())
							}
							observed = html.UnescapeString(value)
						} else {
							var result lwcbrowser.WireResponse
							if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
								t.Fatal(err)
							}
							if result.Error != nil {
								t.Fatalf("bridge error: %+v", result.Error)
							}
							var ok bool
							observed, ok = result.Data.(string)
							if !ok {
								t.Fatalf("observation=%#v", result.Data)
							}
						}
						var got map[string]any
						if err := json.Unmarshal([]byte(observed), &got); err != nil {
							t.Fatal(err)
						}
						for k, want := range map[string]any{"objectRelationship": "other__MeadowEntry__c", "fieldRelationship": "other__State__c", "standardFieldRelationship": "Rating", "matchingGroups": float64(0), "missing": float64(0)} {
							if got[k] != want {
								t.Fatalf("%s=%v want %v", k, got[k], want)
							}
						}
					}
				})
			}
		})
	}
}
