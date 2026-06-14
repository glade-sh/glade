package visualforce

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPDFToolchainRendersBasicPDFWithoutExternalCommand(t *testing.T) {
	body, err := PDFToolchain{}.RenderPDF(context.Background(), "<html><body>Invoice Total</body></html>", "/apex/Invoice")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(body), "%PDF-1.4") {
		t.Fatalf("pdf = %q, want PDF header", string(body[:min(len(body), 20)]))
	}
	if !strings.Contains(string(body), "Invoice Total") {
		t.Fatalf("pdf omitted page text:\n%s", string(body))
	}
}

func TestPDFTextLinesMatchesVisualforcePDFVisibleText(t *testing.T) {
	lines := pdfTextLines(`<html><body>
<style>body { font-family: sans-serif; }</style>
<span class="actionStatusStart">Working</span><span class="actionStatusStop">Done</span>
<select><option>United States</option><option>Canada</option></select>
</body></html>`, "/apex/Probe")
	got := strings.Join(lines, " ")
	want := "body { font-family: sans-serif; } Done United States Canada"
	if got != want {
		t.Fatalf("pdf text = %q, want %q", got, want)
	}
}

func TestRenderPageURLUsesConfiguredPDFRendererForRenderAsPDF(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Invoice.page"), `<apex:page renderAs="pdf"><h1>Invoice Total</h1></apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	SetVMRenderEnvironment(machine, p)
	renderer := &testPDFRenderer{pdf: []byte("%PDF-1.4\ninvoice")}
	restore := SetPDFRendererForTest(renderer)
	defer restore()

	body, err := RenderPageURL(machine, "/apex/Invoice", false)
	if err != nil {
		t.Fatal(err)
	}
	if body.Kind != vm.ValueObject || body.Type != "Blob" || body.Fields["value"].Text != string(renderer.pdf) {
		t.Fatalf("body = %#v, want PDF blob", body)
	}
	if !strings.Contains(renderer.html, "Invoice Total") {
		t.Fatalf("pdf renderer html = %q", renderer.html)
	}
	if renderer.baseURL != "/apex/Invoice" {
		t.Fatalf("pdf renderer baseURL = %q", renderer.baseURL)
	}
}

func TestRenderPageURLChecksPDFOutputSize(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Invoice.page"), `<apex:page renderAs="pdf"><h1>Invoice Total</h1></apex:page>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	SetVMRenderEnvironment(machine, p)
	restore := SetPDFRendererForTest(&testPDFRenderer{pdf: bytes.Repeat([]byte("x"), MaxVisualforcePDFBytes+1)})
	defer restore()

	_, err = RenderPageURL(machine, "/apex/Invoice", false)
	if err == nil || !strings.Contains(err.Error(), "visualforce pdf limit exceeded") {
		t.Fatalf("err = %v, want PDF size limit", err)
	}
}

type testPDFRenderer struct {
	html    string
	baseURL string
	pdf     []byte
}

func (r *testPDFRenderer) RenderPDF(ctx context.Context, html string, baseURL string) ([]byte, error) {
	r.html = html
	r.baseURL = baseURL
	return r.pdf, nil
}
