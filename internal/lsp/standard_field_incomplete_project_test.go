package lsp

import (
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"os"
	"path/filepath"
	"testing"
)

func TestStandardFieldIncompleteProjectLSPRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, declared := range []bool{false, true} {
			t.Run(api+map[bool]string{false: "/unknown", true: "/declared"}[declared], func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "Shape.cls")
				write := func(path, source string) {
					t.Helper()
					if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(source), 0600); err != nil {
						t.Fatal(err)
					}
				}
				write(path, "public class Shape { public static Account sample() { Account a=new Account(RecordTypeId=null); a.RecordTypeId=null; Id r=a.RecordTypeId; return a; } }")
				write(path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				if declared {
					write(filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer"}`)
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
					diagnostics := handler.diagnosticsForDocument(uriFromPath(path))
					rejected := index.HasErrors()
					for _, d := range diagnostics {
						rejected = rejected || d.Severity == diagnostic.Error
					}
					if rejected != declared {
						t.Fatalf("%s: rejected=%t, want %t: %v", pass, rejected, declared, diagnostics)
					}
				}
			})
		}
	}
}
