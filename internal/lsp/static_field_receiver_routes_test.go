package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStaticFieldReceiverLSPNamedTestAndTriggerRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/static_field_receiver_routes"
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"", "orchard"} {
			t.Run(api+"/namespace="+namespace, func(t *testing.T) {
				root := t.TempDir()
				var paths []string
				for _, name := range []string{"ReceiverValues.cls", "StaticFieldReceiverProbe.cls", "StaticFieldReceiverProbeTest.cls", "StaticFieldReceiverProbeTrigger.trigger"} {
					data, err := os.ReadFile(filepath.Join(fixture, name))
					if err != nil {
						t.Fatal(err)
					}
					source := string(data)
					if namespace != "" {
						source = strings.ReplaceAll(source, "ReceiverValues.Text", namespace+".ReceiverValues.Text")
					}
					path := filepath.Join(root, name)
					if err := os.WriteFile(path, []byte(source), 0600); err != nil {
						t.Fatal(err)
					}
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
				if index.HasErrors() {
					t.Fatalf("index errors: %v", index.Diagnostics)
				}
				handler := NewHandler(index)
				defer handler.parser.Close()
				for _, path := range paths[1:] {
					t.Run(filepath.Base(path), func(t *testing.T) {
						for _, d := range handler.diagnosticsForDocument(uriFromPath(path)) {
							if d.Severity == diagnostic.Error {
								t.Errorf("static field receiver rejected: %v", d)
							}
						}
					})
				}
			})
		}
	}
}
