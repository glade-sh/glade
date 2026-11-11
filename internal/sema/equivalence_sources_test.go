package sema

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forEachRepositoryApexSource calls fn with every .cls and .trigger source under
// the repository root, plus any directories listed in GLADE_SEMA_EQUIV_DIRS
// (os.PathListSeparator-separated). Scanner rewrites use it to pin a new
// implementation to the pattern it replaces on real inputs.
func forEachRepositoryApexSource(t *testing.T, fn func(path, source string)) {
	t.Helper()
	roots := []string{filepath.Join("..", "..")}
	if extra := os.Getenv("GLADE_SEMA_EQUIV_DIRS"); extra != "" {
		roots = append(roots, strings.Split(extra, string(os.PathListSeparator))...)
	}
	files := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if base := entry.Name(); base == "node_modules" || base == ".git" || base == "web" {
					return filepath.SkipDir
				}
				return nil
			}
			if ext := filepath.Ext(path); ext != ".cls" && ext != ".trigger" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			files++
			fn(path, string(data))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if files == 0 {
		t.Fatal("no Apex sources found under the repository")
	}
}
