package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// P0/org.tsv:85,187 captures the declared Developer org's WebStore type
// rejection at API 62/67. The source-only twin preserves Glade compatibility;
// it does not assert native availability for an unspecified org shape.
func TestStandardFieldEvidenceCommerceLSPShapeRoutes(t *testing.T) {
	definition, err := os.ReadFile("../apextest/testdata/conformance/standard_describe_profiles/profiles/P0.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, declared := range []bool{false, true} {
			name := map[bool]string{false: "source-only", true: "declared-P0"}[declared]
			t.Run(api+"/"+name, func(t *testing.T) {
				root := t.TempDir()
				write := func(relative, content string) {
					t.Helper()
					path := filepath.Join(root, relative)
					if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(content), 0600); err != nil {
						t.Fatal(err)
					}
				}
				write("OwnedStoreShape.cls", "public class OwnedStoreShape { public static WebStore make(){ return new WebStore(); } }")
				write("OwnedStoreShape.cls-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				if declared {
					write("config/project-scratch-def.json", string(definition))
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				p.SourceAPIVersion = api
				s, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, s)
				handler := NewHandler(index)
				defer handler.parser.Close()
				for _, pass := range []string{"cold", "warm"} {
					diagnostics := handler.diagnosticsForDocument(uriFromPath(filepath.Join(root, "OwnedStoreShape.cls")))
					rejected := index.HasErrors()
					for _, item := range diagnostics {
						rejected = rejected || item.Severity == diagnostic.Error
					}
					if rejected != declared {
						t.Fatalf("%s: rejected=%t, want %t: %v %v", pass, rejected, declared, index.Diagnostics, diagnostics)
					}
				}
			})
		}
	}
}
