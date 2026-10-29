package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Offline release compatibility: missing static calls are compile exclusions,
// not implementations of a managed package. Native missing_static rows are in
// testdata/conformance/unloaded_dependency/api{62,67}/org.tsv:2 and ORG-RUN.txt.
func TestManagedDependencyAbsentStaticCallReleaseBoundary(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, dependency := range []string{"zzns", "BoundaryPackage"} {
			for _, receiver := range []string{"zzns.SomeApi", "zzns.OtherApi", "zzns.Tools.Dispatcher"} {
				for _, route := range []string{"istest", "named", "guarded"} {
					t.Run(api+"/"+dependency+"/"+receiver+"/"+route, func(t *testing.T) {
						root := t.TempDir()
						writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":%q}]}]}`, api, dependency))
						write := func(name, source string) {
							path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
							writeFile(t, path, source)
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						}
						body := receiver + ".run('owned');"
						if route == "named" {
							write("BoundaryHelper", "public class BoundaryHelper { public static void observe() { "+body+" } }")
							body = "BoundaryHelper.observe();"
						} else if route == "guarded" {
							body = "if (false) { " + body + " }"
						}
						write("BoundaryTest", "@IsTest private class BoundaryTest { @IsTest static void observe() { "+body+" System.assert(true); } }")
						write("IndependentTest", "@IsTest private class IndependentTest { @IsTest static void observe() { Type value = Type.forName('zzns.SomeApi'); System.assertEquals(null, value); String zzns = 'owned'; System.assertEquals(5, zzns.length()); } }")
						index := loadTestIndex(t, root)
						for _, warm := range []bool{false, true} {
							run := Run(index, Options{Parallelism: 1})
							summary := run.Summary()
							wantExcluded := 1
							if route == "named" {
								wantExcluded = 2
							}
							if summary.Total != 2 || summary.Passed != 1 || summary.CompileErrors != 1 || summary.RuntimeErrors != 0 || summary.Unsupported != 0 || len(run.ExcludedClasses) != wantExcluded {
								t.Fatalf("warm=%v summary=%+v excluded=%+v problem=%s", warm, summary, run.ExcludedClasses, firstRunProblem(run))
							}
							for _, entry := range run.ExcludedClasses {
								if entry.ClassName == "IndependentTest" || !strings.Contains(entry.Message, "dependency_missing") {
									t.Fatalf("release dependency scope lost: %+v", entry)
								}
							}
						}
					})
				}
			}
		}
	}
}
