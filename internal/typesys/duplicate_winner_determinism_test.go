package typesys

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

// The input order is part of the local index contract; this checks the first
// candidate and all equal-key candidates across repeated complete builds.
func TestDuplicateClassWinnerDeterminism(t *testing.T) {
	for _, name := range []string{"Amber", "Jade", "Quartz"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			var files, want []string
			for i := 0; i < 48; i++ {
				typeName := name
				if i%3 == 0 {
					typeName = fmt.Sprintf("Other%02d", 48-i)
				}
				path := filepath.Join(root, fmt.Sprintf("source%02d.cls", i))
				writeFile(t, path, "public class "+typeName+" {}")
				files = append(files, path)
				if typeName == name {
					want = append(want, path)
				}
			}
			p := project.Project{Root: root, ApexFiles: files}
			for build := 0; build < 3; build++ {
				var idx Index
				if build == 1 {
					idx, _ = BuildWithArtifacts(p, schema.Schema{})
				} else {
					idx = Build(p, schema.Schema{})
				}
				var got []string
				for _, typ := range idx.Types {
					if typ.Namespace == "" && typ.Name == name {
						got = append(got, typ.File)
					}
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("build %d: candidate order = %v, want %v", build, got, want)
				}
			}
		})
	}
}
