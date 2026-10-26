package watch

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestSupplementalMetadataScopeRoutes(t *testing.T) {
	for _, route := range []string{"primary", "direct_dependency", "package_shim"} {
		t.Run(route, func(t *testing.T) {
			workspace := t.TempDir()
			root := filepath.Join(workspace, "project")
			extra := filepath.Join(workspace, "supplemental")
			class := filepath.Join(extra, "classes/Service.cls")
			writeWatchFile(t, filepath.Join(root, "sfdx-project.json"), `{}`)
			writeWatchFile(t, class, "public class Service {}")
			owner := project.Project{Root: root, ExtraMetadataDirs: []string{extra}}
			p := owner
			if route != "primary" {
				owner.Root = filepath.Join(workspace, "dependency")
				writeWatchFile(t, filepath.Join(owner.Root, "sfdx-project.json"), `{}`)
				p = project.Project{Root: root}
				if route == "direct_dependency" {
					p.ManagedPackageDependencies = []project.ManagedPackageDependency{{SourceRoot: owner.Root, Status: "loaded", Project: &owner}}
				} else {
					p.PackageShims = []project.PackageShim{{SourceRoot: owner.Root, Status: "loaded", Project: &owner}}
				}
			}
			for _, scope := range []Scope{ProjectScope(root, p), ProjectScopeWithPrevious(root, p, Scope{})} {
				snapshot, err := CaptureScope(scope)
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := snapshot.Files[class]; !ok {
					t.Errorf("%s scope omitted loaded supplemental Apex source", route)
				}
				writeWatchFile(t, class, "public class Service { public static Integer value() { return 2; } }")
				next, err := CaptureScope(scope)
				if err != nil {
					t.Fatal(err)
				}
				changes := DiffSnapshots(snapshot, next)
				if len(changes) != 1 || changes[0].Path != class {
					t.Errorf("supplemental modification = %#v", changes)
				}
				writeWatchFile(t, class, "public class Service {}")
			}
		})
	}
}

func TestNetworkMetadataWatchClassificationRoutes(t *testing.T) {
	for _, route := range []string{"package", "supplemental", "case_folded_extension"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, route, "networks/Synthetic Portal.network-meta.xml")
			if route == "case_folded_extension" {
				path = filepath.Join(root, route, "networks/Synthetic Portal.NETWORK-META.XML")
			}
			writeWatchFile(t, path, `<Network><status>UnderConstruction</status><urlPathPrefix>synthetic</urlPathPrefix></Network>`)
			got := ClassifyPath(path)
			if !got.Watchable || string(got.Kind) != "network_metadata" {
				t.Errorf("Network classification = %#v", got)
			}
			snapshot, err := CaptureScope(Scope{Roots: []string{root}})
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := snapshot.Files[path]; !ok {
				t.Error("snapshot omitted Network metadata")
			}
		})
	}
}

func TestSupplementalMetadataScopeExcludesTransitiveAndArtifactRoutes(t *testing.T) {
	for _, route := range []string{"transitive", "artifact"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			directRoot := filepath.Join(root, "direct")
			excludedRoot := filepath.Join(root, "excluded")
			extra := filepath.Join(root, "excluded-extra")
			class := filepath.Join(extra, "classes/Excluded.cls")
			writeWatchFile(t, class, "public class Excluded {}")
			nested := project.Project{Root: excludedRoot, ExtraMetadataDirs: []string{extra}}
			dependency := project.ManagedPackageDependency{SourceRoot: excludedRoot, Status: "loaded", Project: &nested}
			p := project.Project{Root: root}
			if route == "artifact" {
				dependency.ArtifactPath = filepath.Join(root, "package.json")
				p.ManagedPackageDependencies = []project.ManagedPackageDependency{dependency}
			} else {
				direct := project.Project{Root: directRoot, ManagedPackageDependencies: []project.ManagedPackageDependency{dependency}}
				p.ManagedPackageDependencies = []project.ManagedPackageDependency{{SourceRoot: directRoot, Status: "loaded", Project: &direct}}
			}
			snapshot, err := CaptureScope(ProjectScope(root, p))
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := snapshot.Files[class]; ok {
				t.Fatal("scope included supplemental source from an excluded dependency")
			}
		})
	}
}
