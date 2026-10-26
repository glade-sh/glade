package startupcache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestNetworkSupplementalDirectoryManifestRoutes(t *testing.T) {
	for _, route := range []string{"internal", "external"} {
		t.Run(route, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "project")
			dir := filepath.Join(root, "supplemental")
			if route == "external" {
				dir = filepath.Join(filepath.Dir(root), "shared")
			}
			if err := os.MkdirAll(root, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "Synthetic Portal.network-meta.xml")
			if err := os.WriteFile(path, []byte(`<Network><status>UnderConstruction</status><urlPathPrefix>synthetic</urlPathPrefix></Network>`), 0600); err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(map[string]any{"root": root, "extraMetadataDirs": []string{dir}, "networkFiles": []string{path}})
			if err != nil {
				t.Fatal(err)
			}
			var p project.Project
			if err := json.Unmarshal(raw, &p); err != nil {
				t.Fatal(err)
			}
			manifest := BuildManifest(root, p, typesys.Index{})
			if !manifest.Complete {
				t.Fatal("supplemental directories must not be hashed as files")
			}
			relDir, err := filepath.Rel(root, dir)
			if err != nil {
				t.Fatal(err)
			}
			foundRoot := false
			for _, entry := range manifest.PackageRoots {
				if entry.Path == filepath.ToSlash(relDir) {
					foundRoot = true
				}
			}
			if !foundRoot {
				t.Fatalf("supplemental directory absent from manifest roots: %+v", manifest.PackageRoots)
			}
			relFile, err := filepath.Rel(root, path)
			if err != nil {
				t.Fatal(err)
			}
			foundFile := false
			for _, entry := range manifest.Files {
				if entry.Path == filepath.ToSlash(relFile) {
					foundFile = true
				}
			}
			if !foundFile {
				t.Fatalf("Network file absent from manifest inputs: %+v", manifest.Files)
			}
			if err := os.WriteFile(path, []byte(`<Network><status>UnderConstruction</status><urlPathPrefix>changed</urlPathPrefix></Network>`), 0600); err != nil {
				t.Fatal(err)
			}
			changed := BuildManifest(root, p, typesys.Index{})
			before, _ := json.Marshal(manifest)
			after, _ := json.Marshal(changed)
			if strings.EqualFold(string(before), string(after)) {
				t.Fatal("changed Network metadata retained the old manifest")
			}
		})
	}
}
