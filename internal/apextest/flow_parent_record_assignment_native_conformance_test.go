package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowParentRecordAssignmentNativeConformance(t *testing.T) {
	runFlowRecordAssignmentNativeConformance(t, "flow_parent_record_assignment", "GroveParentProbe", "GroveParentProbeTest", "observeParent", "Grove_Parent", "FLOW_PARENT_RECORD_ASSIGNMENT", []string{"R01", "R02", "R03", "R04"})
}

func TestFlowNullLookupUpdateNativeConformance(t *testing.T) {
	runFlowRecordAssignmentNativeConformance(t, "flow_null_lookup_update", "GroveLookupProbe", "GroveLookupProbeTest", "observeLookup", "Grove_Lookup", "FLOW_NULL_LOOKUP_UPDATE", []string{"N01", "N02"})
}

func runFlowRecordAssignmentNativeConformance(t *testing.T, directory, helper, wrapper, method, flow, group string, ids []string, builders ...func(*testing.T, string, map[string]string, map[string]string) typesys.Index) {
	t.Helper()
	fixture := filepath.Join("testdata/conformance", directory)
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	anonymous := read("anonymous.apex")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := "api" + strings.TrimSuffix(api, ".0")
			classes, files, sources := map[string]string{}, map[string]string{}, map[string]string{}
			for _, name := range []string{helper, wrapper} {
				for _, suffix := range []string{".cls", ".cls-meta.xml"} {
					path := "classes/" + name + suffix
					sources[path] = read(version + "/" + path)
					files["force-app/main/default/"+path] = sources[path]
				}
				classes[name] = sources["classes/"+name+".cls"]
			}
			path := "flows/" + flow + ".flow-meta.xml"
			sources[path] = read(version + "/" + path)
			files["force-app/main/default/"+path] = sources[path]
			buildIndex := flowRefsIndex
			if len(builders) != 0 {
				buildIndex = builders[0]
			}
			index := buildIndex(t, api, classes, files)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					root := version + "/native/" + route + "/"
					run := read(root + "RUN-FACTS.txt")
					for key, value := range map[string]string{"api": api, "context": route, "rows": fmt.Sprint(len(ids)), "native_complete": "true", "deployment_success": "true"} {
						if !strings.Contains("\n"+run, "\n"+key+"="+value+"\n") {
							t.Fatalf("missing capture fact %s", key)
						}
					}
					for path, source := range sources {
						if !strings.Contains(run, fmt.Sprintf("source_sha256:%s=%x\n", path, sha256.Sum256([]byte(source)))) {
							t.Fatalf("capture source mismatch %s", path)
						}
					}
					if !strings.Contains(run, fmt.Sprintf("anonymous_source_sha256=%x\n", sha256.Sum256([]byte(anonymous)))) {
						t.Fatal("anonymous capture source mismatch")
					}
					lines := strings.Split(strings.TrimSuffix(read(root+"org.tsv"), "\n"), "\n")
					if len(lines) != len(ids)+1 || lines[0] != "id\tgroup\troute\tstatus\tvalue" {
						t.Fatal("incomplete native rows")
					}
					want := map[string]map[string]any{}
					for _, line := range lines[1:] {
						row := strings.SplitN(line, "\t", 5)
						if len(row) != 5 || row[1] != group || row[2] != route || row[3] != "runtime" {
							t.Fatalf("invalid native row %q", line)
						}
						var cell map[string]any
						if err := json.Unmarshal([]byte(row[4]), &cell); err != nil {
							t.Fatal(err)
						}
						if _, duplicate := want[row[0]]; duplicate {
							t.Fatal("duplicate native row")
						}
						want[row[0]] = cell
					}
					var message string
					if route == "anonymous" {
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						analysis := sema.AnalyzeAnonymous(index, anonymous, api)
						program, err := vm.CompileAnonymousWithOptions(anonymous, vm.CompileOptions{APIVersion: api})
						if err != nil || analysis.HasErrors() {
							t.Fatalf("native anonymous accepted: %v %v", err, analysis.Diagnostics)
						}
						_, err = runner.newMachine().Execute(program)
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || flowRefsExceptionType(runtimeErr.Type) != "System.AssertException" {
							t.Fatalf("missing observation assertion: %v", err)
						}
						message = runtimeErr.ExceptionMessage()
					} else {
						result := Run(index, Options{SelectedClasses: []string{wrapper}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
						if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
							t.Fatalf("unexpected test selection: %#v", result.Summary())
						}
						observed := result.Suites[0].Cases[0]
						if observed.Problem == nil || flowRefsExceptionType(observed.Problem.Type) != "System.AssertException" {
							t.Fatalf("missing observation assertion: %#v; problem: %#v", observed, observed.Problem)
						}
						message = observed.Problem.Message
					}
					_, payload, found := strings.Cut(message, "GROVE_RESULT|")
					if !found || strings.Count(message, "GROVE_RESULT|") != 1 {
						t.Fatalf("missing observation envelope: %s", message)
					}
					var got map[string]map[string]any
					if err := json.Unmarshal([]byte(payload), &got); err != nil || len(got) != len(ids) {
						t.Fatalf("invalid observation: %s (%v)", payload, err)
					}
					for _, id := range ids {
						t.Run(id, func(t *testing.T) {
							if !reflect.DeepEqual(got[id], want[id]) {
								t.Fatalf("native %#v; Glade %#v", want[id], got[id])
							}
						})
					}
				})
			}
		})
	}
}
