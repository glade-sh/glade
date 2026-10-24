package visualforce_test

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

const outputTextPatternFamily = "testdata/output_text_patterns"

func TestOutputTextPatternsNativeRenderingRoutes(t *testing.T) {
	for _, api := range []string{"59.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(outputTextPatternFamily, "project")
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
				target := filepath.Join(root, rel)
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				return os.WriteFile(target, []byte(strings.ReplaceAll(string(body), "67.0", api)), 0644)
			}); err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index, err := visualforce.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(typesys.Build(p, sch), nil)
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(filepath.Join(outputTextPatternFamily, "api"+api[:2], "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			reader := csv.NewReader(f)
			reader.Comma = '\t'
			rows, err := reader.ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 31 {
				t.Fatalf("native rows=%d, want 30", len(rows)-1)
			}
			for _, route := range []string{"Markup", "RenderPage", "AnonymousGetContent"} {
				t.Run(route, func(t *testing.T) {
					for _, row := range rows[1:] {
						t.Run(row[1], func(t *testing.T) {
							if row[0] != api {
								t.Fatalf("native API=%q, want %q", row[0], api)
							}
							machine := vm.New(nil)
							if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
								t.Fatal(err)
							}
							visualforce.SetVMRenderEnvironment(machine, p)
							defer visualforce.ClearVMRenderEnvironment(machine)
							var rendered string
							if route == "Markup" {
								body, err := os.ReadFile(filepath.Join(root, "force-app/main/default/pages", row[2]+".page"))
								if err != nil {
									t.Fatal(err)
								}
								tree, err := visualforce.ParseMarkupTree(string(body))
								if err != nil {
									t.Fatal(err)
								}
								controller := vm.Object("PatternProbeController")
								for key, amount := range map[string]float64{"amount": 12345.625, "negative": -12345.625, "ratio": 0.125, "zero": 0} {
									controller.Fields[key] = vm.Decimal(amount)
								}
								stamp := vm.Object("Datetime")
								stamp.Fields["value"] = vm.String("2024-02-09T17:06:03Z")
								controller.Fields["stamp"] = stamp
								day := vm.Object("Date")
								day.Fields["value"] = vm.String("2024-02-09")
								controller.Fields["day"] = day
								rendered, err = visualforce.RenderMarkupTree(tree, &visualforce.RenderContext{PageName: row[2], PageMeta: visualforce.Page{APIVersion: api}, Expression: &visualforce.ExpressionContext{Controller: controller}})
								if err != nil {
									t.Fatal(err)
								}
							} else if route == "RenderPage" {
								result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: index, Machine: machine, PageName: row[2]})
								if err != nil {
									t.Fatal(err)
								}
								if result.Error != nil {
									t.Fatal(result.Error)
								}
								rendered = result.HTML
							} else {
								program, err := vm.CompileAnonymous(fmt.Sprintf("System.debug(new PageReference('/apex/%s').getContent().toString());", row[2]))
								if err != nil {
									t.Fatal(err)
								}
								result, err := machine.Execute(program)
								if err != nil {
									t.Fatal(err)
								}
								if len(result.Debug) != 1 {
									t.Fatalf("debug rows=%d, want 1", len(result.Debug))
								}
								rendered = result.Debug[0]
							}
							got := outputTextCaseHTML(t, rendered)
							want := outputTextCaseHTML(t, `<div data-case="expected">`+row[3]+`</div>`)["expected"]
							if actual, present := got[row[1]]; !present || actual != want || len(got) != 1 {
								t.Fatalf("HTML=%q, native=%q", got, want)
							}
						})
					}
				})
			}
		})
	}
}
