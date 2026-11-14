package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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

func TestReferencesDetectSameSizeReplacementWithRestoredMtime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.cls")
	if err := os.WriteFile(path, []byte("class Otherx {}"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(typesys.Index{Types: []typesys.TypeSymbol{{File: path}}}).snapshot()
	if got := h.referenceLocations("Needle"); len(got) != 0 {
		t.Fatalf("initial references: %#v", got)
	}
	if err := os.WriteFile(path, []byte("class Needle {}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if got := h.referenceLocations("Needle"); len(got) != 1 {
		t.Fatalf("same-size replacement: got %#v, want one reference", got)
	}
}

func BenchmarkReferenceCandidatesWorkspace(b *testing.B) {
	dir := b.TempDir()
	idx := typesys.Index{}
	for i := 0; i < 200; i++ {
		path := filepath.Join(dir, fmt.Sprintf("%03d.cls", i))
		text := "class Other {\n" + strings.Repeat("  Integer value;\n", 120) + "}\n"
		if i < 2 {
			text += "// Needle\n"
		}
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			b.Fatal(err)
		}
		idx.Types = append(idx.Types, typesys.TypeSymbol{File: path})
	}
	h := NewHandler(idx).snapshot()
	if len(h.referenceCandidates("Needle")) != 2 {
		b.Fatal("unexpected candidates")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if len(h.referenceCandidates("Needle")) != 2 {
			b.Fatal("unexpected candidates")
		}
	}
}

func TestReferenceCandidateInvalidation(t *testing.T) {
	for _, scenario := range []string{"size", "mtime", "missing", "initially missing", "new generation"} {
		t.Run(scenario, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "Example.cls")
			if scenario != "initially missing" {
				if err := os.WriteFile(path, []byte("class Otherx {}"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			index := typesys.Index{Types: []typesys.TypeSymbol{{File: path}}}
			handler := NewHandler(index)
			h := handler.snapshot()
			h.referenceCandidates("Needle")
			switch scenario {
			case "size":
				if err := os.WriteFile(path, []byte("class Needle { Integer x; }"), 0600); err != nil {
					t.Fatal(err)
				}
			case "mtime":
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, info.ModTime().Add(time.Second), info.ModTime().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "initially missing":
				if err := os.WriteFile(path, []byte("class Needle {}"), 0600); err != nil {
					t.Fatal(err)
				}
			case "new generation":
				if err := os.WriteFile(path, []byte("class Needle {}"), 0600); err != nil {
					t.Fatal(err)
				}
				handler.refreshMu.Lock()
				handler.installBaseLocked(Base{Index: index}, func(g *generation) { handler.gen.Store(g) })
				handler.refreshMu.Unlock()
				h = handler.snapshot()
			}
			got := h.referenceCandidates("Needle")
			if len(got) != 1 || got[0] != path {
				t.Fatalf("candidates = %#v", got)
			}
		})
	}
}

func TestReferenceAndRenameAfterMetadataPreservingReplacement(t *testing.T) {
	dir := t.TempDir()
	declaration := filepath.Join(dir, "Needle.cls")
	changed := filepath.Join(dir, "Other.cls")
	if err := os.WriteFile(declaration, []byte("class Needle {}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(changed, []byte("class Otherx {}"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(changed)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(typesys.Index{Types: []typesys.TypeSymbol{{Name: "Needle", File: declaration}, {Name: "Otherx", File: changed}}})
	uri := uriFromPath(declaration)
	params := ReferenceParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Character: 8}, Context: ReferenceContext{IncludeDeclaration: true}}
	if got := h.References(params); len(got) != 1 {
		t.Fatalf("initial references = %#v", got)
	}
	if err := os.WriteFile(changed, []byte("class Needle {}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(changed, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if got := h.References(params); len(got) != 2 {
		t.Fatalf("references = %#v", got)
	}
	edit := h.Rename(RenameParams{TextDocument: params.TextDocument, Position: params.Position, NewName: "Replacement"})
	if edit == nil || len(edit.Changes[uriFromPath(changed)]) != 1 {
		t.Fatalf("rename = %#v", edit)
	}
}
