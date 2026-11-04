package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestNamespacedDependencyPrivateNestedReflectionAnonymous(t *testing.T) {
	root := t.TempDir()
	dep := filepath.Join(root, "dep")
	consumer := filepath.Join(root, "consumer")
	writeFile(t, filepath.Join(dep, "sfdx-project.json"), `{"namespace":"pkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(dep, "force-app/main/classes/Factory.cls"), `global virtual class Factory {
  private class First extends Factory { private First() {} }
  private class Second extends Factory { private Second() {} }
  global static String make() {
    Type a = First.class; Type b = Second.class;
    String left; String right;
    try { a.newInstance(); left = 'constructed'; } catch (Exception e) { left = e.getTypeName(); }
    try { b.newInstance(); right = 'constructed'; } catch (Exception e) { right = e.getTypeName(); }
    return left + ':' + right;
  }
}`)
	writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"namespace":"otherpkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  managedPackageDependencies: [\"pkg:../dep:1.0\"]\n")
	writeFile(t, filepath.Join(consumer, "force-app/main/classes/Factory.cls"), `public class Factory { public static String make() { return 'local'; } }`)
	writeFile(t, filepath.Join(consumer, "force-app/main/classes/FactoryTest.cls"), `@IsTest private class FactoryTest {
  @IsTest static void rejectPrivateConstructors() {
    System.assertEquals('System.TypeException:System.TypeException', pkg.Factory.make());
  }
}`)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			meta := "<ApexClass><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
			writeFile(t, filepath.Join(dep, "force-app/main/classes/Factory.cls-meta.xml"), meta)
			writeFile(t, filepath.Join(consumer, "force-app/main/classes/Factory.cls-meta.xml"), meta)
			writeFile(t, filepath.Join(consumer, "force-app/main/classes/FactoryTest.cls-meta.xml"), meta)
			index := loadTestIndex(t, consumer)
			program, err := vm.CompileAnonymousWithOptions(`System.assertEquals('System.TypeException:System.TypeException', pkg.Factory.make());`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			for i := 0; i < 2; i++ {
				if _, err := runner.execute(program); err != nil {
					t.Fatal(err)
				}
			}
			run := Run(index, Options{Parallelism: 1, SelectedClasses: []string{"FactoryTest"}})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.RuntimeErrors != 0 || summary.CompileErrors != 0 {
				t.Fatalf("test route summary=%+v problem=%s", summary, firstRunProblem(run))
			}
		})
	}
}
