package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Native rows: testdata/conformance/flow_partial_rows/org.tsv.
func TestFlowPartialValidationNativeRoutes(t *testing.T) {
	partial := `Account bad=new Account(Id='001000000000001AAA',Name='BAD_ID'); Account flow=new Account(Name='FLOW_REJECT'); List<Database.SaveResult> a=Database.insert(new List<Account>{bad,flow},false); System.assertEquals(2,a.size()); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(a[0].getErrors()[0].getStatusCode())); System.assertEquals('FIELD_CUSTOM_VALIDATION_EXCEPTION',String.valueOf(a[1].getErrors()[0].getStatusCode())); List<Database.SaveResult> b=Database.insert(new List<Account>{flow,bad},false); System.assertEquals(2,b.size()); System.assertEquals('FIELD_CUSTOM_VALIDATION_EXCEPTION',String.valueOf(b[0].getErrors()[0].getStatusCode())); System.assertEquals('INVALID_FIELD_FOR_INSERT_UPDATE',String.valueOf(b[1].getErrors()[0].getStatusCode())); List<Database.SaveResult> c=Database.insert(new List<Account>{new Account(Name='NORMAL'),new Account(Name='FLOW_REJECT')},false); System.assertEquals(2,c.size()); System.assertEquals(true,c[0].isSuccess()); System.assertEquals('FIELD_CUSTOM_VALIDATION_EXCEPTION',String.valueOf(c[1].getErrors()[0].getStatusCode()));`
	checkbox := `Account a=new Account(Name='DEFAULT_PROBE_NULL',ProbeFlag__c=null); Database.SaveResult r=Database.insert(a,false); System.assertEquals(false,r.isSuccess()); System.assertEquals('INVALID_TYPE_ON_FIELD_IN_RECORD',String.valueOf(r.getErrors()[0].getStatusCode()));`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			for _, file := range []struct{ path, dest string }{
				{"testdata/conformance/flow_partial_rows/Partial_Reject.flow-meta.xml", "flows/Partial_Reject.flow-meta.xml"},
				{"../dml/testdata/flow_default_order/Default_Order_Probe.flow-meta.xml", "flows/Default_Order_Probe.flow-meta.xml"},
				{"../dml/testdata/flow_default_order/ProbeFlag__c.field-meta.xml", "objects/Account/fields/ProbeFlag__c.field-meta.xml"},
			} {
				b, err := os.ReadFile(file.path)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default", file.dest), strings.ReplaceAll(strings.ReplaceAll(string(b), "62.0", api), "67.0", api))
			}
			cls := `@IsTest private class PartialRouteTest { @IsTest static void partialRoute(){` + partial + `} @IsTest static void checkboxRoute(){` + checkbox + `} }`
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/PartialRouteTest.cls"), cls)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/PartialRouteTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			index := loadTestIndex(t, root)
			org, err := orgFromIndexWithError(index)
			if err != nil {
				t.Fatal(err)
			}
			org.APIVersion = api
			for _, tc := range []struct{ name, source, method string }{{"partial", partial, "partialRoute"}, {"checkbox", checkbox, "checkboxRoute"}} {
				t.Run(tc.name, func(t *testing.T) {
					t.Run("anonymous", func(t *testing.T) {
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						p, err := vm.CompileAnonymousWithOptions(tc.source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err = runner.execute(p); err != nil {
							t.Fatalf("anonymous: %v", err)
						}
					})
					t.Run("istest", func(t *testing.T) {
						r := Run(index, Options{SelectedClasses: []string{"PartialRouteTest"}, SelectedMethod: tc.method, RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
						if len(r.Suites) != 1 || len(r.Suites[0].Cases) != 1 || r.Suites[0].Cases[0].Problem != nil {
							t.Fatalf("named: %+v", r.Summary())
						}
					})
				})
			}
		})
	}
}
