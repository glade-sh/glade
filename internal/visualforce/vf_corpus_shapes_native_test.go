package visualforce_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

// Synthetic shapes that real projects use and that deploy natively: an
// ApexPages.Action attribute, buttons in a form supplied by a component body or
// a composition template, table rows around repeats, a map literal with bare
// quotes, and apex:variable scope. The rows were captured with gen.py/run.sh.
const shapesFamily = "testdata/vf_corpus_shapes"

var shapesSupport = map[string]bool{"ShapeTemplate": true, "ShapePlainTemplate": true}

func shapesProject(t *testing.T, api string, keep map[string]bool) string {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(shapesFamily, "project")
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(filepath.ToSlash(rel), "force-app/main/default/pages/") {
			base := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(rel), ".page-meta.xml"), ".page")
			if !keep[base] && !shapesSupport[base] {
				return nil
			}
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, []byte(strings.ReplaceAll(string(body), "67.0", api)), 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestVisualforceCorpusShapesNativeRows(t *testing.T) {
	for _, api := range []string{"59.0", "62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			rows := posRows(t, shapesFamily, api)
			if len(rows) != 19 {
				t.Fatalf("native rows=%d, want 19", len(rows))
			}
			ok := map[string]bool{}
			for _, row := range rows {
				if row.outcome == "render" || row.outcome == "compiles" {
					ok[row.name] = true
				}
			}
			machine, _, index := ispickvalMachineWith(t, shapesProject(t, api, ok), func(org *storage.OrgState) {
				account, found := storage.StandardObjectDefinition("Account")
				if !found {
					t.Fatal("no Account definition")
				}
				org.Objects["Account"] = storage.ObjectState{Definition: account, Records: map[storage.ID]storage.Record{}}
			})
			for _, row := range rows {
				if row.outcome == "error" {
					continue
				}
				t.Run(row.outcome+"/"+row.name, func(t *testing.T) {
					for _, route := range []string{"RenderPage", "AnonymousGetContent"} {
						var rendered string
						if route == "RenderPage" {
							result, err := visualforce.RenderPage(visualforce.PageRenderRequest{VFIndex: index, Machine: machine, PageName: row.name})
							if err != nil {
								t.Fatalf("%s: %v", route, err)
							}
							if result.Error != nil {
								t.Fatalf("%s: %v", route, result.Error.Message)
							}
							rendered = result.HTML
						} else {
							program, err := vm.CompileAnonymous(fmt.Sprintf("System.debug(new PageReference('/apex/%s').getContent().toString());", row.name))
							if err != nil {
								t.Fatal(err)
							}
							result, err := machine.Execute(program)
							if err != nil {
								t.Fatalf("%s: %v", route, err)
							}
							if len(result.Debug) != 1 {
								t.Fatalf("%s: debug rows=%d", route, len(result.Debug))
							}
							rendered = result.Debug[0]
						}
						if row.outcome == "compiles" {
							if !strings.Contains(rendered, `data-case="`+row.name+`"`) {
								t.Fatalf("%s: page did not render its case: %q", route, rendered)
							}
							continue
						}
						got := outputTextCaseHTML(t, rendered)
						value, present := got[row.name]
						if !present || len(got) != 1 {
							t.Fatalf("%s: HTML=%q", route, got)
						}
						want := outputTextCaseHTML(t, `<div data-case="expected">`+row.value+`</div>`)["expected"]
						if strings.Contains(want, "j_id") {
							// Generated ids and component whitespace differ; compare the text.
							value, want = strings.Join(strings.Fields(posText(value)), ""), strings.Join(strings.Fields(posText(want)), "")
						}
						if value != want {
							t.Fatalf("%s: HTML=%q, native=%q", route, value, want)
						}
					}
				})
			}
			for _, row := range rows {
				if row.outcome != "error" {
					continue
				}
				t.Run("error/"+row.name, func(t *testing.T) {
					p, err := project.Load(shapesProject(t, api, map[string]bool{row.name: true}))
					if err != nil {
						t.Fatal(err)
					}
					_, err = visualforce.LoadProject(p)
					if err == nil {
						t.Fatalf("LoadProject succeeded, native error=%q", row.value)
					}
					if !strings.Contains(err.Error(), row.value) {
						t.Fatalf("error=%q, native=%q", err.Error(), row.value)
					}
				})
			}
		})
	}
}
