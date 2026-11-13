package lsp

import (
	"fmt"
	"os"
	"path/filepath"
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
