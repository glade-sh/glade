package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestReferencesReadOnlyCandidateFiles(t *testing.T) {
	dir := t.TempDir()
	idx := typesys.Index{}
	for i := 0; i < 200; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%03d.cls", i))
		text := "class Other {}"
		if i < 2 {
			text = "class Needle {}"
		}
		if err := os.WriteFile(p, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		idx.Types = append(idx.Types, typesys.TypeSymbol{File: p})
	}
	h := NewHandler(idx).snapshot()
	files := h.referenceCandidates("Needle")
	if len(files) != 2 {
		t.Fatalf("got %d candidates, want 2", len(files))
	}
	if got := h.referenceLocations("Needle"); len(got) != 2 {
		t.Fatalf("got %d locations, want 2", len(got))
	}
	p := idx.Types[2].File
	if err := os.WriteFile(p, []byte("class Needle { Integer changed; }"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := h.referenceLocations("Needle"); len(got) != 3 {
		t.Fatalf("external edit: got %d locations", len(got))
	}
}

func TestReferenceLocationsEqualFullScan(t *testing.T) {
	dir := t.TempDir()
	idx := typesys.Index{}
	for i := 0; i < 200; i++ {
		p := filepath.Join(dir, fmt.Sprintf("%03d.cls", i))
		text := fmt.Sprintf("class File%d { String needleSuffix; }", i)
		if i%10 == 0 {
			text += " NeEdLe NEEDLE"
		}
		if err := os.WriteFile(p, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		idx.Types = append(idx.Types, typesys.TypeSymbol{File: p})
	}
	h := NewHandler(idx).snapshot()
	for _, name := range []string{"needle", "File42", "missing", "NeedleSuffix"} {
		var want []Location
		for _, p := range h.sourceFiles() {
			text, ok := h.documentText(p)
			if !ok {
				t.Fatal(p)
			}
			for _, r := range wordRanges(text, name) {
				want = append(want, Location{URI: uriFromPath(p), Range: r})
			}
		}
		got := h.referenceLocations(name)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %#v want %#v", name, got, want)
		}
	}
}

func TestReferencesSeeUnsavedAndExternalEdits(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "A.cls")
	b := filepath.Join(dir, "B.cls")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("class Other {}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(typesys.Index{Types: []typesys.TypeSymbol{{File: a}, {File: b}}})
	h.snapshot().referenceCandidates("Needle")
	h.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uriFromPath(a), Text: "class Needle {}"}})
	if err := os.WriteFile(b, []byte("class Needle { Integer x; }"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := h.snapshot().referenceLocations("Needle"); len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
}

func TestReferencesSeeSameSizeExternalEditWithRestoredTime(t *testing.T) {
	p := filepath.Join(t.TempDir(), "A.cls")
	if err := os.WriteFile(p, []byte("class Otherx {}"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(typesys.Index{Types: []typesys.TypeSymbol{{File: p}}}).snapshot()
	if got := h.referenceLocations("Needle"); len(got) != 0 {
		t.Fatalf("initial references: %v", got)
	}
	if err := os.WriteFile(p, []byte("class Needle {}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if got := h.referenceLocations("Needle"); len(got) != 1 {
		t.Fatalf("references after external edit: %v", got)
	}
}
