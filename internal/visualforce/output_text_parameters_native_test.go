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
	"golang.org/x/net/html"
)

const outputTextParameterFamily = "testdata/output_text_parameters"

func TestOutputTextParametersNativeMarkupRoute(t *testing.T) {
	controller := vm.Object("FormatProbeController")
	controller.Fields["amount"] = vm.Decimal(12345.625)
	controller.Fields["amounts"] = vm.List(vm.Decimal(12345.625))
	controller.Fields["negative"] = vm.Decimal(-12345.625)
	controller.Fields["tie"] = vm.Decimal(2.5)
	controller.Fields["zero"] = vm.Decimal(0)
	controller.Fields["caption"] = vm.String("<em>Blue & green</em>")
	controller.Fields["missing"] = vm.Null
	for name, value := range map[string]string{"day": "2024-02-09", "stamp": "2024-02-09T17:06:03Z"} {
		scalar := vm.Object("Date")
		if name == "stamp" {
			scalar.Type = "Datetime"
		}
		scalar.Fields["value"] = vm.String(value)
		controller.Fields[name] = scalar
	}
	for _, api := range []string{"59.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, route := range []string{"Direct", "DataList"} {
				t.Run(route, func(t *testing.T) {
					name := "FormatProbe" + route
					body, err := os.ReadFile(filepath.Join(outputTextParameterFamily, "project/force-app/main/default/pages", name+".page"))
					if err != nil {
						t.Fatal(err)
					}
					tree, err := visualforce.ParseMarkupTree(string(body))
					if err != nil {
						t.Fatal(err)
					}
					rendered, err := visualforce.RenderMarkupTree(tree, &visualforce.RenderContext{PageName: name, PageMeta: visualforce.Page{APIVersion: api}, Expression: &visualforce.ExpressionContext{Controller: controller}})
					if err != nil {
						t.Fatal(err)
					}
					outputTextAssertNative(t, api, route, rendered)
				})
			}
		})
	}
}

// The same native pages exercise controller loading, custom-component and include
// controllers, and row scope in repeat/dataList/dataTable/pageBlockTable. Both public
// rendering entry points must produce the captured parameter values.
func TestOutputTextParametersNativeProjectRoutes(t *testing.T) {
	for _, api := range []string{"59.0", "62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(outputTextParameterFamily, "project")
			err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				target := filepath.Join(root, relative)
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				return os.WriteFile(target, []byte(strings.ReplaceAll(string(body), "67.0", api)), 0644)
			})
			if err != nil {
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
			for _, route := range []string{"Direct", "Custom", "Include", "Repeat", "DataList", "DataTable", "PageBlockTable"} {
				for _, entry := range []string{"RenderPage", "AnonymousGetContent"} {
					t.Run(route+"/"+entry, func(t *testing.T) {
						machine := vm.New(nil)
						if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
							t.Fatal(err)
						}
						visualforce.SetVMRenderEnvironment(machine, p)
						defer visualforce.ClearVMRenderEnvironment(machine)
						name := "FormatProbe" + route
						var rendered string
						if entry == "RenderPage" {
							result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: index, Machine: machine, PageName: name})
							if err != nil {
								t.Fatal(err)
							}
							if result.Error != nil {
								t.Fatal(result.Error)
							}
							rendered = result.HTML
						} else {
							program, err := vm.CompileAnonymous(fmt.Sprintf("System.debug(new PageReference('/apex/%s').getContent().toString());", name))
							if err != nil {
								t.Fatal(err)
							}
							result, err := machine.Execute(program)
							if err != nil {
								t.Fatal(err)
							}
							if len(result.Debug) != 1 {
								t.Fatalf("debug rows = %d, want 1", len(result.Debug))
							}
							rendered = result.Debug[0]
						}
						outputTextAssertNative(t, api, route, rendered)
					})
				}
			}
		})
	}
}

func outputTextAssertNative(t *testing.T, api, route, rendered string) {
	t.Helper()
	f, err := os.Open(filepath.Join(outputTextParameterFamily, "api"+api[:2], "org.tsv"))
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
	got := outputTextCaseHTML(t, rendered)
	count := 0
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatalf("invalid native row: %v", row)
		}
		if row[0] != api || row[1] != route {
			continue
		}
		count++
		want := outputTextCaseHTML(t, `<div data-case="expected">`+row[3]+`</div>`)["expected"]
		t.Run(row[2], func(t *testing.T) {
			if actual, ok := got[row[2]]; !ok || actual != want {
				t.Errorf("HTML = %q (present=%t), native = %q", actual, ok, want)
			}
		})
	}
	if count != 12 || len(got) != 12 {
		t.Fatalf("native cases=%d local cases=%d, want 12", count, len(got))
	}
}

func outputTextCaseHTML(t *testing.T, source string) map[string]string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{}
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		for _, a := range node.Attr {
			if a.Key != "data-case" {
				continue
			}
			var body strings.Builder
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				if err := html.Render(&body, c); err != nil {
					t.Fatal(err)
				}
			}
			if _, exists := cases[a.Val]; exists {
				t.Fatalf("duplicate case %s", a.Val)
			}
			cases[a.Val] = body.String()
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(doc)
	return cases
}
