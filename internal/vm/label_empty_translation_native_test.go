package vm

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

func TestEmptyLabelTranslationNativeRoutes(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "label_translation_empty", "project")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := resource.LoadProjectWithDependencies(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
System.assertEquals('Master empty', System.Label.get('', 'EmptyLabel', 'fr'));
System.assertEquals('Master missing', System.Label.get('', 'MissingLabel', 'fr'));
System.assertEquals('Traduit', System.Label.get('', 'FilledLabel', 'fr'));
System.assertEquals(false, System.Label.translationExists('', 'EmptyLabel', 'fr'));
System.assertEquals(false, System.Label.translationExists('', 'MissingLabel', 'fr'));
System.assertEquals(true, System.Label.translationExists('', 'FilledLabel', 'fr'));
System.assertEquals('Master empty', Label.EmptyLabel);
System.assertEquals('Master missing', Label.MissingLabel);
System.assertEquals('Master filled', Label.FilledLabel);
try { System.Label.get('', 'AbsentLabel', 'fr'); System.assert(false, 'expected invalid label'); }
catch (System.InvalidParameterValueException error) {
 System.assertEquals('Invalid or non-existent label: AbsentLabel', error.getMessage());
}
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				org.Metadata = registry
				machine := New(nil)
				machine.SetOrg(&org)
				if route == "istest" {
					machine.EnableTestContext()
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
