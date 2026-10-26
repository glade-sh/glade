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

func TestDottedReceiverNativeAnonymousNamedTestAndTriggerRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/type_name_resolution_pins"
	cases, err := readTypeNamePinRows(filepath.Join(fixture, "receiver_routes/cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, row := range cases[1:] {
		sources[row[1]+"/"+row[0]] = row[2]
	}
	rows, err := readTypeNamePinRows(filepath.Join(fixture, "receiver_routes/org.tsv"))
	if err != nil || len(rows) != 25 {
		t.Fatalf("native rows: %d, %v", len(rows), err)
	}
	for _, row := range rows[1:] {
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			if row[3] != "false" || !strings.HasPrefix(row[4], "Variable does not exist: ") {
				t.Fatalf("invalid native rejection: %v", row)
			}
			source, err := os.ReadFile(filepath.Join(fixture, sources[row[1]+"/"+row[2]]))
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			var paths []string
			for _, name := range []string{"KnownType", "TypeNamePinInstance"} {
				data, err := os.ReadFile(filepath.Join(fixture, "project/force-app/main/default/classes", name+".cls"))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, name+".cls")
				writeSemaFile(t, path, string(data))
				paths = append(paths, path)
			}
			if row[1] != "anonymous" {
				path := filepath.Join(root, filepath.Base(sources[row[1]+"/"+row[2]]))
				writeSemaFile(t, path, string(source))
				paths = append(paths, path)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[0], ApexFiles: paths}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
			if index.HasErrors() {
				t.Fatalf("index errors: %v", index.Diagnostics)
			}
			result := Analyze(index)
			if row[1] == "anonymous" {
				result = AnalyzeAnonymous(index, string(source), row[0])
			}
			for _, d := range result.Diagnostics {
				if d.NativeMessage == row[4] {
					if d.Range == nil || d.Range.Start.Line < 1 {
						t.Fatalf("receiver rejection has no source location: %v", d)
					}
					return
				}
			}
			t.Fatalf("native rejected with %q; local diagnostics=%v", row[4], result.Diagnostics)
		})
	}
}
