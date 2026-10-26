package sema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPersonAccountCustomFieldsNativeSemanticRoutes(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/person_account_custom_fields/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Files           map[string]string
		StaticRejection string
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, accountSource := range []bool{true, false} {
			for _, route := range []string{"anonymous", "named", "istest"} {
				t.Run(fmt.Sprintf("%s/%s/accountSource=%t", api, route, accountSource), func(t *testing.T) {
					root := t.TempDir()
					write := func(name, body string) {
						t.Helper()
						path := filepath.Join(root, name)
						if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
							t.Fatal(err)
						}
						writeSemaFile(t, path, body)
					}
					write("sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
					write("glade.yml", "org:\n  features: [PersonAccounts]\n")
					for name, body := range cases.Files {
						if strings.Contains(name, "/classes/") || (!accountSource && strings.Contains(name, "/objects/Account/")) {
							continue
						}
						write(name, strings.ReplaceAll(body, "62.0", api))
					}
					body := "Account a = new Account(LastName='Garden',Note__pc='x'); a.Note__pc='y'; List<Account> rows=[SELECT Note__pc,Sponsor__pr.Name FROM Account WHERE Note__pc='y']; Schema.DescribeFieldResult describe=Schema.SObjectType.Account.fields.getMap().get('Note__pc').getDescribe();"
					for _, reject := range []bool{false, true} {
						source := body
						if reject {
							source = cases.StaticRejection
						}
						if route != "anonymous" {
							prefix := "public class PersonFieldSemanticProbe { public static void probe() {"
							if route == "istest" {
								prefix = "@IsTest private class PersonFieldSemanticProbe { @IsTest static void probe() {"
							}
							path := filepath.Join(root, "force-app/main/default/classes/PersonFieldSemanticProbe.cls")
							if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
								t.Fatal(err)
							}
							writeSemaFile(t, path, prefix+source+"} }")
							writeSemaFile(t, path+"-meta.xml", "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
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
						if !reject {
							found := false
							for _, object := range prepareAnalysisIndex(index).Objects {
								if !strings.EqualFold(object.Name, "Account") {
									continue
								}
								for _, field := range object.Fields {
									if !strings.EqualFold(field.Name, "Note__pc") {
										continue
									}
									found = true
									if got := semaApexTypeForSchemaField(field); got != "String" {
										t.Errorf("native text mirror must bind as String, got %q from %q", got, field.Type)
									}
								}
							}
							if !found {
								t.Error("native text mirror is absent from prepared schema")
							}
						}
						result := Analyze(index)
						if route == "anonymous" {
							result = AnalyzeAnonymous(index, source, api)
						}
						if result.HasErrors() != reject {
							t.Errorf("native rejection=%t; diagnostics=%v", reject, result.Diagnostics)
						}
					}
				})
			}
		}
	}
}
