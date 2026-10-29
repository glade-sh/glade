package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The project index and linked preparation may be shared. Executing sibling
// rows must still match independent fresh projects, including error cleanup.
func TestConformanceProjectTemplateConcurrentExecution(t *testing.T) {
	cache := newConformanceProjectCache(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			name := "SharedSetupProbe" + strings.TrimSuffix(api, ".0")
			path := "force-app/main/default/classes/" + name + ".cls"
			source, err := os.ReadFile(filepath.Join("testdata/conformance/api_parallel_cases/native-project", path))
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{path: string(source), path + "-meta.xml": "<ApexClass><apiVersion>" + api + "</apiVersion></ApexClass>",
				"force-app/main/default/classes/ParallelState.cls": `public class ParallelState { public static Integer calls=0; public static List<String> entries=new List<String>{'seed'}; }`}
			index := cache.index(t, api, files)
			org := orgFromIndex(index)
			org.APIVersion = api
			request, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, route := range []string{"test", "request"} {
				t.Run(route, func(t *testing.T) {
					options := conformanceRunnerOptions{LinkProject: route == "test", Org: &org}
					if route == "request" {
						options.RequestRuntime = &request
					}
					shared := newConformanceRunner(t, index, options)
					for row := 0; row < 4; row++ {
						runConformanceSubtestParallel(t, fmt.Sprint(row), func(t *testing.T) {
							cached := cache.index(t, api, files)
							fresh := newConformanceProjectCache(t).index(t, api, files)
							clean := `System.debug('NATIVE|' + ` + name + `.value()); System.debug(ParallelState.calls); System.debug(ParallelState.entries); System.debug([SELECT COUNT() FROM Account WHERE Name='parallel-row']); System.debug(UserInfo.getUserId());`
							dirty := clean + ` ParallelState.calls++; ParallelState.entries.add('dirty'); insert new Account(Name='parallel-row'); Savepoint s=Database.setSavepoint(); insert new Account(Name='rollback-row'); Database.rollback(s); System.debug(Limits.getDmlStatements());`
							freshOrg := orgFromIndex(fresh)
							freshOrg.APIVersion = api
							freshOptions := conformanceRunnerOptions{LinkProject: route == "test", Org: &freshOrg}
							freshRuntime, err := CompileProjectRuntimeForRequestWithSourceDigests(fresh, nil)
							if err != nil {
								t.Fatal(err)
							}
							if route == "request" {
								freshOptions.RequestRuntime = &freshRuntime
							}
							independent := newConformanceRunner(t, fresh, freshOptions)
							for _, body := range []string{dirty, dirty + ` Object absent=null; System.debug(absent.toString());`, clean} {
								a := sema.AnalyzeAnonymous(cached, body, api)
								b := sema.AnalyzeAnonymous(fresh, body, api)
								normalize := func(v any, root string) string {
									raw, err := json.Marshal(v)
									if err != nil {
										t.Fatal(err)
									}
									return strings.ReplaceAll(string(raw), root, "<project>")
								}
								if normalize(a.Diagnostics, cached.Project.Root) != normalize(b.Diagnostics, fresh.Project.Root) {
									t.Fatal("shared preparation changed diagnostics")
								}
								program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								got, gotErr := shared.execute(program)
								want, wantErr := independent.execute(program)
								got.Limits.CPUTimeMS = 0
								want.Limits.CPUTimeMS = 0
								// Independent VM allocations must have different private Ref IDs.
								// Compare observable values, then prove the savepoint identities differ.
								gotVars := normalize(got.Vars, cached.Project.Root)
								wantVars := normalize(want.Vars, fresh.Project.Root)
								if savepoint, ok := got.Vars["s"]; ok && (savepoint.Ref == 0 || savepoint.Ref == want.Vars["s"].Ref) {
									t.Fatal("independent rows shared a savepoint reference identity")
								}
								if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) || !reflect.DeepEqual(got.Debug, want.Debug) || !reflect.DeepEqual(got.Limits, want.Limits) || gotVars != wantVars {
									t.Fatalf("shared/fresh row mismatch: debug=%q/%q limits=%+v/%+v vars=%s/%s errors=%v/%v", got.Debug, want.Debug, got.Limits, want.Limits, gotVars, wantVars, gotErr, wantErr)
								}
								// Preserved native rows: api_parallel_cases/native/api{62,67}/org.tsv:2.
								if !strings.Contains(strings.Join(got.Debug, "\n"), "NATIVE|ready") {
									t.Fatal("lost native observation")
								}
							}
						})
					}
				})
			}
		})
	}
}
