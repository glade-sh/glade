package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func overlayTestHandler(t *testing.T, sources map[string]string) (*Handler, map[string]DocumentURI) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"classes","default":true}],"sourceApiVersion":"67.0"}`), 0644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "classes")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	uris := map[string]DocumentURI{}
	for name, text := range sources {
		path := filepath.Join(dir, name+".cls")
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
		uris[name] = uriFromPath(path)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(typesys.Build(p, schema.Schema{})), uris
}
func overlayOpen(h *Handler, uri DocumentURI, text string) []Notification {
	return h.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: 1, Text: text}})
}
func overlayChange(t *testing.T, h *Handler, uri DocumentURI, text string) []Notification {
	t.Helper()
	n, err := h.DidChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 2}, ContentChanges: []TextDocumentContentChangeEvent{{Text: text}}})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestOverlayRevertReturnsBaseGeneration(t *testing.T) {
	original := "public class First { void run() { Integer x = 1; } }"
	h, uris := overlayTestHandler(t, map[string]string{"First": original})
	overlayOpen(h, uris["First"], original)
	base := h.gen.Load()
	changed := overlayChange(t, h, uris["First"], "public class First { void run() { Integer x = 'wrong'; } }")
	if diagnosticsCount(t, changed) == 0 {
		t.Fatal("missing edited diagnostic")
	}
	result := overlayChange(t, h, uris["First"], original)
	if h.gen.Load() != base || diagnosticsCount(t, result) != 0 {
		t.Fatal("revert did not restore base")
	}
}

func TestParseErrorKeepsLastCleanOverlay(t *testing.T) {
	original := "public class First {}"
	h, uris := overlayTestHandler(t, map[string]string{"First": original})
	overlayOpen(h, uris["First"], original)
	overlayChange(t, h, uris["First"], "public class First { void added() {} }")
	before := h.gen.Load()
	result := overlayChange(t, h, uris["First"], "public class First {")
	if h.gen.Load() != before || diagnosticsCount(t, result) == 0 {
		t.Fatal("parse error did not retain clean overlay")
	}
}

func TestOverlayTwoDirtyBuffers(t *testing.T) {
	a := "public class First { void run() {} }"
	b := "public class Second {}"
	h, uris := overlayTestHandler(t, map[string]string{"First": a, "Second": b})
	overlayOpen(h, uris["First"], a)
	overlayOpen(h, uris["Second"], b)
	overlayChange(t, h, uris["First"], "public class First { void run() { Second.added(); } }")
	overlayChange(t, h, uris["Second"], "public class Second { public static void added() {} }")
	if got := diagnosticsCount(t, h.currentDiagnostics(uris["First"])); got != 0 {
		t.Fatalf("diagnostics with two buffers = %d", got)
	}
	h.DidClose(DidCloseTextDocumentParams{TextDocument: TextDocumentIdentifier{URI: uris["Second"]}})
	if got := diagnosticsCount(t, h.currentDiagnostics(uris["First"])); got == 0 {
		t.Fatal("closing unsaved dependency produced no diagnostics")
	}
}

func TestSymbolsFollowUnsavedEdit(t *testing.T) {
	initial := "public class First {}"
	h, uris := overlayTestHandler(t, map[string]string{"First": initial})
	overlayOpen(h, uris["First"], initial)
	overlayChange(t, h, uris["First"], "public class First { public void added() {} }")
	symbols := h.DocumentSymbols(uris["First"])
	if len(symbols) != 1 {
		t.Fatalf("symbols: %#v", symbols)
	}
	found := false
	for _, child := range symbols[0].Children {
		if child.Name == "added" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing unsaved declaration: %#v", symbols)
	}
}

func TestSaveAfterOverlayPublishesDiskGeneration(t *testing.T) {
	initial := "public class First {}"
	edited := "public class First { void added() {} }"
	h, uris := overlayTestHandler(t, map[string]string{"First": initial})
	overlayOpen(h, uris["First"], initial)
	overlayChange(t, h, uris["First"], edited)
	if !h.gen.Load().index.Volatile() {
		t.Fatal("expected volatile overlay")
	}
	path := pathFromURI(uris["First"])
	if err := os.WriteFile(path, []byte(edited), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(h.gen.Load().index.Project.Root)
	if err != nil {
		t.Fatal(err)
	}
	h.SetRefresher(func(context.Context, RefreshRequest) (Base, error) {
		idx, artifacts := typesys.BuildWithArtifacts(p, schema.Schema{})
		return Base{Index: idx, Project: p, Artifacts: &artifacts}, nil
	})
	if err := h.Refresh(context.Background(), []string{path}); err != nil {
		t.Fatal(err)
	}
	if h.gen.Load().index.Volatile() {
		t.Fatal("saved generation is volatile")
	}
}

// A handler with a rebuild hook never falls back to the fixed index: when no
// generation reflects the buffer, no semantic diagnostics are published.
func TestUnreflectedBufferPublishesNothing(t *testing.T) {
	original := "public class First { void run() { Integer x = 'wrong'; } }"
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"classes","default":true}],"sourceApiVersion":"67.0"}`), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "classes", "First.cls")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	h := NewPendingHandler()
	rebuilds := 0
	h.SetBase(Base{
		Index:   typesys.Build(p, schema.Schema{}),
		Project: project.Project{Root: root}, // a different identity: the updater declines
		RebuildOverlay: func(context.Context, *typesys.SourceOverlay) (typesys.Index, typesys.BuildArtifacts, error) {
			rebuilds++
			return typesys.Index{}, typesys.BuildArtifacts{}, os.ErrInvalid
		},
	})
	uri := uriFromPath(path)
	if got := diagnosticsCount(t, overlayOpen(h, uri, original)); got == 0 {
		t.Fatal("disk text has no diagnostic")
	}
	if got := overlayChange(t, h, uri, "public class First { void run() { Integer x = 1; } }"); got != nil {
		t.Fatalf("published from a generation that does not reflect the buffer: %#v", got)
	}
	if rebuilds == 0 {
		t.Fatal("rebuild hook not called")
	}
}

// A buffer past the per-document cap stays out of the overlay and gets
// parser diagnostics only.
func TestOversizedBufferGetsParserDiagnosticsOnly(t *testing.T) {
	original := "public class First { void run() { Integer x = 'wrong'; } }"
	h, uris := overlayTestHandler(t, map[string]string{"First": original})
	overlayOpen(h, uris["First"], original)
	base := h.gen.Load()
	big := original + "\n//" + strings.Repeat("x", maxOverlayDocumentBytes)
	if got := diagnosticsCount(t, overlayChange(t, h, uris["First"], big)); got != 0 {
		t.Fatalf("oversized buffer diagnostics = %d", got)
	}
	if h.gen.Load() != base {
		t.Fatal("oversized buffer entered the overlay")
	}
}
