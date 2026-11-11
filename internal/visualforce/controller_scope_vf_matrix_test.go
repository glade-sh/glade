package visualforce_test

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

// This helper is called from TestCapturedEnclosingStaticVisualforceRoute so
// that the VF matrix remains part of the same nonparallel top-level test.
func runCapturedControllerScopeVisualforceMatrix(t *testing.T) {
	const fixture = "testdata/controller_scope_vf"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	if len(rows) != 32 || strings.Join(rows[0], "|") != "page_api|apex_api|case|route|outcome" {
		t.Fatalf("incomplete native matrix: %d rows", len(rows))
	}
	type cell struct{ deploy, render string }
	cells := map[string]cell{}
	for _, row := range rows[1:] {
		if len(row) != 5 {
			t.Fatalf("invalid native row: %v", row)
		}
		key := row[0] + "/" + row[1] + "/" + row[2]
		value := cells[key]
		switch row[3] {
		case "metadata-deploy":
			if value.deploy != "" {
				t.Fatalf("duplicate deployment row: %s", key)
			}
			value.deploy = row[4]
		case "vf-getContent":
			if value.render != "" {
				t.Fatalf("duplicate render row: %s", key)
			}
			value.render = row[4]
		default:
			t.Fatalf("unexpected native route: %v", row)
		}
		cells[key] = value
	}
	names := []string{"modifier_good", "modifier_bad", "comparison_good", "comparison_bad", "override_private", "override_protected", "loop_after", "loop_before", "sibling_good", "sibling_bad"}
	if len(cells) != 2*len(names) {
		t.Fatalf("native cells = %d, want %d", len(cells), 2*len(names))
	}
	rejections := map[string]string{
		"modifier_bad":     "Missing ';' at 'Integer'",
		"comparison_bad":   "Comparison arguments must be compatible types: Integer, Boolean",
		"override_private": "override methods require at least one of the following global, public, protected",
		"loop_before":      "Duplicate variable: i",
		"sibling_bad":      "Illegal assignment from Integer to Map<String,Object>",
	}
	for _, pair := range [][2]string{{"59.0", "62.0"}, {"67.0", "67.0"}} {
		for _, name := range names {
			pageAPI, apexAPI := pair[0], pair[1]
			key := pageAPI + "/" + apexAPI + "/" + name
			value, ok := cells[key]
			if !ok {
				t.Fatalf("missing native case: %s", key)
			}
			reject := name == "modifier_bad" || name == "comparison_bad" || name == "loop_before" || name == "sibling_bad" || (name == "override_private" && apexAPI == "67.0")
			if reject {
				if !strings.Contains(value.deploy, rejections[name]) || value.render != "" {
					t.Fatalf("invalid native reject: %s: %+v", key, value)
				}
			} else if value.deploy != "accepted" || value.render != "passed" {
				t.Fatalf("invalid native accept: %s: %+v", key, value)
			}
			t.Run(key, func(t *testing.T) {
				root := t.TempDir()
				write := func(path, body string) {
					t.Helper()
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(body), 0644); err != nil {
						t.Fatal(err)
					}
				}
				read := func(name string) string {
					t.Helper()
					body, err := os.ReadFile(filepath.Join(fixture, name))
					if err != nil {
						t.Fatal(err)
					}
					return string(body)
				}
				write(filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+apexAPI+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				class := filepath.Join(root, "force-app/main/default/classes/CaseController.cls")
				write(class, read(filepath.Join("cases", name+".apex")))
				write(class+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+apexAPI+`</apiVersion><status>Active</status></ApexClass>`)
				page := filepath.Join(root, "force-app/main/default/pages/CasePage.page")
				write(page, read("CasePage.page"))
				write(page+"-meta.xml", `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+pageAPI+`</apiVersion><label>Case Page</label></ApexPage>`)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				sch, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, sch)
				analysis := sema.Analyze(index)
				if reject {
					if !index.HasErrors() && !analysis.HasErrors() {
						t.Fatalf("native-rejected controller accepted: %s", value.deploy)
					}
					// These three controls have stable, specific semantic codes.
					// For lexical/comparison rejects, keep all diagnostics visible.
					want := map[string][]string{"override_private": {"GLADESEMA016", "GLADESEMA032"}, "loop_before": {"GLADESEMA014"}, "sibling_bad": {"GLADESEMA018"}}[name]
					if len(want) > 0 {
						matched := false
						for _, diag := range analysis.Diagnostics {
							for _, code := range want {
								if diag.Code == code {
									matched = true
								}
							}
						}
						if !matched {
							t.Fatalf("native reject lacked its semantic diagnostic: %v", analysis.Diagnostics)
						}
					}
					return
				}
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native-accepted controller rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				vf, err := visualforce.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				machine := vm.New(nil)
				if err := apextest.RegisterProjectRuntimeForRequest(machine, index); err != nil {
					t.Fatal(err)
				}
				visualforce.SetVMRenderEnvironment(machine, p)
				defer visualforce.ClearVMRenderEnvironment(machine)
				result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: vf, Machine: machine, PageName: "CasePage"})
				if err != nil || result.Error != nil || !strings.Contains(result.HTML, "VF_NATIVE_OK") {
					t.Fatalf("native VF output: %v / %+v", err, result)
				}
				program, err := vm.CompileAnonymousWithOptions(read("anonymous.apex"), vm.CompileOptions{APIVersion: apexAPI})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatalf("native Page.getContent: %v", err)
				}
			})
		}
	}
}
