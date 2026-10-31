package typesys

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

// Identically named declarations that survive project source selection retain
// their input order in the index, even when other names need sorting around them.
// This is an index ordering rule, not a Salesforce deployment winner assertion.
func TestBuildDuplicateTypeOrderStable(t *testing.T) {
	for _, duplicateName := range []string{"Mango", "Pear", "Quince"} {
		t.Run(duplicateName, func(t *testing.T) {
			root := t.TempDir()
			var paths, want []string
			for i := 0; i < 80; i++ {
				name := duplicateName
				if i%3 == 0 {
					name = fmt.Sprintf("Other%03d", 80-i)
				}
				path := filepath.Join(root, fmt.Sprintf("source%03d.cls", i))
				writeFile(t, path, "public class "+name+" {}")
				paths = append(paths, path)
				if name == duplicateName {
					want = append(want, path)
				}
			}
			idx := Build(project.Project{Root: root, ApexFiles: paths}, schema.Schema{})
			var got []string
			for _, typ := range idx.Types {
				if typ.Name == duplicateName {
					got = append(got, typ.File)
				}
			}
			if len(got) != len(want) {
				t.Fatalf("got %d duplicate symbols, want %d", len(got), len(want))
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("duplicate %d = %s, want %s", i, got[i], want[i])
				}
			}
		})
	}
}
