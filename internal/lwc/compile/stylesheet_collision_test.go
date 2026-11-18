package compile

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// The output names of local and outside-bundle imports must stay distinct.
// This checks Glade's output graph, not Salesforce deployment acceptance.
func TestCompileSharedStylesheetOutputCollision(t *testing.T) {
	repoRoot, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "sharedStyles")
	writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "sharedStyles.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "sharedStyles.css"), `@import './__external_styles/0.css'; @import '../external.css'; .root { color: red; }`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "__external_styles", "0.css"), `.local-marker { color: blue; }`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "..", "external.css"), `.outside-marker { color: green; }`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Compile(p, Options{OutDir: filepath.Join(root, "dist"), Namespace: "c", RepoRoot: repoRoot})
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := manifest.Modules["c:sharedStyles"]
	if !ok {
		t.Fatal("shared stylesheet module missing from manifest")
	}
	code, err := os.ReadFile(entry.File)
	if err != nil {
		t.Fatal(err)
	}
	imports := regexp.MustCompile(`(?m)^import\s+\S+\s+from\s+"([^"]+)"`).FindAllStringSubmatch(string(code), -1)
	if len(imports) != 2 {
		t.Fatalf("expected two output imports; got %q", code)
	}
	if imports[0][1] == imports[1][1] {
		t.Fatalf("distinct sources collided at output %s", imports[0][1])
	}
	for i, marker := range []string{".local-marker", ".outside-marker"} {
		file := filepath.Join(filepath.Dir(entry.File), filepath.FromSlash(imports[i][1]))
		data, err := os.ReadFile(file)
		if err != nil || !strings.Contains(string(data), marker) {
			t.Errorf("compiled module %s: contents %q, error %v; want %q", file, data, err, marker)
		}
	}
}
