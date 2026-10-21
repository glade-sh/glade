package apextest

import (
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"path/filepath"
	"testing"
)

func TestRuntimeSourceUnitStaticFieldWritesIgnoreSpelling(t *testing.T) {
	root := t.TempDir()
	var paths []string
	for _, unit := range []string{"first", "second"} {
		base := filepath.Join(root, unit)
		writeFile(t, filepath.Join(base, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
		path := filepath.Join(base, "force-app", "classes", "UnitCounter.cls")
		writeFile(t, path, `public class UnitCounter { public static Integer seed=2; }`)
		paths = append(paths, path)
	}
	path := filepath.Join(root, "second", "force-app", "classes", "UnitCounterTest.cls")
	writeFile(t, path, `@IsTest private class UnitCounterTest { @IsTest static void check(){
 for(Integer i=0;i<32;i++) { UnitCounter.seed=i+10; System.assertEquals(i+10,UnitCounter.SEED); System.assertEquals(i+10,UnitCounter.sEeD); }
 } }`)
	paths = append(paths, path)
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: paths}, schema.Schema{})
	run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
	}
}
