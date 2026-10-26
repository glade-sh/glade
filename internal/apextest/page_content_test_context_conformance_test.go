package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPageReferenceContentTestContextNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/blob_pdf_rendering"
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeClass := func(name, source string) {
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), source)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			writeClass("PageContentHelper", `public class PageContentHelper {
 public static String observed;
 public static PageReference page;
 public static Boolean pdf;
 public static Blob render(PageReference p, Boolean asPDF) { if(asPDF) return p.getContentAsPDF(); return p.getContent(); }
}`)
			// Catch inside the trigger so DML wrapping cannot hide the exception type.
			writeFile(t, filepath.Join(root, "force-app/main/default/triggers/PageContentTrigger.trigger"), `trigger PageContentTrigger on Account (before insert) {
 try { PageContentHelper.render(PageContentHelper.page, PageContentHelper.pdf); PageContentHelper.observed = 'rendered'; }
 catch(Exception e) { PageContentHelper.observed = e.getTypeName() + '|' + e.getMessage(); }
}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/triggers/PageContentTrigger.trigger-meta.xml"), `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexTrigger>`)
			for _, page := range []string{"PdfProbePage", "PdfProbeHtmlPage"} {
				markup, err := os.ReadFile(filepath.Join(fixture, "project/main/default/pages", page+".page"))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/pages", page+".page"), string(markup))
			}
			var methods strings.Builder
			methods.WriteString("@IsTest private class PageContentContextTest {\n")
			anonymous := map[string]string{}
			count := 0
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				row := strings.SplitN(line, "\t", 4)
				if len(row) != 4 || row[0] != api || !strings.HasPrefix(row[2], "page_getContent") {
					continue
				}
				pdf := strings.Contains(row[2], "getContentAsPDF")
				method, page := "getContent", "PdfProbePage"
				if pdf {
					method = "getContentAsPDF"
				}
				if strings.HasSuffix(row[2], "htmlpage") {
					page = "PdfProbeHtmlPage"
				}
				var native string
				if row[1] == "isTest" {
					if json.Unmarshal([]byte(row[3]), &native) != nil || native != "System.VisualforceException|Methods defined as TestMethod do not support getContent call" {
						t.Fatalf("invalid native exception: %s", line)
					}
				} else if row[1] == "anonymous" {
					// Native PDF-page getContent succeeds; converting its bytes to
					// String is the separately captured StringException.
					var facets map[string]any
					if json.Unmarshal([]byte(row[3]), &facets) != nil {
						if json.Unmarshal([]byte(row[3]), &native) != nil || native != "System.StringException|BLOB is not a valid UTF-8 string" {
							t.Fatalf("invalid anonymous observation: %s", line)
						}
					}
				} else {
					t.Fatalf("unknown route: %s", row[1])
				}
				for _, entry := range []string{"pageToken", "constructor"} {
					reference := "Page." + page
					if entry == "constructor" {
						reference = "new PageReference('/apex/" + page + "')"
					}
					for _, route := range []string{"direct", "helper", "trigger"} {
						call := reference + "." + method + "()"
						if route == "helper" {
							call = fmt.Sprintf("PageContentHelper.render(%s, %t)", reference, pdf)
						}
						name := row[2] + "_" + entry + "_" + route
						if row[1] == "anonymous" {
							if route != "trigger" {
								anonymous[name] = "Blob body = " + call + "; System.assert(body.size() > 0);"
							}
							continue
						}
						body := "String observed; try { " + call + "; observed='rendered'; } catch(Exception e) { observed=e.getTypeName()+'|'+e.getMessage(); }"
						if route == "trigger" {
							body = fmt.Sprintf("PageContentHelper.page=%s; PageContentHelper.pdf=%t; insert new Account(Name='Content probe'); String observed=PageContentHelper.observed;", reference, pdf)
						}
						fmt.Fprintf(&methods, "@IsTest static void %s() { %s System.assertEquals('%s', observed); }\n", name, body, native)
						count++
					}
				}
			}
			if count != 24 || len(anonymous) != 16 {
				t.Fatalf("incomplete route matrix: tests=%d anonymous=%d", count, len(anonymous))
			}
			methods.WriteString("}")
			writeClass("PageContentContextTest", methods.String())
			index := loadTestIndex(t, root)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			run := Run(index, Options{SelectedClasses: []string{"PageContentContextTest"}, NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != count || summary.Passed != count {
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						if result.Problem != nil {
							t.Errorf("isTest/%s: %s: %s", result.MethodName, result.Problem.Type, result.Problem.Message)
						}
					}
				}
				t.Errorf("project route matrix: %+v", summary)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			for name, code := range anonymous {
				t.Run("anonymous/"+name, func(t *testing.T) {
					machine := runner.newMachine()
					machine.RegisterPageReference("PdfProbePage")
					machine.RegisterPageReference("PdfProbeHtmlPage")
					visualforce.SetVMRenderEnvironment(machine, p)
					t.Cleanup(func() { visualforce.ClearVMRenderEnvironment(machine) })
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
