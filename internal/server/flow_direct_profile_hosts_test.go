package server

import (
	"encoding/csv"
	"encoding/xml"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestFlowDirectProfileCapturedHostRoutes(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "flow_direct_profile_decision")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			f, err := os.Open(filepath.Join(fixture, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			reader := csv.NewReader(f)
			reader.Comma = '\t'
			rows, err := reader.ReadAll()
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			for _, row := range rows[1:] {
				if row[1] == api && row[2] == "anonymous" {
					_, want, _ = strings.Cut(row[5], "OBS=")
				}
			}
			if want == "" {
				t.Fatal("missing native observation")
			}
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			sourceRoot := filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), "force-app", "main", "default")
			for _, rel := range []string{"flows/ProfileProbe.flow-meta.xml", "classes/ProfileProbeCase.cls", "classes/ProfileProbeCase.cls-meta.xml"} {
				data, err := os.ReadFile(filepath.Join(sourceRoot, rel))
				if err != nil {
					t.Fatal(err)
				}
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default", rel), string(data))
			}
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/ProfileHost.cls"), `public class ProfileHost { @AuraEnabled public static String observe() { return ProfileProbeCase.run(); } public String getOutput() { return observe(); } }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/ProfileHost.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/ProfileHost.page"), `<apex:page controller="ProfileHost"><apex:outputText value="{!output}"/></apex:page>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sourceMetadata, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			flows, err := automation.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			for _, name := range []string{"Contact", "User", "Profile"} {
				storage.EnsureStandardObject(&org, name)
			}
			storage.EnsureDeterministicPlatformData(&org)
			automation.ApplyToOrg(&org, flows)
			srv := newVisualforceHTMLTestServer(t, &org, sourceMetadata)
			srv.VisualforceHTMLUserID = storage.DefaultUserID
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			t.Run("vf", func(t *testing.T) {
				rec := httptest.NewRecorder()
				srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/ProfileHost", nil))
				if rec.Code != http.StatusOK || !strings.Contains(html.UnescapeString(rec.Body.String()), want) {
					t.Fatalf("VF status=%d body=%s", rec.Code, rec.Body.String())
				}
			})
			t.Run("lwc", func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/apex/ProfileHost/observe", strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
					t.Fatalf("LWC status=%d body=%s", rec.Code, rec.Body.String())
				}
			})
			t.Run("soap", func(t *testing.T) {
				body := `System.assertEquals('` + want + `',ProfileProbeCase.run());`
				var escaped strings.Builder
				if err := xml.EscapeText(&escaped, []byte(body)); err != nil {
					t.Fatal(err)
				}
				rec := httptest.NewRecorder()
				srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, strings.Replace(soapApexTestPath, "65.0", api, 1), strings.NewReader(soapEnvelope("<apex:executeAnonymous><apex:String>"+escaped.String()+"</apex:String></apex:executeAnonymous>"))))
				if api == "62.0" {
					if !strings.Contains(rec.Body.String(), "INVALID_VERSION") {
						t.Fatalf("SOAP floor: %d %s", rec.Code, rec.Body.String())
					}
					return
				}
				if rec.Code != http.StatusOK {
					t.Fatalf("SOAP status=%d", rec.Code)
				}
				result := decodeSOAPResult(t, rec.Body.String())
				if result["compiled"] != "true" || result["success"] != "true" {
					t.Fatalf("SOAP result: %v", result)
				}
			})
		})
	}
}
