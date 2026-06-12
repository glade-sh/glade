package compile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestCompileProjectLWCBundles(t *testing.T) {
	if os.Getenv("GLADE_LWC_COMPILE") == "" {
		if _, err := os.Stat(filepath.Join("..", "..", "..", "third_party", "lwc", "node_modules")); err != nil {
			t.Skip("run npm install in third_party/lwc or set GLADE_LWC_COMPILE=1")
		}
	}
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "testdata", "local-tests", "lwc-rendering")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Compile(p, Options{
		OutDir:    filepath.Join(t.TempDir(), "dist"),
		Namespace: "c",
		RepoRoot:  root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Modules) == 0 {
		t.Fatalf("modules = %#v", manifest.Modules)
	}
}

func TestCompileRewritesTemplateStylesheetImports(t *testing.T) {
	if os.Getenv("GLADE_LWC_COMPILE") == "" {
		if _, err := os.Stat(filepath.Join("..", "..", "..", "third_party", "lwc", "node_modules")); err != nil {
			t.Skip("run npm install in third_party/lwc or set GLADE_LWC_COMPILE=1")
		}
	}
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "testdata", "local-tests", "lightning-out-vf")
	p, err := project.Load(fixture)
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "dist")
	_, err = Compile(p, Options{OutDir: outDir, Namespace: "c", RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	htmlJS, err := os.ReadFile(filepath.Join(outDir, "c", "counter", "counter.html.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(htmlJS)
	if strings.Contains(src, `from "./counter.css"`) {
		t.Fatalf("expected template CSS import rewrite, got:\n%s", src)
	}
	if !strings.Contains(src, `from "./counter.css.js"`) {
		t.Fatalf("missing counter.css.js import in:\n%s", src)
	}
}

func TestCompileEmitsSiblingJSModules(t *testing.T) {
	if os.Getenv("GLADE_LWC_COMPILE") == "" {
		if _, err := os.Stat(filepath.Join("..", "..", "..", "third_party", "lwc", "node_modules")); err != nil {
			t.Skip("run npm install in third_party/lwc or set GLADE_LWC_COMPILE=1")
		}
	}
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixtureDir := filepath.Join(t.TempDir(), "sibling-fixture")
	bundleDir := filepath.Join(fixtureDir, "force-app", "main", "default", "lwc", "widget")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCompileFixtureFile(t, filepath.Join(bundleDir, "widget.html"), `<template><p>{label.title}</p></template>`)
	writeCompileFixtureFile(t, filepath.Join(bundleDir, "widget.js"), `import { LightningElement } from 'lwc';
import { labels } from './labels';
export default class Widget extends LightningElement {
  label = labels;
}`)
	writeCompileFixtureFile(t, filepath.Join(bundleDir, "labels.js"), `export const labels = { title: "Hello" };`)
	writeCompileFixtureFile(t, filepath.Join(bundleDir, "widget.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><isExposed>true</isExposed></LightningComponentBundle>`)

	p, err := project.Load(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "dist")
	_, err = Compile(p, Options{OutDir: outDir, Namespace: "c", RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "c", "widget", "labels.js")); err != nil {
		t.Fatalf("missing compiled sibling module: %v", err)
	}
}

func writeCompileFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
