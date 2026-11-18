package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The nested argument position is captured by org.tsv:52-53.
func TestReceiverSplitUncapturedNeighbors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ReceiverProbe.cls")
	writeSemaFile(t, path, "public class ReceiverProbe { public static String value() { return 'ok'; } }")
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{path}}, schema.Schema{})
	// An additional nested call still reports at the outer statement's opener.
	cases := []string{
		"System.assertEquals('ok', String.valueOf(String.valueOf(ReceiverProbe.\nvalue())));",
	}
	for _, source := range cases {
		result := AnalyzeAnonymous(index, source, "67.0")
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].NativeMessage != "Unexpected token '('." {
			t.Errorf("source %q: diagnostics %v, want captured argument message", source, result.Diagnostics)
		}
	}
}
