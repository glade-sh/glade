package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native evidence is the named Apex test at API 62/67 in
// pr163_apex_basics/org.tsv:2-3. These are local host-route twins.
func TestHeaderIdLocalHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			dir := filepath.Join(root, "force-app/main/default/classes")
			body := `Id value = Id.valueOf('001000000000001AAA'); HttpRequest request = new HttpRequest(); request.setHeader('X-Probe', value); return request.getHeader('X-Probe');`
			for name, source := range map[string]string{
				"HeaderHost": `public class HeaderHost { @AuraEnabled @RemoteAction public static String observe() { ` + body + ` } public String getValue() { ` + body + ` } }`,
				"HeaderRest": `@RestResource(urlMapping='/header-probe/*') global class HeaderRest { @HttpGet global static String observe() { ` + body + ` } }`,
			} {
				path := filepath.Join(dir, name+".cls")
				writeServerTestFile(t, path, source)
				writeServerTestFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			page := filepath.Join(root, "force-app/main/default/pages/HeaderHost.page")
			writeServerTestFile(t, page, `<apex:page controller="HeaderHost"><apex:outputText value="{!value}"/></apex:page>`)
			writeServerTestFile(t, page+"-meta.xml", `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><label>Header</label></ApexPage>`)
			remotePage := filepath.Join(root, "force-app/main/default/pages/Remote.page")
			writeServerTestFile(t, remotePage, `<apex:page controller="HeaderHost"><apex:outputText value="{!value}"/></apex:page>`)
			writeServerTestFile(t, remotePage+"-meta.xml", `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><label>Remote</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			objects, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, objects)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if result := sema.Analyze(index); result.HasErrors() {
				t.Fatal(result.Diagnostics)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := testOrg()
			srv := newVisualforceHTMLTestServer(t, &org, source)
			srv.SetProjectIndex(index)
			const want = "001000000000001AAA"
			for pass := 0; pass < 2; pass++ {
				for _, route := range []struct{ method, path, body string }{
					{http.MethodGet, "/apex/HeaderHost", ""},
					{http.MethodPost, "/lightning/apex/HeaderHost/observe", `{}`},
					{http.MethodGet, "/services/apexrest/header-probe/1", ""},
				} {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
					req.Header.Set("Content-Type", "application/json")
					srv.ServeHTTP(rec, req)
					if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
						t.Fatalf("%s pass %d: %d %s", route.path, pass, rec.Code, rec.Body.String())
					}
				}
				responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"HeaderHost","method":"observe","data":[],"type":"rpc","tid":3}]`)
				if len(responses) != 1 || !responses[0].Status || responses[0].Result != want {
					t.Fatalf("remoting pass %d: %+v", pass, responses)
				}
			}
		})
	}
}

// The fixture pages and extensions were accepted for deployment at VF API 59
// and 67 (standard_controller_record_field/org.tsv:6-7). Page rendering here
// is a local route check, not a captured Salesforce page execution result.
func TestStandardControllerRecordIdLocalVisualforceHost(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/standard_controller_record_field/project"
	for _, version := range []string{"59", "67"} {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			name := "CaptureExtension" + version
			for _, item := range []struct{ dir, suffix string }{{"classes", ".cls"}, {"classes", ".cls-meta.xml"}, {"pages", ".page"}, {"pages", ".page-meta.xml"}} {
				data, err := os.ReadFile(filepath.Join(fixture, item.dir, name+item.suffix))
				if err != nil {
					t.Fatal(err)
				}
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default", item.dir, name+item.suffix), string(data))
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			objects, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, objects)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if result := sema.Analyze(index); result.HasErrors() {
				t.Fatal(result.Diagnostics)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := testOrg()
			const recordID storage.ID = "001000000000001"
			addAccountForTest(&org, recordID, "Host route")
			srv := newVisualforceHTMLTestServer(t, &org, source)
			grantVisualforceTestAccountNameRead(t, srv)
			srv.SetProjectIndex(index)
			for pass := 0; pass < 2; pass++ {
				rec := httptest.NewRecorder()
				srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/"+name+"?id="+string(recordID), nil))
				if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), string(recordID)) {
					t.Fatalf("pass %d: %d %s", pass, rec.Code, rec.Body.String())
				}
			}
		})
	}
}
