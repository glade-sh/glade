package visualforce_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPageContentNativeProjectContextRoutes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "apextest", "testdata", "conformance", "blob_pdf_rendering", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			write := func(name, body string) {
				t.Helper()
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			base := "force-app/main/default/"
			write(base+"classes/ContentDateController.cls", `public class ContentDateController { public Datetime getStamp() { return Datetime.newInstanceGmt(2024,2,9,17,6,3); } }`)
			write(base+"classes/ContentDateController.cls-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			write(base+"pages/ContentDate.page", `<apex:page controller="ContentDateController" showHeader="false" sidebar="false"><apex:outputText value="{0,date,yyyy-MM-dd}"><apex:param value="{!stamp}"/></apex:outputText></apex:page>`)
			write(base+"pages/ContentDate.page-meta.xml", `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><label>Content date</label></ApexPage>`)
			var methods strings.Builder
			methods.WriteString("@IsTest private class ContentDateTest {\n")
			anonymous := map[string]string{}
			count := 0
			for _, line := range lines[1:] {
				row := strings.SplitN(line, "\t", 4)
				if len(row) != 4 {
					t.Fatalf("incomplete native row: %q", line)
				}
				if row[0] != api || row[1] != "isTest" || (row[2] != "page_getContent_htmlpage" && row[2] != "page_getContentAsPDF_htmlpage") {
					continue
				}
				var native string
				if err := json.Unmarshal([]byte(row[3]), &native); err != nil {
					t.Fatal(err)
				}
				for _, receiver := range []string{"new PageReference('/apex/ContentDate')", "Page.ContentDate"} {
					method := "getContent"
					if strings.Contains(row[2], "getContentAsPDF") {
						method = "getContentAsPDF"
					}
					call := receiver + "." + method + "()"
					body := "String observed; try { " + call + "; observed='rendered'; } catch(Exception e) { observed=e.getTypeName()+'|'+e.getMessage(); } System.assertEquals('" + native + "', observed);"
					fmt.Fprintf(&methods, "@IsTest static void content%d() { %s }\n", count, body)
					anonymous[fmt.Sprint(count)] = "Blob body=" + call + "; System.assert(body.size()>0);"
					count++
				}
			}
			if count != 4 {
				t.Fatalf("native method/receiver cells=%d, want 4", count)
			}
			methods.WriteString("}")
			write(base+"classes/ContentDateTest.cls", methods.String())
			write(base+"classes/ContentDateTest.cls-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, sch)
			t.Run("runner", func(t *testing.T) {
				run := apextest.Run(index, apextest.Options{SelectedClasses: []string{"ContentDateTest"}, NoDiskCache: true, Parallelism: 1})
				if summary := run.Summary(); summary.Total != count || summary.Passed != count {
					t.Fatalf("native test-context rejection: %+v; suites=%+v", summary, run.Suites)
				}
			})
			runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			for name, code := range anonymous {
				t.Run("anonymous/"+name, func(t *testing.T) {
					machine := vm.New(nil)
					if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
						t.Fatal(err)
					}
					visualforce.SetVMRenderEnvironment(machine, p)
					defer visualforce.ClearVMRenderEnvironment(machine)
					program, err := vm.CompileAnonymousWithOptions(code, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}
