package apextest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Sweep every captured feature profile, including Communities, schedules and
// OrderManagement. This checks propagation, not uncaptured profile attribution:
// the effective runtime must use exactly the same gates as a main-root profile.
func TestDependencyEveryFeatureProfileRuntimeSchema(t *testing.T) {
	profiles, err := filepath.Glob("testdata/conformance/standard_describe_profiles/profiles/*.json")
	if err != nil || len(profiles) != 13 {
		t.Fatalf("profiles=%v: %v", profiles, err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, profile := range profiles {
			t.Run(api+"/"+filepath.Base(profile), func(t *testing.T) {
				raw, err := os.ReadFile(profile)
				if err != nil {
					t.Fatal(err)
				}
				root := t.TempDir()
				consumer, dependency := filepath.Join(root, "consumer"), filepath.Join(root, "dependency")
				writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"ProfileBase"}]}]}`)
				writeFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  dependencyPaths: [\"../dependency\"]\n")
				writeFile(t, filepath.Join(consumer, "force-app/main/default/classes/Consumer.cls"), "public class Consumer {}")
				writeFile(t, filepath.Join(dependency, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"package":"ProfileBase"}]}`)
				writeFile(t, filepath.Join(dependency, "force-app/main/default/classes/Dependency.cls"), "public class Dependency {}")
				writeFile(t, filepath.Join(dependency, "config/project-scratch-def.json"), string(raw))
				features := project.OrgShapeFeatures(dependency)
				index := loadTestIndex(t, consumer)
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				for _, feature := range features {
					if !slices.Contains(index.OrgShapeFeatures, feature) {
						t.Errorf("dependency feature %s missing: %v", feature, index.OrgShapeFeatures)
					}
				}
				org := orgFromIndex(index)
				// Include the off-profile's objects so lost gates cannot hide by
				// removing an object from StandardFeatureObjectNames.
				names := append(storage.StandardFeatureObjectNames(features), "Account", "Contact", "User")
				for _, name := range names {
					storage.EnsureStandardObject(&org, name)
					expected := storage.ObjectDefinition{APIName: name}
					storage.EnsureStandardObjectFieldsForFeatures(&expected, features)
					actual := org.Objects[name].Definition
					for field := range expected.Fields {
						if _, exists := storage.ResolveFieldName(actual, "", field); !exists {
							t.Errorf("dependency profile lost %s.%s", name, field)
						}
					}
				}
			})
		}
	}
}
