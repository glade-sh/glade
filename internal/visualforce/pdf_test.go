package visualforce

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	if !strings.Contains(pdfInflatedText(t, body), "(Invoice Total)Tj") {
		t.Fatalf("pdf omitted page text:\n%s", string(body))
	}
	for _, want := range []string{"/BaseFont/Times-Roman", "/Encoding/WinAnsiEncoding", "/Filter/FlateDecode", "/MediaBox[0 0 612 792]"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestPDFTextLinesMatchesVisualforcePDFVisibleText(t *testing.T) {
	body, err := PDFToolchain{}.RenderPDF(context.Background(), `<html><body>
<style>body { font-family: sans-serif; }</style>
<span class="actionStatusStart">Working</span><span class="actionStatusStop">Done</span>
<select><option>United States</option><option>Canada</option></select>
</body></html>`, "/apex/Probe")
	if err != nil {
		t.Fatal(err)
	}
	got := pdfInflatedText(t, body)
	want := "(Done United States Canada)Tj"
	if !strings.Contains(got, want) {
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

func pdfInflatedText(t *testing.T, data []byte) string {
	t.Helper()
	var out strings.Builder
	for _, m := range regexp.MustCompile(`/Length (\d+)>>stream\n`).FindAllSubmatchIndex(data, -1) {
		n, _ := strconv.Atoi(string(data[m[2]:m[3]]))
		r, err := zlib.NewReader(bytes.NewReader(data[m[1] : m[1]+n]))
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		out.Write(raw)
	}
	return out.String()
}

func TestPagePDFLocalLayoutRouteTwins(t *testing.T) {
	for _, renderAs := range []bool{false, true} {
		for _, asPDF := range []bool{false, true} {
			if !renderAs && !asPDF {
				continue
			}
			t.Run(fmt.Sprintf("renderAs=%v/asPDF=%v", renderAs, asPDF), func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
				attr := ""
				if renderAs {
					attr = ` renderAs="pdf"`
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/pages/Invoice.page"), `<apex:page`+attr+`><p>one</p><div style="page-break-before:always"></div><p>two</p><select><option>United States</option><option>Canada</option></select></apex:page>`)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				machine := vm.New(nil)
				SetVMRenderEnvironment(machine, p)
				defer ClearVMRenderEnvironment(machine)
				body, err := RenderPageURL(machine, "/apex/Invoice", asPDF)
				if err != nil {
					t.Fatal(err)
				}
				data := []byte(body.Fields["value"].Text)
				if !bytes.Contains(data, []byte("/Type/Pages/Count 2")) || !bytes.Contains(data, []byte("/BaseFont/Times-Roman")) {
					t.Fatal("missing shared PDF layout")
				}
				text := pdfInflatedText(t, data)
				if !strings.Contains(text, "(one)Tj") || !strings.Contains(text, "(two)Tj") || !strings.Contains(text, "(United States Canada)Tj") {
					t.Fatalf("text=%q", text)
				}
			})
		}
	}
}

func TestPagePDFExternalCommandFromProjectConfiguration(t *testing.T) {
	for _, renderAs := range []bool{false, true} {
		t.Run(fmt.Sprint(renderAs), func(t *testing.T) {
			root := t.TempDir()
			command := filepath.Join(root, "render-pdf")
			if err := os.WriteFile(command, []byte("#!/bin/sh\ncat >/dev/null\nprintf '%%PDF-1.4 external renderer'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "glade.yml"), "project:\n  root: .\npdf:\n  command: '"+command+"'\n")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			attr := ""
			if renderAs {
				attr = ` renderAs="pdf"`
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/pages/Invoice.page"), `<apex:page`+attr+`>Invoice</apex:page>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			machine := vm.New(nil)
			SetVMRenderEnvironment(machine, p)
			defer ClearVMRenderEnvironment(machine)
			body, err := RenderPageURL(machine, "/apex/Invoice", !renderAs)
			if err != nil {
				t.Fatal(err)
			}
			if body.Fields["value"].Text != "%PDF-1.4 external renderer" {
				t.Fatal("configured command was not used")
			}
		})
	}
}
