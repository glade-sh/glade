package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPublishingDateArgumentNativeSemaAnonymousAndNamedRoutes(t *testing.T) {
	runPlatformMemberNativeSema(t, "publishing_date_arguments")
}

func TestFinalizerFieldNativeSemaAnonymousAndNamedRoutes(t *testing.T) {
	runPlatformMemberNativeSema(t, "finalizer_field_dispatch")
}

func runPlatformMemberNativeSema(t *testing.T, family string) {
	t.Helper()
	fixture := filepath.Join("..", "vm", "testdata", "conformance", family)
	read := func(path string) []byte {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			p := project.Project{Root: root, SourceAPIVersion: api}
			files, err := filepath.Glob(filepath.Join(fixture, "sources", "*.cls"))
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range files {
				path := filepath.Join(root, filepath.Base(file))
				writeSemaFile(t, path, string(read(filepath.Join("sources", filepath.Base(file)))))
				p.ApexFiles = append(p.ApexFiles, path)
			}
			if shape, err := os.ReadFile(filepath.Join(fixture, "project-scratch-def.json")); err == nil {
				if err := os.MkdirAll(filepath.Join(root, "config"), 0755); err != nil {
					t.Fatal(err)
				}
				writeSemaFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(shape))
			}
			index := typesys.Build(p, schema.Schema{})
			t.Run("accepted_named", func(t *testing.T) {
				if result := Analyze(index); result.HasErrors() {
					t.Fatalf("native named accepted; diagnostics=%v", result.Diagnostics)
				}
			})
			t.Run("accepted_anonymous", func(t *testing.T) {
				if result := AnalyzeAnonymous(index, string(read("sources/anonymous.apex")), api); result.HasErrors() {
					t.Fatalf("native anonymous accepted; diagnostics=%v", result.Diagnostics)
				}
			})
			native := filepath.Join(fixture, "native", "api"+api[:2])
			f, err := os.Open(filepath.Join(native, "named-rejections.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			reader := csv.NewReader(f)
			reader.Comma = '\t'
			rows, err := reader.ReadAll()
			f.Close()
			if err != nil || len(rows) != 2 {
				t.Fatalf("invalid named native rejection: %v", err)
			}
			name := rows[1][2]
			t.Run("rejected_named", func(t *testing.T) {
				bad := p
				path := filepath.Join(root, name)
				writeSemaFile(t, path, string(read(filepath.Join("rejections", name))))
				bad.ApexFiles = append(append([]string(nil), p.ApexFiles...), path)
				if result := Analyze(typesys.Build(bad, schema.Schema{})); !result.HasErrors() {
					t.Fatal("native named rejected; Glade accepted")
				}
			})
			reject := "interface_reject.apex"
			if family == "publishing_date_arguments" {
				reject = "date_reject.apex"
			}
			t.Run("rejected_anonymous", func(t *testing.T) {
				if result := AnalyzeAnonymous(index, string(read(filepath.Join("sources", reject))), api); !result.HasErrors() {
					t.Fatal("native anonymous rejected; Glade accepted")
				}
			})
			if family == "publishing_date_arguments" {
				t.Run("knowledge_disabled_anonymous", func(t *testing.T) {
					if err := os.Remove(filepath.Join(root, "config/project-scratch-def.json")); err != nil {
						t.Fatal(err)
					}
					disabled := typesys.Build(p, schema.Schema{})
					if result := AnalyzeAnonymous(disabled, string(read("sources/knowledge_disabled.apex")), api); !result.HasErrors() {
						t.Fatal("native Knowledge-disabled call rejected; Glade accepted")
					}
				})
			}
		})
	}
}
