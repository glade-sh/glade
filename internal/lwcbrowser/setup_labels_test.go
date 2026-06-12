package lwcbrowser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestSetupBundleIncludesLabelsSibling(t *testing.T) {
	root := writeSetupLabelsFixture(t)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "cache")
	_, compiled, err := PreparePageConfig(p, cache)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := compiled.Modules["c:setup"]; !ok {
		t.Fatal("missing c:setup")
	}
	labelsPath := filepath.Join(cache, "lwc", "c", "setup", "labels.js")
	if _, err := os.Stat(labelsPath); err != nil {
		t.Fatalf("missing labels.js: %v", err)
	}
	body, err := os.ReadFile(labelsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Greeting") {
		t.Fatalf("unexpected labels.js: %s", body[:min(200, len(body))])
	}
}

func writeSetupLabelsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeSetupLabelsFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	bundleDir := filepath.Join(root, "force-app", "main", "default", "lwc", "setup")
	writeSetupLabelsFile(t, filepath.Join(bundleDir, "setup.html"), `<template><span>{label}</span></template>`)
	writeSetupLabelsFile(t, filepath.Join(bundleDir, "setup.js"), `import { LightningElement } from 'lwc';
import { labels } from './labels';

export default class Setup extends LightningElement {
  label = labels.Greeting;
}
`)
	writeSetupLabelsFile(t, filepath.Join(bundleDir, "labels.js"), `export const labels = { Greeting: "Hello from Glade" };`)
	writeSetupLabelsFile(t, filepath.Join(bundleDir, "setup.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <apiVersion>62.0</apiVersion>
  <isExposed>true</isExposed>
</LightningComponentBundle>`)
	return root
}

func writeSetupLabelsFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSetupImportMapIncludesLocalComponents(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "lightning-out-vf")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "cache")
	cfg, _, err := PreparePageConfig(p, cache)
	if err != nil {
		t.Fatal(err)
	}
	imports := LocalLWCImportMap(cfg.Namespace, cfg.Manifest)
	if imports["c/counter"] == "" {
		t.Fatal("missing c/counter import map entry")
	}
	if imports["c/apexWireHost"] == "" {
		t.Fatal("missing c/apexWireHost import map entry")
	}
}
