package typesys_test

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// A source-free inner job must never reach its unavailable execute body during
// stopTest. The top-level artifact uses a configured shim and must still run.
func TestArtifactScheduledNestingNativeNamedAndRunnerRoutes(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	const fixture = "testdata/conformance/artifact_scheduled_nesting"
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	var data struct {
		APIVersions, Methods, Routes []string
		AnonymousBoundary            string
		SourceSHA256                 map[string]string
	}
	if err := json.Unmarshal(read("cases.json"), &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || strings.Join(data.Methods, ",") != "innerArtifact,topArtifact" ||
		strings.Join(data.Routes, ",") != "namedIsTest,runner" || data.AnonymousBoundary == "" || len(data.SourceSHA256) != 3 {
		t.Fatal("incomplete artifact schedule matrix")
	}
	sources := map[string]string{}
	for name, digest := range data.SourceSHA256 {
		source := read("sources/" + name + ".cls")
		if fmt.Sprintf("%x", sha256.Sum256(source)) != digest {
			t.Fatalf("native source binding changed: %s", name)
		}
		sources[name] = string(source)
	}
	reader := csv.NewReader(strings.NewReader(string(read("org.tsv"))))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 || strings.Join(rows[0], ",") != "api,method,observation" {
		t.Fatalf("incomplete native rows: %v", err)
	}
	want := map[string]string{}
	for _, row := range rows[1:] {
		if len(row) != 3 {
			t.Fatalf("invalid native row: %v", row)
		}
		key := row[0] + "/" + row[1]
		if want[key] != "" || !strings.HasPrefix(row[2], "OBS effects=") {
			t.Fatalf("invalid native row: %v", row)
		}
		want[key] = row[2]
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			write := func(path, content string) {
				t.Helper()
				path = filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			write("glade.yml", "project:\n  managedPackageDependencies: [\"meadow:artifact:packages/meadow.json:1.0\"]\n  packageShims: [\"meadow:shims\"]\n")
			write("shims/sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"classes","default":true}],"sourceApiVersion":%q}`, api))
			write("shims/classes/MeadowTopSchedule.cls", strings.ReplaceAll(sources["MeadowTopSchedule"], "public ", "global "))
			// Native has no package namespace; only qualify the two construction sites.
			wrapper := strings.ReplaceAll(sources["MeadowArtifactScheduleTest"], "new Meadow", "new meadow.Meadow")
			write("force-app/classes/MeadowArtifactScheduleTest.cls", wrapper)
			meta := fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api)
			write("force-app/classes/MeadowArtifactScheduleTest.cls-meta.xml", meta)
			write("shims/classes/MeadowTopSchedule.cls-meta.xml", meta)
			execute := packageartifact.ApexMember{
				Kind: apexast.DeclarationMethod, Name: "execute", Type: "void", Modifiers: []string{"global"},
				Parameters: []apexast.Parameter{{Name: "context", Type: "SchedulableContext"}},
			}
			artifact, err := packageartifact.BuildCaptured(packageartifact.BuildCapturedOptions{
				Namespace: "meadow", Version: "1.0", SourceAPIVersion: api,
				Capture: packageartifact.CaptureProvenance{Source: "synthetic"},
				ApexTypes: []packageartifact.ApexType{
					{Kind: apexast.DeclarationClass, Name: "MeadowScheduleContainer", Namespace: "meadow", Modifiers: []string{"global"}},
					{Kind: apexast.DeclarationClass, Name: "MeadowScheduleContainer.InnerJob", Namespace: "meadow", Modifiers: []string{"global"}, Interfaces: []string{"Schedulable"}, Members: []packageartifact.ApexMember{execute}},
					{Kind: apexast.DeclarationClass, Name: "MeadowTopSchedule", Namespace: "meadow", Modifiers: []string{"global"}, Interfaces: []string{"Schedulable"}, Members: []packageartifact.ApexMember{execute}},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			write("packages/meadow.json", "")
			if err := packageartifact.WriteJSON(filepath.Join(root, "packages/meadow.json"), artifact); err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("synthetic project rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			artifactCount := 0
			for _, typ := range index.Types {
				if typ.Artifact {
					artifactCount++
					if typ.HasSourceSnapshot() {
						t.Fatal("artifact unexpectedly has source")
					}
				}
			}
			if artifactCount != 3 {
				t.Fatalf("artifact types loaded = %d; want 3", artifactCount)
			}
			for _, route := range data.Routes {
				for _, method := range data.Methods {
					t.Run(route+"/"+method, func(t *testing.T) {
						expected := want[api+"/"+method]
						if expected == "" {
							t.Fatal("missing native observation")
						}
						if route == "runner" {
							opts := apextest.Options{NoDiskCache: true, SelectedClasses: []string{"MeadowArtifactScheduleTest"}, SelectedMethod: method}
							run := apextest.RunCasesContext(context.Background(), index, opts, apextest.Discover(index, opts))
							if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
								t.Fatalf("runner selection: %+v", run)
							}
							problem := run.Suites[0].Cases[0].Problem
							if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") || !strings.HasSuffix(problem.Message, expected) {
								t.Fatalf("native=%q; runner=%+v", expected, problem)
							}
							return
						}
						runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
						if err != nil {
							t.Fatal(err)
						}
						classNesting := map[string]bool{}
						for _, class := range runtime.Classes {
							if class.Namespace == "meadow" {
								classNesting[class.Name] = class.Nested
							}
						}
						for name, nested := range map[string]bool{"MeadowScheduleContainer": false, "MeadowScheduleContainer.InnerJob": true, "MeadowTopSchedule": false} {
							if got, found := classNesting[name]; !found || got != nested {
								t.Errorf("artifact runtime class %s Nested=%v, found=%v; want %v", name, got, found, nested)
							}
						}
						machine := vm.New(nil)
						if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
							t.Fatal(err)
						}
						org := apextest.OrgFromIndex(index)
						org.APIVersion = api
						machine.SetOrg(&org)
						machine.EnableTestContext()
						if err := apextest.RegisterTestMethodForRequest(machine, index, "MeadowArtifactScheduleTest", method); err != nil {
							t.Fatal(err)
						}
						program, err := vm.CompileAnonymousWithOptions("MeadowArtifactScheduleTest."+method+"();", vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = machine.ExecuteTestInClass(program, "MeadowArtifactScheduleTest")
						var fault *vm.RuntimeError
						if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") || !strings.HasSuffix(fault.ExceptionMessage(), expected) {
							t.Fatalf("native=%q; named test=%v", expected, err)
						}
					})
				}
			}
		})
	}
}
