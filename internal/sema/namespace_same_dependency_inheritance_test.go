package sema

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

func TestSameNamespaceDependencyInheritedNestedType(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "standard_object_shape_compile")
	provider, err := os.ReadFile(filepath.Join(root, "classes", "NsOuter.cls"))
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := os.ReadFile(filepath.Join(root, "named", "I10.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			tmp := t.TempDir()
			consumerDir := filepath.Join(tmp, "consumer")
			dep := filepath.Join(tmp, "provider")
			for _, dir := range []string{filepath.Join(consumerDir, "force-app"), filepath.Join(dep, "force-app")} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			writeSemaFile(t, filepath.Join(consumerDir, "sfdx-project.json"), `{"namespace":"v036ns","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"synthetic-provider"}]}]}`)
			writeSemaFile(t, filepath.Join(consumerDir, "glade.yml"), "project:\n  dependencyPaths: [../provider]\n")
			writeSemaFile(t, filepath.Join(dep, "sfdx-project.json"), `{"namespace":"v036ns","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"package":"synthetic-provider"}]}`)
			writeSemaFile(t, filepath.Join(dep, "force-app", "NsOuter.cls"), string(provider))
			writeSemaFile(t, filepath.Join(consumerDir, "force-app", "I10.cls"), string(consumer))
			p, err := project.Load(consumerDir)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, schema.Schema{})
			loaded := false
			for _, d := range index.Dependencies {
				loaded = loaded || d.Status == "loaded"
			}
			if !loaded {
				t.Fatal("provider dependency not loaded")
			}
			for _, d := range Analyze(index).Diagnostics {
				if d.Severity == diagnostic.Error || strings.Contains(d.Message, "Pb") || strings.Contains(d.NativeMessage, "Pb") {
					t.Errorf("native I10 compiles, offline diagnostic: %+v", d)
				}
			}
		})
	}
}
