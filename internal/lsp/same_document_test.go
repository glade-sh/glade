package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// referenceSameDocument is the unmemoized comparison.
func referenceSameDocument(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	ap := filepath.Clean(pathFromURI(DocumentURI(a)))
	bp := filepath.Clean(pathFromURI(DocumentURI(b)))
	if abs, err := filepath.Abs(ap); err == nil {
		ap = abs
	}
	if abs, err := filepath.Abs(bp); err == nil {
		bp = abs
	}
	return ap == bp
}

func TestSameDocumentMatchesUnmemoizedComparison(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "force-app", "A.cls")
	values := []string{
		"", file, filepath.Join(dir, "force-app", "..", "force-app", "A.cls"), "file://" + file,
		"file://" + filepath.Join(dir, "force-app", "B.cls"), "A.cls", "./force-app/A.cls", "force-app/A.cls",
		"glade-schema:///Account", "file:///no/such/dir/../x.cls",
	}
	check := func() {
		for _, a := range values {
			for _, b := range values {
				if got, want := sameDocument(a, b), referenceSameDocument(a, b); got != want {
					t.Fatalf("sameDocument(%q, %q) = %v, want %v", a, b, got, want)
				}
			}
		}
	}
	check()
	check() // answered from the memo
	// A relative path follows the working directory, so it must not be memoized.
	t.Chdir(filepath.Join(dir))
	check()
	if err := os.MkdirAll(filepath.Join(dir, "force-app"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(dir, "force-app"))
	check()
}

func BenchmarkSameDocumentAgainstIndex(b *testing.B) {
	files := make([]string, 5000)
	for i := range files {
		files[i] = fmt.Sprintf("/work/project/force-app/main/default/classes/Service%04d.cls", i)
	}
	uri := "file:///work/project/force-app/main/default/classes/Service4999.cls"
	for _, mode := range []string{"unmemoized", "current"} {
		b.Run(mode, func(b *testing.B) {
			same := sameDocument
			if mode == "unmemoized" {
				same = referenceSameDocument
			}
			for i := 0; i < b.N; i++ {
				matched := 0
				for _, file := range files {
					if same(uri, file) {
						matched++
					}
				}
				if matched != 1 {
					b.Fatalf("matched %d", matched)
				}
			}
		})
	}
}
