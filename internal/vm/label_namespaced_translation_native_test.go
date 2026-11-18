package vm

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

// These cases mirror label_translation_namespaced/org.tsv. The em-space
// insertion was rejected, so Blank has no localization in the registry.
func TestNamespacedLabelTranslationNativeRoutes(t *testing.T) {
	const namespace = "samplepkg"
	const suffix = "2cea372ec811"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []struct{ name, namespace string }{
			{"qualified", namespace}, {"local", ""},
		} {
			for _, tc := range []struct {
				name, master, translated, want string
				present                        bool
			}{
				{"Filled", "MasterFilled", "EnglishFilled", "EnglishFilled", true},
				{"Missing", "MasterMissing", "", "MasterMissing", false},
				{"Blank", "MasterBlank", "", "MasterBlank", false},
				{"Nbsp", "MasterNbsp", "\u00a0", "\u00a0", true},
			} {
				t.Run(api+"/"+route.name+"/"+tc.name, func(t *testing.T) {
					name := "GladeProbe" + tc.name + suffix
					org := storage.NewOrgState()
					org.Namespace = namespace
					org.Metadata.Labels = []storage.LabelMetadata{{Name: name, Namespace: namespace, Language: "fr", Value: tc.master}}
					if tc.present {
						org.Metadata.Labels = append(org.Metadata.Labels, storage.LabelMetadata{Name: name, Namespace: namespace, Language: "en_US", Value: tc.translated})
					}
					program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
System.assertEquals('%s', System.Label.get('%s', '%s', 'en_US'));
System.assertEquals(%t, System.Label.translationExists('%s', '%s', 'en_US'));
`, tc.want, route.namespace, name, tc.present, route.namespace, name), CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					machine := New(nil)
					machine.SetOrg(&org)
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// The source representation contains only accepted localizations: Salesforce
// rejected the em-space insertion, leaving Blank without a localization.
func TestNamespacedLabelTranslationNativeProject(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "label_translation_namespaced", "project")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := resource.LoadProjectWithDependencies(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []struct{ name, namespace string }{{"qualified", "samplepkg"}, {"local", ""}} {
			for _, tc := range []struct {
				name, want string
				present    bool
			}{
				{"Filled", "EnglishFilled", true},
				{"Missing", "MasterMissing", false},
				{"Blank", "MasterBlank", false},
				{"Nbsp", "\u00a0", true},
			} {
				t.Run(api+"/"+route.name+"/"+tc.name, func(t *testing.T) {
					name := "GladeProbe" + tc.name + "2cea372ec811"
					program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
System.assertEquals('%s', System.Label.get('%s', '%s', 'en_US'));
System.assertEquals(%t, System.Label.translationExists('%s', '%s', 'en_US'));
`, tc.want, route.namespace, name, tc.present, route.namespace, name), CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := storage.NewOrgState()
					org.Namespace = p.Namespace
					org.Metadata = registry
					machine := New(nil)
					machine.SetOrg(&org)
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
