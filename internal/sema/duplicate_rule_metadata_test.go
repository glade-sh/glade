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

func TestDuplicateRuleMetadataNativeProjectAnonymousAndIsTestAPIRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/setup_metadata"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			sourceRoot := filepath.Join(fixture, "project")
			err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				relative, err := filepath.Rel(sourceRoot, path)
				if err != nil {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				target := filepath.Join(root, relative)
				if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
					return err
				}
				return os.WriteFile(target, []byte(strings.ReplaceAll(string(body), "62.0", api)), 0600)
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
			t.Run("project_istest", func(t *testing.T) {
				result := Analyze(index)
				if index.HasErrors() || result.HasErrors() {
					t.Fatalf("native project compiled; diagnostics %v %v", index.Diagnostics, result.Diagnostics)
				}
			})
			for _, method := range []string{"duplicateRule", "personDuplicateRule"} {
				t.Run("anonymous/"+method, func(t *testing.T) {
					body, err := os.ReadFile(filepath.Join(fixture, "anonymous", method+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					if result := AnalyzeAnonymous(index, string(body), api); result.HasErrors() {
						t.Fatalf("native anonymous source compiled; diagnostics %v", result.Diagnostics)
					}
				})
			}
		})
	}
}
