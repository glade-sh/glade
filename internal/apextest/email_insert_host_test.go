package apextest_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestEmailInsertHostRoutes(t *testing.T) {
	body := `Database.SaveResult r=Database.insert(new Contact(LastName='Probe',Email='a@example.com.'),false); if(r.isSuccess()) return 'accepted'; return String.valueOf(r.getErrors()[0].getStatusCode())+'|'+r.getErrors()[0].getMessage()+'|'+String.valueOf(r.getErrors()[0].getFields());`
	want := "INVALID_EMAIL_ADDRESS|Email: invalid email address: a@example.com.|(Email)"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"sfdx-project.json":                                     fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api),
				"force-app/main/default/classes/EmailHost.cls":          "global class EmailHost { @AuraEnabled global static String lwc() { " + body + " } global String vf() { " + body + " } }",
				"force-app/main/default/classes/EmailHost.cls-meta.xml": fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api),
			}
			for name, content := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			for _, route := range []string{"lwc", "vf"} {
				t.Run(route, func(t *testing.T) {
					org, err := apextest.OrgFromIndexWithConfiguredPermissions(index)
					if err != nil {
						t.Fatal(err)
					}
					org.APIVersion = api
					machine := vm.New(nil)
					machine.SetOrg(&org)
					if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
						t.Fatal(err)
					}
					var result vm.UIInvocationResult
					if route == "lwc" {
						result, err = machine.InvokeLWCMethod("EmailHost", "lwc", nil)
					} else {
						result, err = machine.InvokeVisualforceAction("EmailHost", "vf", "/apex/EmailHost", nil)
					}
					if err != nil || !result.Success || result.ReturnValue != want {
						t.Fatalf("host result=%#v error=%v", result, err)
					}
				})
			}
			if api == "67.0" {
				t.Run("server", func(t *testing.T) {
					org := storage.NewOrgState()
					org.APIVersion = api
					storage.EnsureStandardObject(&org, "Contact")
					h := server.New(&org)
					source := `Database.SaveResult r=Database.insert(new Contact(LastName='Probe',Email='a@example.com.'),false); System.assertEquals(false,r.isSuccess()); System.assertEquals('INVALID_EMAIL_ADDRESS',String.valueOf(r.getErrors()[0].getStatusCode())); System.assertEquals('Email: invalid email address: a@example.com.',r.getErrors()[0].getMessage()); System.assertEquals('Email',r.getErrors()[0].getFields()[0]);`
					q := url.Values{"anonymousBody": []string{source}}
					rec := httptest.NewRecorder()
					h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services/data/v"+api+"/tooling/executeAnonymous?"+q.Encode(), nil))
					var result struct {
						Compiled         bool   `json:"compiled"`
						Success          bool   `json:"success"`
						ExceptionMessage string `json:"exceptionMessage"`
						CompileProblem   string `json:"compileProblem"`
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					if rec.Code != 200 || !result.Compiled || !result.Success {
						t.Fatalf("status=%d result=%+v", rec.Code, result)
					}
				})
			}
		})
	}
}
