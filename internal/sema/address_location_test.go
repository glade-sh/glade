package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAddressLocationAssignmentIdentity(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shadow := range []string{"", "Address", "Location"} {
			root := t.TempDir()
			files := []string{}
			if shadow != "" {
				file := filepath.Join(root, shadow+".cls")
				writeSemaFile(t, file, "public class "+shadow+" {}")
				files = append(files, file)
			}
			index := typesys.Build(project.Project{Root: root, ApexFiles: files, SourceAPIVersion: api}, schema.Schema{})
			for _, tc := range []struct {
				name, body string
				reject     bool
			}{
				{"qualified", "System.Address a; System.Location l=a;", false},
				{"bare", "Address a; Location l=a;", shadow != ""},
				{"reverse", "System.Location l; System.Address a=l;", true},
				{"schema_source", "Schema.Address a; System.Location l=a;", true},
				{"schema_target", "System.Address a; Schema.Location l=a;", true},
			} {
				t.Run(api+"/"+shadow+"/"+tc.name, func(t *testing.T) {
					result := AnalyzeAnonymous(index, tc.body, api)
					if result.HasErrors() != tc.reject {
						t.Fatalf("reject=%v diagnostics=%v", tc.reject, result.Diagnostics)
					}
				})
			}
		}
	}
}
