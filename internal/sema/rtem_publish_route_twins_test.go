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

// Captured deployment results cover named and @IsTest declarations. Repeated
// analysis exercises the same compiled index after the first analysis.
func TestRTEMPublishRouteTwinsNative(t *testing.T) {
	raw, err := os.ReadFile("testdata/rtem_publish_route_twins/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Route, Case, Source, Message string
		Compiled                          bool
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 28 {
		t.Fatalf("expected 28 native rows, got %d", len(rows))
	}
	for _, row := range rows {
		t.Run(row.API+"/"+row.Route+"/"+row.Case, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "Probe.cls")
			source := "public class Probe { public static void run() { " + row.Source + " } }"
			if row.Route == "istest" {
				source = "@IsTest private class Probe { @IsTest static void run() { " + row.Source + " } }"
			}
			writeSemaFile(t, path, source)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row.API, ApexFiles: []string{path}}, schema.Schema{})
			for _, pass := range []string{"fresh", "warm"} {
				result := Analyze(index)
				if got := !index.HasErrors() && !result.HasErrors(); got != row.Compiled {
					t.Errorf("%s native compiled=%v (%s); Glade compiled=%v: index=%v sema=%v", pass, row.Compiled, row.Message, got, index.Diagnostics, result.Diagnostics)
				}
			}
		})
	}
}
