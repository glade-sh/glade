package lsp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestDiskParseFailureRespectsBufferAndDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "First.cls")
	broken := []byte("public class First {")
	if err := os.WriteFile(path, broken, 0600); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(typesys.Index{})
	var output bytes.Buffer
	s := &server{h: h, w: &output, ctx: context.Background()}
	failure := &ParseDiagnosticsError{Path: path, Digest: sha256.Sum256(broken), Diagnostics: []diagnostic.Diagnostic{{File: path, Severity: diagnostic.Error, Message: "parse error"}}}
	uri := uriFromPath(path)
	h.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Text: "public class First {}"}})
	s.publishDiskParseFailure(failure, h.gen.Load())
	if output.Len() != 0 {
		t.Fatal("disk error published over open buffer")
	}
	h.applyClose(uri)
	if got := h.diagnosticsForDocument(uri); len(got) != 1 || got[0].Message != "parse error" {
		t.Fatalf("closed diagnostics = %#v", got)
	}
	if err := os.WriteFile(path, []byte("public class First {}"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := h.diagnosticsForDocument(uri); len(got) != 0 {
		t.Fatalf("stale diagnostics = %#v", got)
	}
	output.Reset()
	s.publishDiskParseFailure(failure, h.gen.Load())
	if output.Len() != 0 {
		t.Fatal("published diagnostics for superseded disk contents")
	}
}

func TestReplacingDiagnosticsKeepsGenerationImmutable(t *testing.T) {
	original := []diagnostic.Diagnostic{{File: "first.cls", Message: "old"}, {File: "second.cls", Message: "keep"}}
	updated := replaceFileDiagnostics(original, "first.cls", []diagnostic.Diagnostic{{File: "first.cls", Message: "parse"}})
	if original[0].Message != "old" || original[1].Message != "keep" {
		t.Fatalf("mutated generation: %#v", original)
	}
	updated[0].Message = "changed"
	if original[1].Message != "keep" {
		t.Fatalf("replacement shares generation slice: %#v", original)
	}
}
