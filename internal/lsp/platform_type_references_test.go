package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native: ../../testdata/conformance/platform_type_references/org.tsv,
// platform_types named/@IsTest at APIs 62 and 67.
func TestPlatformTypeReferencesLSPRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/platform_type_references"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			var paths []string
			files, err := filepath.Glob(filepath.Join(fixture, "*.cls"))
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 4 {
				t.Fatalf("fixture classes=%d", len(files))
			}
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, filepath.Base(file))
				writeReceiverMemberFile(t, path, string(data))
				paths = append(paths, path)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			h := NewHandler(index)
			defer h.parser.Close()
			for _, path := range paths {
				t.Run(filepath.Base(path), func(t *testing.T) {
					if ds := h.diagnosticsForDocument(uriFromPath(path)); len(ds) != 0 {
						t.Fatal(ds)
					}
				})
			}
		})
	}
}

// Native invalid_system @IsTest rows reject this body at both source APIs.
func TestPlatformTypeReferencesLSPInvalidSystem(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "PlatformInvalidTest.cls")
			writeReceiverMemberFile(t, path, "@IsTest private class PlatformInvalidTest { @IsTest static void observe() { System.NotAType value = null; } }")
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
			h := NewHandler(index)
			defer h.parser.Close()
			ds := h.diagnosticsForDocument(uriFromPath(path))
			for _, d := range ds {
				if d.NativeMessage == "Invalid type: System.NotAType" {
					return
				}
			}
			t.Fatalf("native rejection missing: %+v", ds)
		})
	}
}
