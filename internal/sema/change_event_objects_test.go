package sema

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestChangeEventNativeProjectSemanticAPIRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/change_event_objects"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				source := filepath.Join(fixture, "project")
				if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					rel, err := filepath.Rel(source, path)
					if err != nil {
						return err
					}
					if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0700); err != nil {
						return err
					}
					writeSemaFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", api))
					return nil
				}); err != nil {
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
				result := Analyze(index)
				if index.HasErrors() || result.HasErrors() {
					t.Fatalf("native-deployed standard/custom triggers rejected: %v %v", index.Diagnostics, result.Diagnostics)
				}
				if route == "anonymous" {
					for _, method := range []string{"accountFields", "customFields"} {
						body, err := os.ReadFile(filepath.Join(fixture, "anonymous", method+".apex"))
						if err != nil {
							t.Fatal(err)
						}
						if result := AnalyzeAnonymous(index, string(body), api); result.HasErrors() {
							t.Fatalf("native anonymous %s rejected: %v", method, result.Diagnostics)
						}
					}
				}
			})
		}
	}
}
