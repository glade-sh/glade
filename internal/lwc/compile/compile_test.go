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

func TestCompileEmitsUtilityOnlyLWCModules(t *testing.T) {
	if os.Getenv("GLADE_LWC_COMPILE") == "" {
		if _, err := os.Stat(filepath.Join("..", "..", "..", "third_party", "lwc", "node_modules")); err != nil {
			t.Skip("run npm install in third_party/lwc or set GLADE_LWC_COMPILE=1")
		}
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Clean(filepath.Join(root, "..", "..", ".."))
	fixtureDir := filepath.Join(t.TempDir(), "utility-fixture")
	utilityDir := filepath.Join(fixtureDir, "force-app", "main", "default", "lwc", "bUtils")
	writeCompileFixtureFile(t, filepath.Join(utilityDir, "bUtils.js"), `export { classSet } from './classSet';`)
	writeCompileFixtureFile(t, filepath.Join(utilityDir, "classSet.js"), `export function classSet(values) { return values; }`)
	writeCompileFixtureFile(t, filepath.Join(utilityDir, "bUtils.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><isExposed>false</isExposed></LightningComponentBundle>`)

	p, err := project.Load(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "dist")
	manifest, err := Compile(p, Options{OutDir: outDir, Namespace: "c", RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := manifest.Modules["c:bUtils"]
	if !ok {
		t.Fatalf("manifest modules = %#v", manifest.Modules)
	}
	if entry.Tag != "" {
		t.Fatalf("utility module tag = %q, want empty", entry.Tag)
	}
	for _, rel := range []string{"c/bUtils/bUtils.js", "c/bUtils/classSet.js"} {
		if _, err := os.Stat(filepath.Join(outDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("missing compiled utility file %s: %v", rel, err)
		}
	}
	entryBytes, err := os.ReadFile(filepath.Join(outDir, "c", "bUtils", "bUtils.js"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(entryBytes), "bUtils.html.js") {
		t.Fatalf("utility entry was compiled as a component:\n%s", string(entryBytes))
	}
}

func TestCompileEmitsAdditionalHTMLTemplateModules(t *testing.T) {
	if os.Getenv("GLADE_LWC_COMPILE") == "" {
		if _, err := os.Stat(filepath.Join("..", "..", "..", "third_party", "lwc", "node_modules")); err != nil {
			t.Skip("run npm install in third_party/lwc or set GLADE_LWC_COMPILE=1")
		}
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Clean(filepath.Join(root, "..", "..", ".."))
	fixtureDir := filepath.Join(t.TempDir(), "template-fixture")
	componentDir := filepath.Join(fixtureDir, "force-app", "main", "default", "lwc", "paymentForm")
	writeCompileFixtureFile(t, filepath.Join(componentDir, "paymentForm.js"), `import { LightningElement } from 'lwc';
import cardForm from './forms/card.html';
import cashForm from './forms/cash.html';
import helper from './payform';
import formatCard from './forms/formatCard';
export default class PaymentForm extends LightningElement {
  connectedCallback() { helper.touch(); formatCard(); }
  render() { return this.method === 'Cash' ? cashForm : cardForm; }
}`)
	writeCompileFixtureFile(t, filepath.Join(componentDir, "paymentForm.html"), `<template></template>`)
	writeCompileFixtureFile(t, filepath.Join(componentDir, "payform.js"), `const helper = { touch() { return true; } };
export default helper;`)
	writeCompileFixtureFile(t, filepath.Join(componentDir, "forms", "card.html"), `<template><section class="card">Card</section></template>`)
	writeCompileFixtureFile(t, filepath.Join(componentDir, "forms", "card.css"), `.card { display: block; }`)
	writeCompileFixtureFile(t, filepath.Join(componentDir, "forms", "formatCard.js"), `export default function formatCard() { return true; }`)
	writeCompileFixtureFile(t, filepath.Join(componentDir, "forms", "cash.html"), `<template><section>Cash</section></template>`)
	writeCompileFixtureFile(t, filepath.Join(componentDir, "paymentForm.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><isExposed>true</isExposed></LightningComponentBundle>`)

	p, err := project.Load(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "dist")
	_, err = Compile(p, Options{OutDir: outDir, Namespace: "c", RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"c/paymentForm/paymentForm.js",
		"c/paymentForm/paymentForm.html.js",
		"c/paymentForm/payform.js",
		"c/paymentForm/forms/card.html.js",
		"c/paymentForm/forms/card.css.js",
		"c/paymentForm/forms/card.scoped.css.js",
		"c/paymentForm/forms/formatCard.js",
		"c/paymentForm/forms/cash.html.js",
		"c/paymentForm/forms/cash.css.js",
		"c/paymentForm/forms/cash.scoped.css.js",
	} {
		if _, err := os.Stat(filepath.Join(outDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("missing compiled template file %s: %v", rel, err)
		}
	}
	helperBytes, err := os.ReadFile(filepath.Join(outDir, "c", "paymentForm", "payform.js"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(helperBytes), "payform.html.js") {
		t.Fatalf("plain helper was compiled as a component:\n%s", string(helperBytes))
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
