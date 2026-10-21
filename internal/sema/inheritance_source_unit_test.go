package sema

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestInheritanceContractsKeepDuplicateSourceUnitsIndependent(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		name := "interface-new-implementation-old"
		if reverse {
			name = "interface-old-implementation-new"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			var paths []string
			for _, unit := range []string{"old", "new", "invalid"} {
				unitRoot := filepath.Join(root, unit)
				if err := os.MkdirAll(filepath.Join(unitRoot, "force-app", "classes"), 0755); err != nil {
					t.Fatal(err)
				}
				writeSemaFile(t, filepath.Join(unitRoot, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
				members, implementation := "", ""
				if unit != "old" {
					members = "Result fetch(String key);"
				}
				if unit == "new" {
					implementation = "public Result fetch(String key) { return null; }"
				}
				for name, source := range map[string]string{
					"Result":         "public class Result {}",
					"Port":           "public interface Port {" + members + "}",
					"Implementation": "public class Implementation implements Port {" + implementation + "}",
				} {
					path := filepath.Join(unitRoot, "force-app", "classes", name+".cls")
					writeSemaFile(t, path, source)
					paths = append(paths, path)
				}
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: paths}, schema.Schema{})
			// Equal Apex names have no cross-project precedence contract. Exercise
			// conflicting global winners so neither traversal order masks leakage.
			sort.Slice(index.Types, func(i, j int) bool {
				left, right := index.Types[i], index.Types[j]
				if left.Name != right.Name {
					return left.Name < right.Name
				}
				preferNew := left.Name == "Port"
				if reverse {
					preferNew = !preferNew
				}
				rank := func(file string) int {
					if strings.Contains(file, string(filepath.Separator)+"invalid"+string(filepath.Separator)) {
						return 2
					}
					isNew := strings.Contains(file, string(filepath.Separator)+"new"+string(filepath.Separator))
					if isNew == preferNew {
						return 0
					}
					return 1
				}
				return rank(left.File) < rank(right.File)
			})
			result := Analyze(index)
			invalidRejected := false
			for _, d := range result.Diagnostics {
				if d.Severity != diagnostic.Error {
					continue
				}
				if d.Code != "GLADESEMA017" || !strings.Contains(d.File, string(filepath.Separator)+"invalid"+string(filepath.Separator)) {
					t.Fatalf("independent valid unit rejected: %#v", d)
				}
				invalidRejected = true
			}
			if !invalidRejected {
				t.Fatalf("missing implementation in its own source unit was accepted: %#v", result.Diagnostics)
			}
		})
	}
}
