package typesys

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestNativeDuplicateClassWinnerIndexRoutes(t *testing.T) {
	fixtureRoot := filepath.Join("..", "project", "testdata", "source_collisions")
	data, err := os.ReadFile(filepath.Join(fixtureRoot, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		for _, route := range []string{"build", "build_with_artifacts", "update_winner", "update_shadowed", "delete_winner", "delete_shadowed", "add_later", "dependency_build", "shim_build"} {
			t.Run(row[0]+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				if err := os.CopyFS(root, os.DirFS(filepath.Join(fixtureRoot, row[2]))); err != nil {
					t.Fatal(err)
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				winner := filepath.Join(root, "packages", row[6], "main", "default", "classes", row[3]+".cls")
				shadowed := ""
				for _, path := range p.ApexFiles {
					if path != winner {
						shadowed = path
					}
				}
				var idx Index
				switch route {
				case "build":
					idx = Build(p, schema.Schema{})
				case "build_with_artifacts":
					idx, _ = BuildWithArtifacts(p, schema.Schema{})
				case "dependency_build":
					idx = Build(project.Project{Root: t.TempDir(), ManagedPackageDependencies: []project.ManagedPackageDependency{{Namespace: "supplier", SourceRoot: root, Status: "loaded", Project: &p}}}, schema.Schema{})
				case "shim_build":
					idx = Build(project.Project{Root: t.TempDir(), PackageShims: []project.PackageShim{{Namespace: "supplier", SourceRoot: root, Status: "loaded", Project: &p}}}, schema.Schema{})
				case "delete_winner", "delete_shadowed":
					idx = Build(p, schema.Schema{})
					deleted := shadowed
					if route == "delete_winner" {
						deleted = winner
						winner = shadowed
					}
					if err := os.Remove(deleted); err != nil {
						t.Fatal(err)
					}
					idx, err = UpdateApexFilesChecked(idx, nil, []string{deleted})
					if err != nil {
						t.Fatal(err)
					}
				case "add_later":
					body, err := os.ReadFile(winner)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.Remove(winner); err != nil {
						t.Fatal(err)
					}
					initial, err := project.Load(root)
					if err != nil {
						t.Fatal(err)
					}
					idx = Build(initial, schema.Schema{})
					if err := os.WriteFile(winner, body, 0600); err != nil {
						t.Fatal(err)
					}
					idx, err = UpdateApexFilesChecked(idx, []string{winner}, nil)
					if err != nil {
						t.Fatal(err)
					}
				default:
					idx = Build(p, schema.Schema{})
					changed := winner
					if route == "update_shadowed" {
						for _, path := range p.ApexFiles {
							if path != winner {
								changed = path
							}
						}
					}
					body, err := os.ReadFile(changed)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(changed, append(body, '\n'), 0600); err != nil {
						t.Fatal(err)
					}
					idx, err = UpdateApexFilesChecked(idx, []string{changed}, nil)
					if err != nil {
						t.Fatal(err)
					}
				}
				if idx.HasErrors() {
					t.Fatalf("index errors: %v", idx.Diagnostics)
				}
				if len(idx.Types) != 1 || idx.Types[0].File != winner {
					var paths []string
					for _, typ := range idx.Types {
						paths = append(paths, typ.File)
					}
					t.Fatalf("index sources = %v, want only native winner %s", paths, winner)
				}
			})
		}
	}
}
