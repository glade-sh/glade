package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Native rows: testdata/conformance/installed_package_reflection/org.tsv. A
// subscriber calls an installed managed-package owner that probes its own
// nested class literals through Type.newInstance. The owner source is the
// captured owner/CaptureOwner.cls; the dependency project stands in for the
// installed package.
func TestInstalledPackageReflectionConformance(t *testing.T) {
	const fixture = "testdata/conformance/installed_package_reflection"
	owner, err := os.ReadFile(filepath.Join(fixture, "owner", "CaptureOwner.cls"))
	if err != nil {
		t.Fatal(err)
	}
	table, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(table)), "\n")
	if len(lines) != 5 {
		t.Fatalf("org.tsv rows=%d, want header plus 4", len(lines))
	}
	for _, line := range lines[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 3 {
			t.Fatalf("bad row %q", line)
		}
		api, route, want := cells[0], cells[1], cells[2]
		t.Run(api+"/"+route, func(t *testing.T) {
			root := t.TempDir()
			dep, consumer := filepath.Join(root, "dep"), filepath.Join(root, "consumer")
			meta := "<ApexClass><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
			writeFile(t, filepath.Join(dep, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(dep, "force-app/main/classes/CaptureOwner.cls"), string(owner))
			writeFile(t, filepath.Join(dep, "force-app/main/classes/CaptureOwner.cls-meta.xml"), meta)
			writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"namespace":"","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  managedPackageDependencies: [\"pkg:../dep:1.0\"]\n")
			writeFile(t, filepath.Join(consumer, "force-app/main/classes/CaptureTest.cls"), `@IsTest private class CaptureTest {
  @IsTest static void observed() { System.assertEquals('`+want+`', pkg.CaptureOwner.run()); }
}`)
			writeFile(t, filepath.Join(consumer, "force-app/main/classes/CaptureTest.cls-meta.xml"), meta)
			index := loadTestIndex(t, consumer)
			switch route {
			case "anonymous":
				program, err := vm.CompileAnonymousWithOptions(`System.assertEquals('`+want+`', pkg.CaptureOwner.run());`, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true}).execute(program); err != nil {
					t.Fatal(err)
				}
			case "isTest":
				run := Run(index, Options{Parallelism: 1, SelectedClasses: []string{"CaptureTest"}})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 || s.RuntimeErrors != 0 || s.CompileErrors != 0 {
					t.Fatalf("summary=%+v problem=%s", s, firstRunProblem(run))
				}
			default:
				t.Fatalf("route %q", route)
			}
		})
	}
}
