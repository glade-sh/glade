package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/sema"
)

// TestCorpusShape keeps unrelated Apex shapes in one synthetic SFDX project so
// that project-wide resolution and the real test runner see their interactions.
// Value assertions cite existing native captures in ShapeProjectTest.cls. Other
// shapes pin compilation without output values; the named entry points also
// exercise runner smoke routes. Optional package types are looked up dynamically.
// Native project compile: testdata/corpus_shape/native/org.tsv:6,11; ORG-RUN.txt.
func TestCorpusShape(t *testing.T) {
	started := time.Now()
	for _, api := range []string{"62.0", "67.0"} {
		t.Run("API"+api, func(t *testing.T) {
			root := copyCorpusShapeProject(t, api)
			index := loadTestIndex(t, root)
			if len(index.Diagnostics) != 0 {
				t.Fatalf("project index diagnostics: %v", index.Diagnostics)
			}
			analysis := sema.Analyze(index)
			if len(analysis.Diagnostics) != 0 {
				t.Fatalf("project check diagnostics: %v", analysis.Diagnostics)
			}
			t.Run("anonymousPostReturn", func(t *testing.T) {
				source, err := os.ReadFile(filepath.Join(root, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				result := sema.AnalyzeAnonymous(index, string(source), api)
				if len(result.Diagnostics) != 0 {
					t.Fatalf("anonymous check diagnostics: %v", result.Diagnostics)
				}
			})
			t.Run("projectTestRunner", func(t *testing.T) {
				run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
				for _, suite := range run.Suites {
					for _, item := range suite.Cases {
						if item.Problem != nil {
							t.Errorf("%s.%s: %s", suite.Name, item.MethodName, item.Problem.Message)
						}
					}
				}
				if summary := run.Summary(); summary.Total != 14 || summary.Passed != 14 || summary.CompileErrors != 0 {
					t.Fatalf("project test runner: %+v", summary)
				}
			})
			t.Run("fieldEnumCollision", func(t *testing.T) {
				checkCorpusShapeFieldEnumCollision(t, api)
			})
		})
	}
	t.Run("personAccountFields", func(t *testing.T) {
		t.Skip("Person Account __pc field synthesis is not implemented yet")
		for _, api := range []string{"62.0", "67.0"} {
			root := copyCorpusShapeProject(t, api)
			source, err := os.ReadFile(filepath.Join(root, "person-account/ShapePerson.cls.pending"))
			if err != nil {
				t.Fatal(err)
			}
			classes := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(classes, "ShapePerson.cls"), string(source))
			writeFile(t, filepath.Join(classes, "ShapePerson.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [PersonAccounts]\n")
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Contact/fields/Nickname__c.field-meta.xml"), `<CustomField><fullName>Nickname__c</fullName><label>Nickname</label><type>Text</type><length>80</length></CustomField>`)
			index := loadTestIndex(t, root)
			if len(index.Diagnostics) != 0 {
				t.Fatal(index.Diagnostics)
			}
			if result := sema.Analyze(index); len(result.Diagnostics) != 0 {
				t.Fatalf("Person Account project check diagnostics: %v", result.Diagnostics)
			}
		}
	})
	if elapsed := time.Since(started); elapsed >= time.Minute {
		t.Errorf("synthetic project took %s; must stay under one minute", elapsed)
	}
}

func copyCorpusShapeProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/corpus_shape")); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, "-meta.xml") && filepath.Base(path) != "sfdx-project.json" {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, []byte(strings.ReplaceAll(string(source), "67.0", api)), 0o600)
	}); err != nil {
		t.Fatal(err)
	}
	return root
}
