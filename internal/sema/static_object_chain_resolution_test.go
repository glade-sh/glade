package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStaticObjectChainQualifiedFallbackResolvesStaticRootBeforeInstanceFields(t *testing.T) {
	const fixture = "../../testdata/conformance/static_object_chain_routes"
	cases, err := readTypeNamePinRows(filepath.Join(fixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"", "orchard"} {
			t.Run(api+"/namespace="+namespace, func(t *testing.T) {
				root := t.TempDir()
				var paths []string
				for _, name := range []string{"Holder.cls", "ReceiverValues.cls"} {
					data, err := os.ReadFile(filepath.Join(fixture, name))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, name)
					writeSemaFile(t, path, string(data))
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
				if index.HasErrors() {
					t.Fatalf("index errors: %v", index.Diagnostics)
				}
				model := buildSemaTypeMemberView(prepareAnalysisIndex(index))
				for _, row := range cases[1:] {
					t.Run(row[0], func(t *testing.T) {
						receiver := row[1]
						if namespace != "" && row[0] != "missing_namespace" {
							receiver = strings.ReplaceAll(receiver, "ReceiverValues.", namespace+".ReceiverValues.")
						}
						if got, want := semaIRStaticValueReceiver(model, "ChainProbe", receiver), row[3] == "true"; got != want {
							t.Fatalf("qualified fallback receiver %q resolved=%v, want %v", receiver, got, want)
						}
					})
				}
			})
		}
	}
}
