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

func TestCapturedEnclosingStaticVisualforceRoute(t *testing.T) {
	const fixture = "testdata/enclosing_static_vf"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 5 {
			t.Fatalf("invalid native row: %v", row)
		}
		pageAPI, apexAPI, name, route, outcome := row[0], row[1], row[2], row[3], row[4]
		key := pageAPI + "/" + apexAPI + "/" + name
		if seen[key] {
			t.Fatalf("duplicate native row: %s", key)
		}
		seen[key] = true
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
			classes := []string{"VfParent", "VfProbe", "VfController"}
			if name == "missing" {
				classes = append(classes, "VfReject")
			}
			for _, class := range classes {
				path := filepath.Join(root, "force-app/main/default/classes", class+".cls")
				write(path, read(class+".apex"))
				write(path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+apexAPI+`</apiVersion><status>Active</status></ApexClass>`)
			}
			page := filepath.Join(root, "force-app/main/default/pages/VfCapture.page")
			write(page, read("VfCapture.page"))
			write(page+"-meta.xml", `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+pageAPI+`</apiVersion><label>Capture</label></ApexPage>`)
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
			if name == "missing" {
				if route != "metadata-deploy" || !strings.Contains(outcome, "Method does not exist") || !analysis.HasErrors() {
					t.Fatalf("missing method accepted: native=%q diagnostics=%v", outcome, analysis.Diagnostics)
				}
				return
			}
			if name != "inherited" || route != "vf-getContent" || outcome != "passed" {
				t.Fatalf("invalid native case: %v", row)
			}
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("captured project rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
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
			result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: vf, Machine: machine, PageName: "VfCapture"})
			if err != nil || result.Error != nil || !strings.Contains(result.HTML, "VF_NATIVE_MARKER") {
				t.Fatalf("captured VF output: %v / %+v", err, result)
			}
			program, err := vm.CompileAnonymousWithOptions(read("anonymous.apex"), vm.CompileOptions{APIVersion: apexAPI})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatalf("captured Page.getContent: %v", err)
			}
		})
	}
	for _, key := range []string{"59.0/62.0/inherited", "59.0/62.0/missing", "67.0/67.0/inherited", "67.0/67.0/missing"} {
		if !seen[key] {
			t.Fatalf("missing native row: %s", key)
		}
	}
	if len(seen) != 4 {
		t.Fatalf("unexpected native rows: %v", seen)
	}
	t.Run("controller-scope-matrix", runCapturedControllerScopeVisualforceMatrix)
}
