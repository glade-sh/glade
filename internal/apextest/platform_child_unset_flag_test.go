package apextest

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

// The synthetic project walks a parent's child edges before describing the
// selected token. SQLite preserves one omitted facet; each invocation describes
// it twice so first and cached answers must both match the native capture.
func TestPlatformChildUnsetFlagPersistedProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, observation := range nativeStandardChildFieldRows(t, api, "anonymous") {
			row := strings.Split(observation, "|")
			flag, method, column := "", "", 0
			switch row[0] + "/" + row[1] {
			case "Opportunity/Orders":
				flag, method, column = "accessible", "isAccessible", 11
			case "Account/FinanceBalanceSnapshots":
				flag, method, column = "createable", "isCreateable", 7
			case "Account/ParentRecords":
				flag, method, column = "nillable", "isNillable", 6
			default:
				continue
			}
			for _, tokenRoute := range []string{"relationship-token", "child-field-map"} {
				t.Run(api+"/"+row[2]+"/"+flag+"/"+tokenRoute, func(t *testing.T) {
					expression := "edge.getField()"
					if tokenRoute == "child-field-map" {
						expression = "edge.getChildSObject().getDescribe().fields.getMap().get('" + strings.ToLower(row[3]) + "')"
					}
					body := fmt.Sprintf(`
for (Integer pass = 0; pass < 2; pass++) {
    Integer found = 0;
    for (Schema.ChildRelationship edge : Schema.getGlobalDescribe().get('%s').getDescribe().getChildRelationships()) {
        if (edge.getRelationshipName() == '%s') {
            found++;
            System.assertEquals(%s, %s.getDescribe().%s(), 'native %s.%s %s describe pass ' + pass);
        }
    }
    System.assertEquals(1, found);
}`, row[0], row[1], row[column], expression, method, row[2], row[3], flag)
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
					for name, source := range map[string]string{
						"ChildEdgeFlagProbe":     "public class ChildEdgeFlagProbe { public static void check() {" + body + "} }",
						"ChildEdgeFlagProbeTest": "@IsTest private class ChildEdgeFlagProbeTest { @IsTest static void check() { ChildEdgeFlagProbe.check(); } }",
					} {
						path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
						writeFile(t, path, source)
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					}
					index := loadTestIndex(t, root)
					if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
						t.Fatalf("project compilation: %v %v", index.Diagnostics, analysis.Diagnostics)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					storage.EnsureStandardObject(&org, row[2])
					state := org.Objects[row[2]]
					state.Definition = state.Definition.Clone()
					field := state.Definition.Fields[row[3]]
					switch flag {
					case "accessible":
						field.Accessible = nil
					case "createable":
						field.Createable = nil
					case "nillable":
						field.Nillable = nil
					}
					state.Definition.Fields[row[3]] = field
					org.Objects[row[2]] = state
					store, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "org.db"))
					if err != nil {
						t.Fatal(err)
					}
					defer store.Close()
					if err := store.Save(org); err != nil {
						t.Fatal(err)
					}
					loaded, err := store.Load()
					if err != nil {
						t.Fatal(err)
					}
					saved := loaded.Objects[row[2]].Definition.Fields[row[3]]
					flags := map[string]*bool{"accessible": saved.Accessible, "createable": saved.Createable, "nillable": saved.Nillable}
					if flags[flag] != nil {
						t.Fatal("SQLite fixture lost the unset facet")
					}
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &loaded, LinkProject: true})
					for route, source := range map[string]string{"anonymous": body, "named": "ChildEdgeFlagProbe.check();"} {
						t.Run(route, func(t *testing.T) {
							if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
								t.Fatal(analysis.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							if _, err := runner.execute(program); err != nil {
								t.Fatal(err)
							}
						})
					}
					t.Run("istest-runner", func(t *testing.T) {
						cases := Discover(index, Options{SelectedClasses: []string{"ChildEdgeFlagProbeTest"}})
						if len(cases) != 1 {
							t.Fatalf("discovered %d cases", len(cases))
						}
						methods, methodErrors := compileTestMethodsForIndex(&index, cases)
						programs, programErrors := compileTestInvokePrograms(cases)
						runtimeMethods := indexTestRuntimeMethods(methods)
						tc := cases[0]
						key := testCaseKey(tc)
						seed := runner.newOrg()
						initializeTestOrg(&seed)
						result := runCase(context.Background(), tc, methods[key], runtimeMethods[testMethodSourceKey(tc.ClassName, tc.File)], methodErrors[key], programs[key], programErrors[key], runner.base, nil, nil, nil, seed, 0, Options{NoDiskCache: true, Parallelism: 1}, false, nil, newRunPerfCounters(false))
						if result.Status != testreport.StatusPass {
							t.Fatalf("runner: %+v", result.Problem)
						}
					})
				})
			}
		}
	}
}
