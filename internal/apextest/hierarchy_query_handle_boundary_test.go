package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Local preservation controls only: handle behavior is not in the native rows.
func TestHierarchyQueryBoundaryQueryHandleProjectPreservation(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("testdata/conformance/hierarchy_query_boundary/project")); err != nil {
				t.Fatal(err)
			}
			classes := filepath.Join(root, "force-app", "main", "default", "classes")
			var calls strings.Builder
			for _, sharing := range []string{"with sharing", "without sharing"} {
				name := "HandleSharedReader"
				if sharing == "without sharing" {
					name = "HandleSystemReader"
				}
				var methods strings.Builder
				for _, mode := range []string{"USER_MODE", "SYSTEM_MODE", "DEFAULT"} {
					for _, route := range []struct{ name, kind, call, read string }{
						{"locator", "QueryLocator", "getQueryLocator(q,AccessLevel.%s)", "h.iterator().hasNext()?1:0"},
						{"locatorWithBinds", "QueryLocator", "getQueryLocatorWithBinds(q,new Map<String,Object>(),AccessLevel.%s)", "h.iterator().hasNext()?1:0"},
						{"cursor", "Cursor", "getCursor(q,AccessLevel.%s)", "h.getNumRecords()"},
						{"cursorWithBinds", "Cursor", "getCursorWithBinds(q,new Map<String,Object>(),AccessLevel.%s)", "h.getNumRecords()"},
						{"paginationCursor", "PaginationCursor", "getPaginationCursor(q,AccessLevel.%s)", "h.getNumRecords()"},
						{"paginationCursorWithBinds", "PaginationCursor", "getPaginationCursorWithBinds(q,new Map<String,Object>(),AccessLevel.%s)", "h.getNumRecords()"},
					} {
						// The semantic API 62 profile excludes the unbound pagination surface.
						// Its runtime preservation is covered by the direct VM test.
						if api == "62.0" && route.name == "paginationCursor" {
							continue
						}
						if mode == "DEFAULT" && strings.Contains(route.name, "WithBinds") {
							continue
						}
						method := route.name + mode
						call := fmt.Sprintf(route.call, mode)
						if mode == "DEFAULT" {
							call = strings.ReplaceAll(route.call, ",AccessLevel.%s", "")
						}
						fmt.Fprintf(&methods, "public static Integer %s(){String q='SELECT Disabled__c FROM BoundaryPreference__c';Database.%s h=Database.%s;return %s;}\n", method, route.kind, call, route.read)
						want := 0
						effectiveMode := mode
						if mode == "DEFAULT" || route.name == "paginationCursor" {
							effectiveMode = "USER_MODE"
							if api == "62.0" {
								effectiveMode = "SYSTEM_MODE"
							}
						}
						if sharing == "without sharing" && effectiveMode == "SYSTEM_MODE" {
							want = 1
						}
						fmt.Fprintf(&calls, "System.assertEquals(%d,%s.%s(),'%s/%s/%s');\n", want, name, method, sharing, mode, route.name)
					}
				}
				writeFile(t, filepath.Join(classes, name+".cls"), "public "+sharing+" class "+name+"{"+methods.String()+"}")
				writeFile(t, filepath.Join(classes, name+".cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			}
			source := `@IsTest private class HandleBoundaryTest {
 @IsTest static void handles() {
 insert new BoundaryPreference__c(SetupOwnerId=UserInfo.getOrganizationId(),Disabled__c=true);
 User reader=new User(Alias='handle',Email='handle@example.test',EmailEncodingKey='UTF-8',LastName='Handle',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id,Username='handle.'+UserInfo.getOrganizationId()+'@example.test');
 insert reader;
 System.runAs(reader){` + calls.String() + `}
 }
}`
			writeFile(t, filepath.Join(classes, "HandleBoundaryTest.cls"), source)
			writeFile(t, filepath.Join(classes, "HandleBoundaryTest.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"HandleBoundaryTest"}, SelectedMethod: "handles"})
			if len(cases) != 1 {
				t.Fatal("missing handle observer")
			}
			methods, methodErrors := compileTestMethods(cases)
			key := testCaseKey(cases[0])
			if methodErrors[key] != nil {
				t.Fatal(methodErrors[key])
			}
			t.Run("isTestRunner", func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"HandleBoundaryTest"}, SelectedMethod: "handles", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("discovery: %+v", run.Summary())
				}
				if problem := run.Suites[0].Cases[0].Problem; problem != nil {
					t.Fatalf("handle preservation: %+v", problem)
				}
			})
			for _, entry := range []string{"anonymousRequestRuntime", "namedVM"} {
				t.Run(entry, func(t *testing.T) {
					org := orgFromIndex(index)
					org.APIVersion = api
					runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
					if err != nil {
						t.Fatal(err)
					}
					options := conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org}
					if entry == "namedVM" {
						options = conformanceRunnerOptions{LinkProject: true, Org: &org}
					}
					runner := newConformanceRunner(t, index, options)
					// runAs is a test construct, so retain the test context for
					// both project runtime entry points.
					machine := runner.newMachine()
					machine.EnableTestContext()
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					program, err := vm.CompileAnonymousWithOptions("HandleBoundaryTest.handles();", vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					var executionError error
					if entry == "namedVM" {
						_, executionError = machine.ExecuteInClass(program, "HandleBoundaryTest")
					} else {
						_, executionError = machine.Execute(program)
					}
					if err := executionError; err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}
