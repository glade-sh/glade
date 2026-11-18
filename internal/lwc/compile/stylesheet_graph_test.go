package compile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// A compiled stylesheet's relative imports must point to compiled modules.
// This checks local output integrity, not Salesforce deployment acceptance.
func TestCompileSharedStylesheetRelativeGraph(t *testing.T) {
	repoRoot, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "sharedStyles")
	writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "sharedStyles.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "sharedStyles.css"), `@import './parts/base.css'; .outer { color: red; }`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "parts", "base.css"), `@import '../palette.css'; .base { color: blue; }`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "palette.css"), `.palette { color: green; }`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "dist")
	manifest, err := Compile(p, Options{OutDir: out, Namespace: "c", RepoRoot: repoRoot})
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := manifest.Modules["c:sharedStyles"]
	if !ok {
		t.Fatal("shared stylesheet module missing from manifest")
	}
	for _, want := range []struct{ path, content string }{
		{entry.File, `from "./parts/base.css.js"`},
		{filepath.Join(filepath.Dir(entry.File), "parts", "base.css.js"), `from "../palette.css.js"`},
		{filepath.Join(filepath.Dir(entry.File), "palette.css.js"), ".palette"},
	} {
		data, err := os.ReadFile(want.path)
		if err != nil || !strings.Contains(string(data), want.content) {
			t.Errorf("compiled module %s: contents %q, error %v; want %q", want.path, data, err, want.content)
		}
	}
}
