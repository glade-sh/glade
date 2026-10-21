package apextest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// An unbound dependency is a loader composition, not a native duplicate-class
// declaration. The two bound arithmetic bodies reuse the native 23/25 cells.
func TestInstanceInitializerCollisionIgnoresUnboundDeclarations(t *testing.T) {
	root := t.TempDir()
	paths := map[string]string{}
	fileRoots := map[string]string{}
	var files []string
	for _, name := range []string{"A", "B", "C", "U"} {
		unit := name
		if name == "C" {
			unit = "A"
		}
		base := filepath.Join(root, unit)
		writeFile(t, filepath.Join(base, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
		path := filepath.Join(base, "force-app", name, "OwnedMixedInitializer.cls")
		source := `public class OwnedMixedInitializer {}`
		if name != "U" {
			first, second := 3, 7
			if name == "B" {
				first, second = 5, 9
			}
			source = fmt.Sprintf(`public class OwnedMixedInitializer {
 public final Integer OFFSET=%d;
 public final Integer SCALE=%d;
 public final Integer VALUE=(OFFSET+SCALE)*SCALE/OFFSET;
}`, first, second)
		}
		writeFile(t, path, source)
		paths[name] = path
		fileRoots[path] = base
		files = append(files, path)
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: files}, schema.Schema{})
	byFile := map[string]typesys.TypeSymbol{}
	for _, typ := range index.Types {
		if typ.Name == "OwnedMixedInitializer" && typ.HasSourceSnapshot() {
			typ.SourceRoot = fileRoots[typ.File]
			byFile[typ.File] = typ
		}
	}
	if len(byFile) != 4 {
		t.Fatalf("owned declarations=%d, want 4", len(byFile))
	}
	for _, tc := range []struct {
		order     string
		collision bool
	}{
		{"AUB", true}, {"BUA", true}, {"UAB", true},
		{"UBA", true}, {"ABU", true}, {"BAU", true},
		{"AUC", false}, {"UAC", false}, {"ACU", false},
	} {
		t.Run(tc.order, func(t *testing.T) {
			ordered := index
			ordered.Types = nil
			for _, letter := range tc.order {
				typ := byFile[paths[string(letter)]]
				typ.Dependency = letter == 'U'
				ordered.Types = append(ordered.Types, typ)
			}
			sources := newSourceCache()
			generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
			units, err := captureRuntimeSourceUnits(ordered, &generation, map[string]string{})
			if err != nil {
				t.Fatal(err)
			}
			if units[paths["U"]] != "" || units[paths["A"]] == "" {
				t.Fatal("expected a captured project binding and omitted dependency binding")
			}
			if tc.collision && units[paths["A"]] == units[paths["B"]] {
				t.Fatal("fixture lost distinct project bindings")
			}
			if !tc.collision && units[paths["A"]] != units[paths["C"]] {
				t.Fatal("fixture split the same project binding")
			}
			sources.runtimeSourceUnits = units
			classes := compileProjectClasses(ordered, nil, sources)
			machine := vm.New(nil)
			unexpectedFold := false
			for _, class := range classes {
				if class.Name == "OwnedMixedInitializer" && !class.Dependency {
					folded := class.Fields["OFFSET"].InitializerLine != 0
					if folded == tc.collision {
						unexpectedFold = true
					}
				}
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			if !tc.collision {
				if unexpectedFold {
					t.Fatal("same-unit declarations lost literal folding")
				}
				return // Same-unit duplicates are only a detector control.
			}
			for _, check := range []struct {
				unit string
				want int
			}{{"A", 23}, {"B", 25}} {
				body, err := vm.CompileAnonymous(fmt.Sprintf("OwnedMixedInitializer row=new OwnedMixedInitializer(); System.assertEquals(%d,row.VALUE);", check.want))
				if err != nil {
					t.Fatal(err)
				}
				method := "OwnedInitializerHarness.check" + check.unit
				if err := machine.RegisterMethod(vm.Method{Name: method, ClassName: "OwnedInitializerHarness", SourceUnit: units[paths[check.unit]], IsStatic: true, Program: body}); err != nil {
					t.Fatal(err)
				}
				invoke, err := vm.CompileAnonymous(method + "();")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(invoke); err != nil {
					t.Fatalf("unit %s: %v", check.unit, err)
				}
			}
			if unexpectedFold {
				t.Fatal("distinct bound units retained folded instance literals")
			}
		})
	}
}
