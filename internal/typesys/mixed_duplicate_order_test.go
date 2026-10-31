package typesys

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

// Conflicting local inputs exercise index order only. They are not evidence
// that Salesforce installs these conflicting packages or selects this winner.
func TestMixedDuplicateOriginsRetainInputOrder(t *testing.T) {
	for _, name := range []string{"Alder", "Birch", "Cypress"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			p := project.Project{Root: root, Namespace: "supplier"}
			var want []string
			for i := 0; i < 36; i++ {
				typeName := name
				if i%3 == 0 {
					typeName = fmt.Sprintf("Other%02d", 36-i)
				}
				dir := filepath.Join(root, fmt.Sprintf("dep%02d", i))
				if i%2 == 0 {
					file := fmt.Sprintf("artifact%02d.cls", i)
					artifact, err := packageartifact.BuildCaptured(packageartifact.BuildCapturedOptions{
						Namespace: "supplier", SourceAPIVersion: "67.0",
						ApexTypes: []packageartifact.ApexType{{Kind: apexast.DeclarationClass, Name: typeName, Namespace: "supplier", File: file, Modifiers: []string{"global"}}},
					})
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, fmt.Sprintf("artifact%02d.json", i))
					if err := packageartifact.WriteJSON(path, artifact); err != nil {
						t.Fatal(err)
					}
					p.ManagedPackageDependencies = append(p.ManagedPackageDependencies, project.ManagedPackageDependency{Namespace: "supplier", ArtifactPath: path, Status: "loaded"})
					if typeName == name {
						want = append(want, file)
					}
				} else {
					file := filepath.Join(dir, typeName+".cls")
					writeFile(t, file, "global class "+typeName+" {}")
					dep := &project.Project{Root: dir, ApexFiles: []string{file}}
					p.ManagedPackageDependencies = append(p.ManagedPackageDependencies, project.ManagedPackageDependency{Namespace: "supplier", SourceRoot: dir, Status: "loaded", Project: dep})
					if typeName == name {
						want = append(want, file)
					}
				}
			}
			for i := 0; i < 2; i++ {
				dir := filepath.Join(root, fmt.Sprintf("shim%02d", i))
				file := filepath.Join(dir, name+".cls")
				writeFile(t, file, "global class "+name+" {}")
				p.PackageShims = append(p.PackageShims, project.PackageShim{Namespace: "supplier", SourceRoot: dir, Status: "loaded", Project: &project.Project{Root: dir, ApexFiles: []string{file}}})
				want = append(want, file)
			}
			local := filepath.Join(root, name+".cls")
			writeFile(t, local, "public class "+name+" {}")
			p.ApexFiles = []string{local}
			want = append(want, local)
			idx := Build(p, schema.Schema{})
			var got []string
			for _, typ := range idx.Types {
				if typ.Namespace == "supplier" && typ.Name == name {
					got = append(got, typ.File)
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("candidate order = %v, want %v; diagnostics = %v", got, want, idx.Diagnostics)
			}
		})
	}
}
