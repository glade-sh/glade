//go:build cgo

package apexast

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseExampleProjects(t *testing.T) {
	root := "example-projects"
	if _, err := filepath.Abs(root); err != nil {
		t.Fatalf("resolve example-projects: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Skipf("%s not available: %v", root, err)
	}

	parser := NewParser()
	var total int
	var failures []File
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".cls") && !strings.HasSuffix(path, ".trigger") {
			return nil
		}
		total++
		file, err := parser.ParseFile(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			return nil
		}
		if file.HasErrors() {
			failures = append(failures, file)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	for _, file := range failures {
		for _, diagnostic := range file.Diagnostics {
			if diagnostic.Severity != Error {
				continue
			}
			if diagnostic.Range != nil {
				t.Logf("%s:%d:%d: %s: %s", file.Path, diagnostic.Range.Start.Line, diagnostic.Range.Start.Column, diagnostic.Message, diagnostic.Excerpt)
			} else {
				t.Logf("%s: %s", file.Path, diagnostic.Message)
			}
		}
	}
	if len(failures) != 0 {
		t.Fatalf("parsed %d Apex files with %d failures", total, len(failures))
	}
	if total == 0 {
		t.Fatalf("no Apex files found under %s", root)
	}
}
