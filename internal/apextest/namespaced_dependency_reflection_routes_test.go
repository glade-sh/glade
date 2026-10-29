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
    return String.valueOf(a.newInstance() instanceof First) + ':' + String.valueOf(b.newInstance() instanceof Second);
  }
}`)
	writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"namespace":"otherpkg","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  managedPackageDependencies: [\"pkg:../dep:1.0\"]\n")
	writeFile(t, filepath.Join(consumer, "force-app/main/classes/Factory.cls"), `public class Factory { public static String make() { return 'local'; } }`)
	index := loadTestIndex(t, consumer)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := vm.CompileAnonymousWithOptions(`System.assertEquals('true:true', pkg.Factory.make());`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			for i := 0; i < 2; i++ {
				if _, err := runner.execute(program); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
