package sema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestRTEMEventRoutesNative(t *testing.T) {
	data, err := os.ReadFile("testdata/rtem_event_routes/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Route, Case, Source, Message string
		Compiled                          bool
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		t.Run(row.API+"/"+row.Route+"/"+row.Case, func(t *testing.T) {
			root := t.TempDir()
			var files []string
			if row.Route == "named" {
				path := filepath.Join(root, "Probe.cls")
				writeSemaFile(t, path, "public class Probe { public static void run() { "+row.Source+" } }")
				files = append(files, path)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row.API, ApexFiles: files}, schema.Schema{})
			result := Analyze(index)
			if row.Route == "anonymous" {
				result = AnalyzeAnonymous(index, row.Source, row.API)
			}
			if got := !index.HasErrors() && !result.HasErrors(); got != row.Compiled {
				t.Fatalf("native compiled=%v (%s); Glade compiled=%v: index=%v sema=%v", row.Compiled, row.Message, got, index.Diagnostics, result.Diagnostics)
			}
		})
	}
}
