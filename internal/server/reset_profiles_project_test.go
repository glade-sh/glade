package server

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The retained native project observes profile identities through singleton
// queries at both supported API boundaries. Reset must keep those queries valid.
func TestGLADEResetProjectProfiles(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/standard_profiles"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			file, err := os.Open(filepath.Join(fixture, "native/developer/org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			reader := csv.NewReader(file)
			reader.Comma, reader.LazyQuotes = '\t', true
			rows, err := reader.ReadAll()
			file.Close()
			if err != nil {
				t.Fatal(err)
			}
			var identities map[string]string
			for _, row := range rows[1:] {
				if row[1] == api && row[2] == "anonymous" && row[3] == "profiles" {
					if identities != nil {
						t.Fatal("duplicate native profile observation")
					}
					if err := json.Unmarshal([]byte(row[4]), &identities); err != nil {
						t.Fatal(err)
					}
				}
			}
			if len(identities) != 6 {
				t.Fatal("incomplete native profile observation")
			}
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}],"sourceApiVersion":"`+api+`"}`)
			source, err := os.ReadFile(filepath.Join(fixture, "project/force-app/main/default/classes/StandardProfileProbe.cls"))
			if err != nil {
				t.Fatal(err)
			}
			class := filepath.Join(root, "force-app/main/default/classes/StandardProfileProbe.cls")
			writeServerTestFile(t, class, string(source))
			resource := filepath.Join(root, "force-app/main/default/classes/ProfileResource.cls")
			writeServerTestFile(t, resource, `@RestResource(urlMapping='/profile-probe/*') global class ProfileResource {
    @HttpGet global static Map<String,String> profiles() { return StandardProfileProbe.profiles(); }
}`)
			writeServerTestFile(t, resource+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, class+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, schema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			for _, scope := range []string{"data", "users", "platform", "all", "limits,async"} {
				t.Run(scope, func(t *testing.T) {
					org := resetScopeTestOrg()
					handler := New(&org)
					handler.SetProjectIndex(index)
					if handler.runtimeErr != nil {
						t.Fatal(handler.runtimeErr)
					}
					base := serverTestDataPath
					reset := httptest.NewRecorder()
					handler.ServeHTTP(reset, httptest.NewRequest(http.MethodPost, base+"/glade/reset?scope="+scope, nil))
					if reset.Code != http.StatusOK {
						t.Fatalf("reset status=%d: %s", reset.Code, reset.Body.String())
					}
					exec := httptest.NewRecorder()
					handler.ServeHTTP(exec, httptest.NewRequest(http.MethodGet, "/services/apexrest/profile-probe/identities", nil))
					var result map[string]string
					if err := json.Unmarshal(exec.Body.Bytes(), &result); err != nil || exec.Code != http.StatusOK || !reflect.DeepEqual(result, identities) {
						t.Fatalf("native project after reset: status=%d body=%s error=%v", exec.Code, exec.Body.String(), err)
					}
				})
			}
		})
	}
}
