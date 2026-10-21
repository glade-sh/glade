package apextest

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Each nested project defines its own helper and compiles independently.
func TestRuntimeKeepsDuplicateSourceUnitFieldsIndependent(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		name := "public-last"
		if reverse {
			name = "private-last"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			var paths []string
			for _, unit := range []string{"closed", "open"} {
				base := filepath.Join(root, unit)
				writeFile(t, filepath.Join(base, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
				helper := `public class UnitCatalog { private static Integer seed=1; public static Integer read(){return seed;} }`
				testName := "ClosedCatalogTest"
				testBody := `System.assertEquals(1,UnitCatalog.read());`
				if unit == "open" {
					helper = `public class UnitCatalog { public static Integer seed=2; public static Integer read(){return seed;} }`
					testName = "OpenCatalogTest"
					testBody = `System.assertEquals(2,UnitCatalog.seed);`
				}
				for className, source := range map[string]string{"UnitCatalog": helper, testName: "@IsTest private class " + testName + " { @IsTest static void check(){" + testBody + "} }"} {
					path := filepath.Join(base, "force-app", "classes", className+".cls")
					writeFile(t, path, source)
					paths = append(paths, path)
				}
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: paths}, schema.Schema{})
			sort.SliceStable(index.Types, func(i, j int) bool {
				a, b := index.Types[i], index.Types[j]
				if a.Name != b.Name {
					return a.Name < b.Name
				}
				openA := strings.Contains(a.File, string(filepath.Separator)+"open"+string(filepath.Separator))
				openB := strings.Contains(b.File, string(filepath.Separator)+"open"+string(filepath.Separator))
				if reverse {
					return openA && !openB
				}
				return !openA && openB
			})
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
				t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
			}
		})
	}
}

func TestRuntimeSourceUnitManifestInvalidatesGeneration(t *testing.T) {
	root := t.TempDir()
	var paths []string
	var marker string
	for _, unit := range []string{"first", "second"} {
		base := filepath.Join(root, unit)
		marker = filepath.Join(base, "sfdx-project.json")
		writeFile(t, marker, `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
		path := filepath.Join(base, "force-app", "classes", "UnitFlag.cls")
		writeFile(t, path, `public class UnitFlag { public static Integer seed=1; }`)
		paths = append(paths, path)
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: paths}, schema.Schema{})
	first, err := prepareRuntimeGeneration(index, nil, newSourceCache())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	second, err := prepareRuntimeGeneration(index, nil, newSourceCache())
	if err != nil {
		t.Fatal(err)
	}
	if first.key == second.key {
		t.Fatal("source-unit manifest removal reused runtime generation")
	}
}
