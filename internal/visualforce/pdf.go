package visualforce

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

// PDFRenderer converts rendered Visualforce HTML into PDF bytes.
type PDFRenderer interface {
	RenderPDF(ctx context.Context, html string, baseURL string) ([]byte, error)
}

// PDFToolchain is the Glade-managed PDF renderer entry point.
type PDFToolchain struct {
	Command string
	machine *vm.VM
}

func (p PDFToolchain) RenderPDF(ctx context.Context, html string, baseURL string) ([]byte, error) {
	if strings.TrimSpace(p.Command) == "" {
		if p.machine != nil {
			return p.machine.RenderTextPDF(html), nil
		}
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

func renderPDFForVM(machine *vm.VM, ctx context.Context, html string, baseURL string) ([]byte, error) {
	command := ""
	if machine != nil {
		command = renderEnvironmentFromVM(machine).Project.PDF.Command
	}
	return renderPDFWithToolchain(machine, ctx, html, baseURL, command)
}

func renderPDFWithToolchain(machine *vm.VM, ctx context.Context, html, baseURL, command string) ([]byte, error) {
	pdfRendererMu.Lock()
	renderer := pdfRenderer
	pdfRendererMu.Unlock()
	var toolchain PDFToolchain
	switch renderer := renderer.(type) {
	case nil:
	case PDFToolchain:
		toolchain = renderer
	case *PDFToolchain:
		if renderer != nil {
			toolchain = *renderer
		}
	default:
		return renderer.RenderPDF(ctx, html, baseURL)
	}
	toolchain.machine = machine
	if command != "" {
		toolchain.Command = command
	}
	return toolchain.RenderPDF(ctx, html, baseURL)
}

func RenderPDFContent(ctx context.Context, html string, baseURL string, command ...string) ([]byte, error) {
	configured := ""
	if len(command) > 0 {
		configured = command[0]
	}
	return renderPDFWithToolchain(nil, ctx, html, baseURL, configured)
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

func renderBasicPDF(rawHTML string, _ string) []byte {
	return vm.RenderTextPDF(rawHTML, time.Unix(0, 0).UTC(), "glade "+vm.PDFVersion)
}
