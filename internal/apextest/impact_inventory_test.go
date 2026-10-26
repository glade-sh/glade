package apextest

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/trace"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestImpactInventoryProjectDependencyAndShimRoutes(t *testing.T) {
	root := t.TempDir()
	p := &project.Project{Root: root}
	dependency := &project.Project{Root: filepath.Join(root, "dependency")}
	shim := &project.Project{Root: filepath.Join(root, "shim")}
	p.ManagedPackageDependencies = []project.ManagedPackageDependency{{Project: dependency}}
	p.PackageShims = []project.PackageShim{{Project: shim}}
	// A repeated project must not hide another root or recurse forever.
	shim.PackageShims = []project.PackageShim{{Project: p}}
	for _, route := range []struct {
		name string
		p    *project.Project
	}{{"project", p}, {"dependency", dependency}, {"shim", shim}} {
		t.Run(route.name, func(t *testing.T) {
			writeFile(t, filepath.Join(route.p.Root, "cumulusci.yml"), "orgs:\n  scratch:\n    dev:\n      config_file: 'orgs/development.json'\n      config_file: orgs/not-created.json\n")
			writeFile(t, filepath.Join(route.p.Root, "cumulusci.template.yml"), "orgs:\n  scratch:\n    test:\n      config_file: \"orgs/test.JSON\"\n")
			for _, file := range []string{"config/project-scratch-def.json", "config/hc-project-scratch-def.json", "orgs/development.json", "orgs/test.JSON"} {
				writeFile(t, filepath.Join(route.p.Root, file), `{"features":["MultiCurrency"]}`)
			}
			inputs := impactSet{}
			impactProjectInputs(root, p, inputs, map[*project.Project]bool{})
			for _, file := range []string{"cumulusci.yml", "cumulusci.template.yml", "config/project-scratch-def.json", "config/hc-project-scratch-def.json", "orgs/development.json", "orgs/not-created.json", "orgs/test.JSON"} {
				want := impactPath(root, filepath.Join(route.p.Root, file))
				if _, ok := inputs[want]; !ok {
					t.Errorf("%s route omits loader input %s", route.name, want)
				}
			}
		})
	}
}

func TestImpactInventoryIncompleteCompletedUnexecutedAndEmptyRoutes(t *testing.T) {
	for _, snapshot := range []*project.Project{nil, {Root: t.TempDir()}} {
		name := "snapshot"
		if snapshot == nil {
			name = "no-snapshot"
		}
		for _, status := range []testreport.Status{testreport.StatusPass, testreport.StatusFail, testreport.StatusCompileError, testreport.StatusRuntimeError, testreport.StatusUnsupported, testreport.StatusSkipped, "empty"} {
			t.Run(name+"/"+string(status), func(t *testing.T) {
				run := testreport.Run{}
				if status != "empty" {
					run.Suites = []testreport.Suite{{Cases: []testreport.Case{{ClassName: "InventoryTests", MethodName: "check", Status: status, Trace: []trace.Event{{Name: "apex.method"}}}}}}
				}
				report := impactForRun(typesys.Index{}, snapshot, &run, Options{})
				data, err := json.Marshal(report)
				if err != nil {
					t.Fatal(err)
				}
				var contract struct {
					Complete *bool `json:"inputInventoryComplete"`
				}
				if err := json.Unmarshal(data, &contract); err != nil {
					t.Fatal(err)
				}
				if contract.Complete == nil || *contract.Complete {
					t.Errorf("unproven inventory must explicitly require full-suite selection: %s", data)
				}
				for _, row := range report.Tests {
					if !slices.Contains(row.FallbackReasons, "project-input-inventory-incomplete") {
						t.Errorf("completed trace or snapshot cannot prove complete inputs: %+v", row)
					}
					if snapshot == nil && !slices.Contains(row.FallbackReasons, "project-input-inventory-unavailable") {
						t.Errorf("lost unavailable inventory reason: %+v", row)
					}
				}
			})
		}
	}
}
