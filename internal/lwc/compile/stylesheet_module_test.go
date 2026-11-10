package compile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// A bundle that holds only a stylesheet is a shared CSS module. Components
// pull it in with `@import 'c/sharedStyles';`, so it must compile to a module.
func TestCompileEmitsCSSOnlyBundleAsStylesheetModule(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixtureDir := filepath.Join(t.TempDir(), "shared-css")
	lwcDir := filepath.Join(fixtureDir, "force-app", "main", "default", "lwc")
	writeCompileFixtureFile(t, filepath.Join(fixtureDir, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"64.0"}`)
	writeCompileFixtureFile(t, filepath.Join(lwcDir, "sharedStyles", "sharedStyles.css"), `.badge { color: red; }`)
	writeCompileFixtureFile(t, filepath.Join(lwcDir, "sharedStyles", "sharedStyles.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>64.0</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
	writeCompileFixtureFile(t, filepath.Join(lwcDir, "card", "card.js"), `import { LightningElement } from 'lwc';
export default class Card extends LightningElement {}`)
	writeCompileFixtureFile(t, filepath.Join(lwcDir, "card", "card.html"), `<template><span class="badge">New</span></template>`)
	writeCompileFixtureFile(t, filepath.Join(lwcDir, "card", "card.css"), "@import 'c/sharedStyles';\n")
	writeCompileFixtureFile(t, filepath.Join(lwcDir, "card", "card.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>64.0</apiVersion><isExposed>true</isExposed></LightningComponentBundle>`)

	p, err := project.Load(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "dist")
	if _, err := Compile(p, Options{OutDir: outDir, Namespace: "c", RepoRoot: root}); err != nil {
		t.Fatal(err)
	}
	module, err := os.ReadFile(filepath.Join(outDir, "c", "sharedStyles", "sharedStyles.js"))
	if err != nil {
		t.Fatalf("missing shared stylesheet module: %v", err)
	}
	if !strings.Contains(string(module), "badge") {
		t.Fatalf("shared stylesheet module lacks its rules:\n%s", module)
	}
}
