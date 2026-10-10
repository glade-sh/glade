package vm_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPageReferenceGetContentRendersLocalVisualforcePage(t *testing.T) {
	root := makePageReferenceContentProject(t, `<apex:page><h1>Invoice Total</h1></apex:page>`)
	machine := compileContentProject(t, root)
	visualforce.SetVMRenderEnvironment(machine, mustLoadProject(t, root))

	program, err := vm.CompileAnonymous(`
Blob body = Page.Invoice.getContent();
System.assert(body.toString().contains('Invoice Total'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestPageReferenceGetContentSurfacesVisualforceRenderFailureAsExecutionException(t *testing.T) {
	root := makePageReferenceContentProject(t, `<apex:page><h1>Invoice Total</h1></apex:page>`)
	machine := compileContentProject(t, root)
	visualforce.SetVMRenderEnvironment(machine, mustLoadProject(t, root))

	program, err := vm.CompileAnonymous(`
Boolean caught = false;
try {
    new PageReference('/apex/Missing').getContent();
} catch (ExecutionException e) {
    caught = true;
}
System.assertEquals(true, caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestPageReferenceGetContentPreservesUnsupportedVisualforceGlobal(t *testing.T) {
	root := makePageReferenceContentProject(t, `<apex:page><h1>Global Probe</h1></apex:page>`)
	writePageRenderTestFile(t, filepath.Join(root, "force-app/main/default/pages/Global.page"), `<apex:page><apex:outputText value="{!$Action.Widget.save}"/></apex:page>`)
	machine := compileContentProject(t, root)
	visualforce.SetVMRenderEnvironment(machine, mustLoadProject(t, root))

	program, err := vm.CompileAnonymous(`
Boolean caught = false;
try {
    new PageReference('/apex/Global').getContent();
} catch (ExecutionException e) {
    caught = true;
}
System.assertEquals(false, caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "$Action") {
		t.Fatalf("err = %v, want unsupported $Action diagnostic", err)
	}
}

func TestPageReferenceGetContentPreservesUnsupportedVisualforceSurface(t *testing.T) {
	for _, method := range []string{"getContent", "getContentAsPDF"} {
		t.Run(method, func(t *testing.T) {
			root := makePageReferenceContentProject(t, `<apex:page><flow:interview name="demo"/></apex:page>`)
			machine := compileContentProject(t, root)
			visualforce.SetVMRenderEnvironment(machine, mustLoadProject(t, root))

			program, err := vm.CompileAnonymous(`
Boolean caught = false;
try {
    Page.Invoice.` + method + `();
} catch (ExecutionException e) {
    caught = true;
}
System.assertEquals(false, caught);
`)
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			var runtimeErr *vm.RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || !strings.Contains(err.Error(), "flow:interview") {
				t.Fatalf("err = %v, want unsupported flow:interview diagnostic", err)
			}
		})
	}
}

func TestPageReferenceGetContentRestoresCurrentPageContext(t *testing.T) {
	root := makePageReferenceContentProject(t, `<apex:page><h1>Invoice Total</h1></apex:page>`)
	writePageRenderTestFile(t, filepath.Join(root, "force-app/main/default/pages/Inner.page"), `<apex:page><span>Inner Body</span></apex:page>`)
	machine := compileContentProject(t, root)
	machine.EnableTestContext()
	visualforce.SetVMRenderEnvironment(machine, mustLoadProject(t, root))

	program, err := vm.CompileAnonymous(`
Test.setCurrentPage(new PageReference('/apex/Outer?mode=edit'));
Blob body = new PageReference('/apex/Inner').getContent();
System.assert(body.toString().contains('Inner Body'));
System.assertEquals('/apex/Outer?mode=edit', ApexPages.currentPage().getUrl());
System.assertEquals('edit', ApexPages.currentPage().getParameters().get('mode'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestPageReferenceRenderPageURLUsesPDFRendererForRenderAsPDFPage(t *testing.T) {
	root := makePageReferenceContentProject(t, `<apex:page renderAs="pdf"><h1>Invoice Total</h1></apex:page>`)
	machine := compileContentProject(t, root)
	visualforce.SetVMRenderEnvironment(machine, mustLoadProject(t, root))
	recorder := &recordingPDFRenderer{pdf: []byte("%PDF-1.4\ninvoice")}
	restore := visualforce.SetPDFRendererForTest(recorder)
	defer restore()

	body, err := visualforce.RenderPageURL(machine, "/apex/Invoice", false)
	if err != nil {
		t.Fatal(err)
	}
	if body.Kind != vm.ValueObject || body.Type != "Blob" || body.Fields["value"].Text != string(recorder.pdf) {
		t.Fatalf("body = %#v, want PDF blob", body)
	}
	if !strings.Contains(recorder.html, "Invoice Total") {
		t.Fatalf("pdf renderer html = %q", recorder.html)
	}
	if recorder.baseURL != "/apex/Invoice" {
		t.Fatalf("pdf renderer baseURL = %q", recorder.baseURL)
	}
}

func TestPageReferenceGetContentAsPDFRendersDefaultPDFBlob(t *testing.T) {
	root := makePageReferenceContentProject(t, `<apex:page><apex:outputText value="Invoice Total"/></apex:page>`)
	machine := compileContentProject(t, root)
	visualforce.SetVMRenderEnvironment(machine, mustLoadProject(t, root))

	body, err := visualforce.RenderPageURL(machine, "/apex/Invoice", true)
	if err != nil {
		t.Fatal(err)
	}
	if body.Kind != vm.ValueObject || body.Type != "Blob" {
		t.Fatalf("body = %#v, want Blob", body)
	}
	pdf := body.Fields["value"].Text
	if !strings.HasPrefix(pdf, "%PDF-1.4") || !strings.Contains(pdf, "Invoice Total") {
		t.Fatalf("pdf blob = %q", pdf[:min(len(pdf), 80)])
	}
}

type recordingPDFRenderer struct {
	html    string
	baseURL string
	pdf     []byte
}

func (r *recordingPDFRenderer) RenderPDF(ctx context.Context, html string, baseURL string) ([]byte, error) {
	r.html = html
	r.baseURL = baseURL
	return r.pdf, nil
}

func makePageReferenceContentProject(t *testing.T, pageMarkup string) string {
	t.Helper()
	root := t.TempDir()
	writePageRenderTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writePageRenderTestFile(t, filepath.Join(root, "force-app/main/default/pages/Invoice.page"), pageMarkup)
	return root
}

func compileContentProject(t *testing.T, root string) *vm.VM {
	t.Helper()
	machine := vm.New(nil)
	// Register render metadata without requiring hosted deployment dependencies.
	// Source admission is covered by the separate Salesforce conformance tests.
	idx, err := visualforce.LoadProjectForRender(mustLoadProject(t, root))
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range idx.Pages {
		machine.RegisterPageReference(page.Name)
	}
	return machine
}

func mustLoadProject(t *testing.T, root string) project.Project {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writePageRenderTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
