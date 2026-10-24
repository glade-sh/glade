package sema

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Compile complete projects rather than exercising only the reference resolver.
// Namespace coverage is local; the owned native org had namespace disabled.
func TestCommonProjectShapesCompileClean(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/common_project_shapes"
	routes := map[string]string{
		"topLevel":       "new ShapeUse().topLevelRoute()",
		"nested":         "new ShapeBase().nestedRoute()",
		"inheritedLocal": "new ShapeDerived().inheritedLocalRoute()",
		"inheritedField": "new ShapeDerived().inheritedFieldRoute()",
	}
	named := commonProjectShapeRows(t, filepath.Join(fixture, "org.tsv"))
	anonymous := commonProjectShapeRows(t, filepath.Join(fixture, "anonymous.tsv"))
	negative := commonProjectShapeRows(t, filepath.Join(fixture, "negative.tsv"))
	for _, api := range []string{"62.0", "67.0"} {
		for _, view := range []struct{ name, namespace string }{{"plain", ""}, {"namespaced", "garden"}} {
			t.Run(api+"/"+view.name, func(t *testing.T) {
				root := t.TempDir()
				sourceRoot := filepath.Join(fixture, "project")
				err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					rel, err := filepath.Rel(sourceRoot, path)
					if err != nil {
						return err
					}
					target := filepath.Join(root, rel)
					if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
						return err
					}
					return os.WriteFile(target, []byte(strings.ReplaceAll(string(body), "62.0", api)), 0o644)
				})
				if err != nil {
					t.Fatal(err)
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				p.Namespace = view.namespace
				s, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index, artifacts := typesys.BuildWithArtifacts(p, s)
				t.Run("project-and-isTest", func(t *testing.T) {
					result := AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, BuildArtifacts: &artifacts})
					if len(index.Diagnostics) != 0 || len(result.Diagnostics) != 0 {
						t.Fatalf("native project compiled cleanly; index=%+v sema=%+v", index.Diagnostics, result.Diagnostics)
					}
				})
				for route, expression := range routes {
					t.Run("anonymous/"+route, func(t *testing.T) {
						key := api + "/" + route
						if len(named[key]) != 3 || len(anonymous[key]) != 3 {
							t.Fatalf("missing native route twins: %s", key)
						}
						var namedValue, anonymousValue map[string]string
						if json.Unmarshal([]byte(named[key][2]), &namedValue) != nil || json.Unmarshal([]byte(anonymous[key][2]), &anonymousValue) != nil || namedValue["value"] == "" || namedValue["value"] != anonymousValue["value"] {
							t.Fatalf("native route twins disagree: named=%v anonymous=%v", named[key], anonymous[key])
						}
						result := AnalyzeAnonymous(index, "String observed = "+expression+";", api)
						if len(result.Diagnostics) != 0 {
							t.Fatalf("native anonymous route compiled cleanly; diagnostics=%+v", result.Diagnostics)
						}
					})
				}
				for _, route := range []string{"project", "anonymous"} {
					t.Run("unresolved/"+route, func(t *testing.T) {
						row := negative[api+"/"+route]
						if len(row) != 4 || row[2] != "false" || !strings.Contains(row[3], "AbsentGardenType") {
							t.Fatalf("missing native unresolved-type rejection: %v", row)
						}
						var result Result
						if route == "anonymous" {
							result = AnalyzeAnonymous(index, "AbsentGardenType missing;", api)
						} else {
							body, err := os.ReadFile(filepath.Join(fixture, "MissingShape.cls"))
							if err != nil {
								t.Fatal(err)
							}
							path := filepath.Join(root, "MissingShape.cls")
							writeSemaFile(t, path, string(body))
							badProject := p
							badProject.ApexFiles = append(append([]string(nil), p.ApexFiles...), path)
							result = Analyze(typesys.Build(badProject, s))
						}
						for _, d := range result.Diagnostics {
							if d.Severity == diagnostic.Error && strings.Contains(d.Message+" "+d.NativeMessage, "AbsentGardenType") {
								return
							}
						}
						t.Fatalf("unresolved type lost its error; native=%s diagnostics=%+v", row[3], result.Diagnostics)
					})
				}
			})
		}
	}
}

func commonProjectShapeRows(t *testing.T, path string) map[string][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil || len(rows) < 2 || rows[0][0] != "source_api" {
		t.Fatalf("invalid native rows in %s: %v", path, err)
	}
	result := map[string][]string{}
	for _, row := range rows[1:] {
		if len(row) < 3 || (row[0] != "62.0" && row[0] != "67.0") || row[1] == "" {
			t.Fatalf("invalid native row in %s: %v", path, row)
		}
		key := row[0] + "/" + row[1]
		if result[key] != nil {
			t.Fatalf("duplicate native row %s in %s", key, path)
		}
		result[key] = row
	}
	return result
}
