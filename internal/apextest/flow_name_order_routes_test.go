package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Salesforce: ../dml/testdata/flow_name_order/org.tsv. Both object types
// have a null derived Name in before-save criteria at APIs 62 and 67.
func TestFlowNameOrderNativeRoutes(t *testing.T) {
	fixture := "../dml/testdata/flow_name_order"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			for _, obj := range []string{"Contact", "User"} {
				for _, suffix := range []string{"Name_Order", "First_Rejection", "NullProbe", "NonNullProbe"} {
					name := obj + "_" + suffix + ".flow-meta.xml"
					b, err := os.ReadFile(filepath.Join(fixture, name))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "force-app/main/default/flows", name), strings.ReplaceAll(string(b), "62.0", api))
				}
			}
			contact := `Contact a = new Contact(FirstName='Ada', LastName='Probe'); Database.SaveResult r = Database.insert(a, false); System.assertEquals(true, r.isSuccess()); Contact n = new Contact(FirstName='NullProbe', LastName='Probe'); r = Database.insert(n, false); System.assertEquals(false, r.isSuccess()); System.assertEquals('name true null', r.getErrors()[0].getMessage());`
			user := `String suffix=String.valueOf(Datetime.now().getTime()); User a = new User(FirstName='Nia',LastName='Probe',Alias='probe',Email='probe@example.invalid',Username='nameone'+suffix+'@example.invalid',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',ProfileId=UserInfo.getProfileId()); Database.SaveResult r=Database.insert(a,false); System.assertEquals(true,r.isSuccess()); User n = new User(FirstName='NullProbe',LastName='Probe',Alias='probe',Email='probe@example.invalid',Username='nametwo'+suffix+'@example.invalid',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',ProfileId=UserInfo.getProfileId()); r=Database.insert(n,false); System.assertEquals(false,r.isSuccess()); System.assertEquals('name true null',r.getErrors()[0].getMessage());`
			cls := `@IsTest private class NameOrderRouteTest { @IsTest static void contactRoute(){` + contact + `} @IsTest static void userRoute(){` + user + `} }`
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/NameOrderRouteTest.cls"), cls)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/NameOrderRouteTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			index := loadTestIndex(t, root)
			org, err := orgFromIndexWithError(index)
			if err != nil {
				t.Fatal(err)
			}
			org.APIVersion = api
			for _, tc := range []struct{ name, source, method string }{{"contact", contact, "contactRoute"}, {"user", user, "userRoute"}} {
				t.Run(tc.name, func(t *testing.T) {
					t.Run("anonymous", func(t *testing.T) {
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						program, err := vm.CompileAnonymousWithOptions(tc.source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err = runner.execute(program); err != nil {
							t.Fatalf("anonymous: %v", err)
						}
					})
					t.Run("istest", func(t *testing.T) {
						result := Run(index, Options{SelectedClasses: []string{"NameOrderRouteTest"}, SelectedMethod: tc.method, RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
						if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem != nil {
							t.Fatalf("named: %+v", result.Summary())
						}
					})
				})
			}
		})
	}
}
