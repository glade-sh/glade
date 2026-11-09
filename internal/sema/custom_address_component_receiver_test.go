package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// custom_address_fields/org.tsv compile rows (API 62.0 and 67.0, anonymous):
// a constructed record exposes only the generated components of its custom
// Address field. C06/C07 name components that do not exist; C05/C12 read
// real ones.
func TestAnonymousConstructedRecordAddressComponents(t *testing.T) {
	const fixture = "../../testdata/conformance/custom_address_fields"
	want := map[string]string{
		"C05": "",
		"C06": "Variable does not exist: Home__State__s",
		"C07": "Variable does not exist: Home__Country__s",
		"C12": "",
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(fixture, "project")
			err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(source, path)
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				target := filepath.Join(root, rel)
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return err
				}
				writeSemaFile(t, target, strings.ReplaceAll(string(raw), "67.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			for id, problem := range want {
				raw, err := os.ReadFile(filepath.Join(fixture, "cases", id+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				result := AnalyzeAnonymous(index, string(raw), api)
				if problem == "" {
					if len(result.Diagnostics) != 0 {
						t.Errorf("%s: native compiled; Glade %+v", id, result.Diagnostics)
					}
					continue
				}
				matched := false
				for _, d := range result.Diagnostics {
					matched = matched || d.NativeMessage == problem
				}
				if !matched {
					t.Errorf("%s: native %q; Glade %+v", id, problem, result.Diagnostics)
				}
			}
		})
	}
}
