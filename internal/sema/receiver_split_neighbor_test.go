package sema

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// These uncaptured neighbors retain the serialized diagnostics from origin/main.
func TestReceiverSplitUncapturedNeighbors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ReceiverProbe.cls")
	writeSemaFile(t, path, "public class ReceiverProbe { public static String value() { return 'ok'; } }")
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{path}}, schema.Schema{})
	cases := []string{
		"System.assertEquals('ok', String.valueOf(String.valueOf(ReceiverProbe.\nvalue())));",
		"System.assertEquals('ok', ReceiverProbe.// comment\nvalue());",
		"System.assertEquals('ok', ReceiverProbe.\nvalue() + '');",
	}
	for _, source := range cases {
		result := AnalyzeAnonymous(index, source, "67.0")
		data, err := json.Marshal(result.Diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "null" {
			t.Errorf("source %q: diagnostics %s, want origin/main null", source, data)
		}
	}
}
