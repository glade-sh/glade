package visualforce

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/vm"
	nethtml "golang.org/x/net/html"
)

// PDFRenderer converts rendered Visualforce HTML into PDF bytes.
type PDFRenderer interface {
	RenderPDF(ctx context.Context, html string, baseURL string) ([]byte, error)
}

// PDFToolchain is the Glade-managed PDF renderer entry point.
type PDFToolchain struct {
	Command string
}

func (p PDFToolchain) RenderPDF(ctx context.Context, html string, baseURL string) ([]byte, error) {
	if strings.TrimSpace(p.Command) == "" {
		return renderBasicPDF(html, baseURL), nil
	}
	return runPDFCommand(ctx, p.Command, html, baseURL)
}

var (
	pdfRendererMu sync.Mutex
	pdfRenderer   PDFRenderer = PDFToolchain{}
)

func SetPDFRenderer(renderer PDFRenderer) {
	pdfRendererMu.Lock()
	defer pdfRendererMu.Unlock()
	if renderer == nil {
		renderer = PDFToolchain{}
	}
	pdfRenderer = renderer
}

func SetPDFRendererForTest(renderer PDFRenderer) func() {
	pdfRendererMu.Lock()
	previous := pdfRenderer
	if renderer == nil {
		renderer = PDFToolchain{}
	}
	pdfRenderer = renderer
	pdfRendererMu.Unlock()
	return func() {
		pdfRendererMu.Lock()
		pdfRenderer = previous
		pdfRendererMu.Unlock()
	}
}

func renderPDF(ctx context.Context, html string, baseURL string) ([]byte, error) {
	pdfRendererMu.Lock()
	renderer := pdfRenderer
	pdfRendererMu.Unlock()
	if renderer == nil {
		renderer = PDFToolchain{}
	}
	return renderer.RenderPDF(ctx, html, baseURL)
}

func RenderPDFContent(ctx context.Context, html string, baseURL string) ([]byte, error) {
	return renderPDF(ctx, html, baseURL)
}

func runPDFCommand(ctx context.Context, command string, html string, baseURL string) ([]byte, error) {
	command = strings.TrimSpace(command)
	if !filepath.IsAbs(command) {
		return nil, vm.UnsupportedFeature("PageReference.getContentAsPDF requires an absolute PDF toolchain command path")
	}
	cmd := exec.CommandContext(ctx, command, baseURL)
	cmd.Stdin = strings.NewReader(html)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("PDF toolchain failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("PDF toolchain produced no output")
	}
	return out, nil
}

func renderBasicPDF(rawHTML string, baseURL string) []byte {
	lines := pdfTextLines(rawHTML, baseURL)
	if len(lines) == 0 {
		lines = []string{" "}
	}
	var stream bytes.Buffer
	stream.WriteString("BT\n/F1 12 Tf\n72 720 Td\n14 TL\n")
	for _, line := range lines {
		stream.WriteString("(")
		stream.WriteString(escapePDFString(line))
		stream.WriteString(") Tj\nT*\n")
	}
	stream.WriteString("ET\n")
	return buildSinglePagePDF(stream.Bytes())
}

func pdfTextLines(rawHTML string, baseURL string) []string {
	text := strings.TrimSpace(extractHTMLText(rawHTML))
	if text == "" {
		text = strings.TrimSpace(rawHTML)
	}
	if text == "" && baseURL != "" {
		text = baseURL
	}
	words := strings.Fields(text)
	var lines []string
	var line strings.Builder
	for _, word := range words {
		if line.Len() > 0 && line.Len()+1+len(word) > 72 {
			lines = append(lines, line.String())
			line.Reset()
		}
		if line.Len() > 0 {
			line.WriteByte(' ')
		}
		line.WriteString(word)
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	if len(lines) > 42 {
		lines = lines[:42]
	}
	return lines
}

func extractHTMLText(rawHTML string) string {
	root, err := nethtml.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return ""
	}
	var b strings.Builder
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode {
			switch strings.ToLower(node.Data) {
			case "head", "script", "input", "textarea":
				return
			}
			if pdfElementHiddenFromText(node) {
				return
			}
		}
		if node.Type == nethtml.TextNode {
			text := strings.TrimSpace(node.Data)
			if text != "" {
				if b.Len() > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(text)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return b.String()
}

func pdfElementHiddenFromText(node *nethtml.Node) bool {
	for _, attr := range node.Attr {
		switch strings.ToLower(attr.Key) {
		case "class":
			for _, className := range strings.Fields(attr.Val) {
				if strings.EqualFold(className, "actionStatusStart") {
					return true
				}
			}
		case "style":
			if strings.Contains(strings.ToLower(strings.ReplaceAll(attr.Val, " ", "")), "display:none") {
				return true
			}
		}
	}
	return false
}

func escapePDFString(text string) string {
	var b strings.Builder
	for _, r := range text {
		switch r {
		case '\\', '(', ')':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\r', '\n', '\t':
			b.WriteByte(' ')
		default:
			if r >= 32 && r <= 126 {
				b.WriteRune(r)
			} else {
				b.WriteByte('?')
			}
		}
	}
	return b.String()
}

func buildSinglePagePDF(content []byte) []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), string(content)),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xrefOffset := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", len(objects)+1)
	io.WriteString(&b, "0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)
	return b.Bytes()
}
