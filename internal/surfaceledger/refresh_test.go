package surfaceledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/capability"
)

func TestRefreshWritesLedgerReportsAndSnapshots(t *testing.T) {
	docs := t.TempDir()
	out := t.TempDir()
	writeDoc(t, docs, "apex/system_label.md", "# Label Class\n\n### get(String section, String key)\n")
	toolingPath := filepath.Join(t.TempDir(), "tooling.json")
	writeTooling(t, toolingPath)

	result, err := Refresh(RefreshOptions{
		DocsSource:          docs,
		ToolingCompletions:  toolingPath,
		EvidenceFixtureGlob: nil,
		OutputDir:           out,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range OutputFileNames() {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	if result.Summary.Total == 0 {
		t.Fatalf("empty refresh summary")
	}
}

func writeTooling(t *testing.T, path string) {
	t.Helper()
	completions := capability.ToolingCompletions{
		PublicDeclarations: map[string]map[string]capability.ToolingClassDecl{
			"System": {"Label": {Methods: []capability.ToolingMethod{{Name: "get", ReturnType: "String", Parameters: []capability.ToolingParameter{{Type: "String"}, {Type: "String"}}}}}},
		},
	}
	data, err := marshalPretty(completions)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
