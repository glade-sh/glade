package compile

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Bundles with the lightning__dynamicComponent capability may load a
// component whose name is only known at run time; others need a literal.
func TestVariableDynamicImportRequiresDynamicComponentCapability(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, capable := range []bool{true, false} {
		fixtureDir := filepath.Join(t.TempDir(), "dynamic-import")
		componentDir := filepath.Join(fixtureDir, "force-app", "main", "default", "lwc", "loader")
		writeCompileFixtureFile(t, filepath.Join(fixtureDir, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"64.0"}`)
		writeCompileFixtureFile(t, filepath.Join(componentDir, "loader.js"), `import { LightningElement, api } from 'lwc';
export default class Loader extends LightningElement {
  @api componentName = 'c/child';
  view;
  async connectedCallback() {
    const { default: ctor } = await import(this.componentName);
    this.view = ctor;
  }
  get label() {
    return this.view ? "ready" : "loading";
  }
}`)
		writeCompileFixtureFile(t, filepath.Join(componentDir, "loader.html"), `<template><p>{label}</p></template>`)
		capabilities := ""
		if capable {
			capabilities = `<capabilities><capability>lightning__dynamicComponent</capability></capabilities>`
		}
		writeCompileFixtureFile(t, filepath.Join(componentDir, "loader.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>64.0</apiVersion><isExposed>true</isExposed>`+capabilities+`</LightningComponentBundle>`)

		p, err := project.Load(fixtureDir)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Compile(p, Options{OutDir: filepath.Join(t.TempDir(), "dist"), Namespace: "c", RepoRoot: root})
		if capable && err != nil {
			t.Fatalf("capable bundle: %v", err)
		}
		if !capable && (err == nil || !strings.Contains(err.Error(), "import")) {
			t.Fatalf("bundle without capability: err = %v, want a dynamic import diagnostic", err)
		}
	}
}
