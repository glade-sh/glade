package project

import (
	"path/filepath"
	"slices"
	"testing"
)

// P0 and P11 inventory captures differ at API68, so source edition must travel
// with the declared org shape. Dependency scratch editions do not describe it.
func TestStandardFieldShapeRetainsRootEdition(t *testing.T) {
	for _, row := range []struct{ name, root, dependency, want string }{
		{"developer", `{"edition":"Developer"}`, "", "__glade_edition_developer"},
		{"enterprise", `{"edition":"Enterprise"}`, "", "__glade_edition_enterprise"},
		{"dependency only", "", `{"edition":"Enterprise"}`, ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			dir := t.TempDir()
			root, dep := filepath.Join(dir, "root"), filepath.Join(dir, "dep")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{}`)
			if row.root != "" {
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), row.root)
			}
			if row.dependency != "" {
				writeFile(t, filepath.Join(dep, "config/project-scratch-def.json"), row.dependency)
			}
			features := StandardFieldPolicyFeatures(Project{Root: root, ManagedPackageDependencies: []ManagedPackageDependency{{Status: "loaded", Project: &Project{Root: dep}}}})
			if row.want != "" && !slices.Contains(features, row.want) {
				t.Fatalf("edition marker %s missing from %v", row.want, features)
			}
			if row.want == "" && (slices.Contains(features, "__glade_edition_developer") || slices.Contains(features, "__glade_edition_enterprise")) {
				t.Fatalf("dependency edition leaked into %v", features)
			}
		})
	}
}
