package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStandardDefinitionCacheLSPNamedRoutesAPI62And67(t *testing.T) {
	// This source was deployed at both APIs by the runtime isolation fixture.
	source, err := os.ReadFile("../apextest/testdata/conformance/runtime_schema_isolation/fixture/force-app/main/default/classes/SchemaIsolationProbe.cls")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			definition, ok := storage.StandardObjectDefinition("Account")
			if !ok {
				t.Fatal("missing Account definition")
			}
			delete(definition.Fields, "Name")
			root := t.TempDir()
			path := filepath.Join(root, "SchemaIsolationProbe.cls")
			if err := os.WriteFile(path, source, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path+"-meta.xml", []byte("<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>"), 0600); err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
			handler := NewHandler(index)
			defer handler.parser.Close()
			for _, d := range handler.diagnosticsForDocument(uriFromPath(path)) {
				if d.Severity == diagnostic.Error {
					t.Fatalf("native-valid LSP source: %v", d)
				}
			}
		})
	}
}
