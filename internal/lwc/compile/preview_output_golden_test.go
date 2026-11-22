package compile

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Compiled LWC and CSS output for a fixed project is pinned to the digest
// origin/main produced before the preview changes.
func TestPreviewCompileOutputMatchesBaseline(t *testing.T) {
	if os.Getenv("GLADE_LWC_COMPILE") == "" {
		if _, err := os.Stat(filepath.Join("..", "..", "..", "third_party", "lwc", "node_modules")); err != nil {
			t.Skip("run npm install in third_party/lwc or set GLADE_LWC_COMPILE=1")
		}
	}
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(filepath.Join(root, "testdata", "local-tests", "lwc-rendering"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dist")
	if _, err := Compile(p, Options{OutDir: out, Namespace: "c", RepoRoot: root}); err != nil {
		t.Fatal(err)
	}
	var files []string
	err = filepath.WalkDir(out, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			files = append(files, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	sum := sha256.New()
	for _, file := range files {
		rel, _ := filepath.Rel(out, file)
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ReplaceAll(string(data), filepath.ToSlash(out), "<out>")
		text = strings.ReplaceAll(text, out, "<out>")
		fmt.Fprintf(sum, "%s\x00%d\x00%s\x00", filepath.ToSlash(rel), len(text), text)
	}
	const want = "3753f836bb648b8ae1d7ed7a372bf0604b5b28c36778ba800f047d9535b1a30b"
	if got := fmt.Sprintf("%x", sum.Sum(nil)); got != want || len(files) == 0 {
		t.Fatalf("compiled output digest = %s over %d files, want %s", got, len(files), want)
	}
}
