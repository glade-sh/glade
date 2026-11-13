package project

import (
	"path/filepath"
	"slices"
	"testing"
)

// A dependency's own development shape is positive feature evidence, not an
// authoritative negative declaration for the consuming project's target org.
func TestStandardFieldShapeDeclarationOwnership(t *testing.T) {
	for _, row := range []struct {
		name, rootConfig, dependencyConfig string
		unknown, currency                  bool
	}{
		{name: "source-only", unknown: true},
		{name: "dependency-bare", dependencyConfig: `{"edition":"Developer"}`, unknown: true},
		{name: "dependency-positive", dependencyConfig: `{"features":["MultiCurrency"]}`, unknown: true, currency: true},
		{name: "root-declared-dependency-positive", rootConfig: "org:\n  features: []\n", dependencyConfig: `{"features":["MultiCurrency"]}`, currency: true},
		{name: "unrelated-root-config", rootConfig: "test:\n  permissionSets: []\n", dependencyConfig: `{"edition":"Developer"}`, unknown: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			workspace := t.TempDir()
			root := filepath.Join(workspace, "consumer")
			dependency := filepath.Join(workspace, "dependency")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{}`)
			if row.rootConfig != "" {
				writeFile(t, filepath.Join(root, "glade.yml"), row.rootConfig)
			}
			if row.dependencyConfig != "" {
				writeFile(t, filepath.Join(dependency, "config/project-scratch-def.json"), row.dependencyConfig)
			}
			p := Project{Root: root, ManagedPackageDependencies: []ManagedPackageDependency{{Status: "loaded", Project: &Project{Root: dependency}}}}
			got := StandardFieldPolicyFeatures(p)
			if slices.Contains(got, UnspecifiedStandardFieldShape) != row.unknown || slices.Contains(got, "MultiCurrency") != row.currency {
				t.Fatalf("features=%v, want unknown=%t currency=%t", got, row.unknown, row.currency)
			}
		})
	}
}

func TestPlainDeveloperDescribeShape(t *testing.T) {
	for _, row := range []struct {
		name, definition string
		want             bool
	}{
		{"captured", `{"edition":"Developer","features":[],"settings":{}}`, true},
		{"omitted", `{"edition":"Developer"}`, false},
		{"null features", `{"edition":"Developer","features":null,"settings":{}}`, false},
		{"null settings", `{"edition":"Developer","features":[],"settings":null}`, false},
		{"password feature", `{"edition":"Developer","features":["EnableSetPasswordInApi"],"settings":{}}`, false},
		{"lightning setting", `{"edition":"Developer","features":[],"settings":{"lightningExperienceSettings":{"enableS1DesktopEnabled":true}}}`, false},
		{"country", `{"edition":"Developer","country":"FR","features":[],"settings":{}}`, false},
		{"language", `{"edition":"Developer","language":"fr","features":[],"settings":{}}`, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{}`)
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), row.definition)
			got := slices.Contains(StandardFieldPolicyFeatures(Project{Root: root}), "__glade_plain_developer_describe")
			if got != row.want {
				t.Fatalf("describe shape=%t want %t", got, row.want)
			}
		})
	}
}
