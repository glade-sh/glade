package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestIPv4ValidationProjectAnonymousAndIsTestRoutes(t *testing.T) {
	t.Logf("GOOS=%s GOARCH=%s", runtime.GOOS, runtime.GOARCH)
	for _, namespace := range []string{"", "netprobe"} {
		for _, api := range []string{"62.0", "67.0"} {
			for _, route := range []string{"anonymous", "isTest"} {
				t.Run(fmt.Sprintf("namespace=%s/%s/%s", namespace, api, route), func(t *testing.T) {
					root := t.TempDir()
					for _, class := range []string{"IPv4BoundaryProbe", "IPv4BoundaryProbeTest"} {
						dir := filepath.Join("testdata", "ipv4_validation", "project", "force-app", "main", "default", "classes")
						for _, suffix := range []string{".cls", ".cls-meta.xml"} {
							body, err := os.ReadFile(filepath.Join(dir, class+suffix))
							if err != nil {
								t.Fatal(err)
							}
							writeFile(t, filepath.Join(root, "force-app", "main", "default", "classes", class+suffix), strings.ReplaceAll(string(body), "62.0", api))
						}
					}
					writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"namespace":%q,"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, namespace, api))
					index := loadTestIndex(t, root)
					analysis := sema.Analyze(index)
					if index.HasErrors() || analysis.HasErrors() {
						t.Fatalf("index=%v sema=%v", index.Diagnostics, analysis.Diagnostics)
					}
					if route == "isTest" {
						result := Run(index, Options{SelectedClasses: []string{"IPv4BoundaryProbeTest"}, SelectedMethod: "validateAddresses", NoDiskCache: true, Parallelism: 1})
						if summary := result.Summary(); summary.Total != 1 || summary.Passed != 1 || !summary.OK() {
							t.Fatalf("project test summary=%+v problem=%s", summary, firstRunProblem(result))
						}
						return
					}
					program, err := vm.CompileAnonymousWithOptions(`IPv4BoundaryProbe.validateAddresses();`, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					if _, err := runner.execute(program, func(machine *vm.VM) {
						machine.SetCurrentNamespace(namespace)
					}); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
