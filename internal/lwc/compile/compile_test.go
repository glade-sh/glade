package compile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/project"
)

func TestBuildCompileConfigAPIVersionMatrix(t *testing.T) {
	for _, test := range []struct {
		projectVersion string
		bundleVersion  string
		want           int
		wantError      string
	}{
		{projectVersion: "65.0", bundleVersion: "59.0", want: 59},
		{projectVersion: "65.0", bundleVersion: "60.0", want: 60},
		{projectVersion: "65.0", bundleVersion: "61.0", want: 61},
		{projectVersion: "65.0", bundleVersion: "62.0", want: 62},
		{projectVersion: "65.0", bundleVersion: "63.0", want: 63},
		{projectVersion: "65.0", bundleVersion: "64.0", want: 64},
		{projectVersion: "65.0", bundleVersion: "65.0", want: 65},
		{projectVersion: "65.0", bundleVersion: "66.0", want: 66},
		{projectVersion: "67.0", bundleVersion: "59.0", want: 59},
		{projectVersion: "67.0", bundleVersion: "60.0", want: 60},
		{projectVersion: "67.0", bundleVersion: "61.0", want: 61},
		{projectVersion: "67.0", bundleVersion: "62.0", want: 62},
		{projectVersion: "67.0", bundleVersion: "63.0", want: 63},
		{projectVersion: "67.0", bundleVersion: "64.0", want: 64},
		{projectVersion: "67.0", bundleVersion: "65.0", want: 65},
		{projectVersion: "67.0", bundleVersion: "66.0", want: 66},
		{projectVersion: "67.0", bundleVersion: "67.0", want: 67},
		{projectVersion: "66.0", bundleVersion: "67.0", want: 67},
		{projectVersion: "67.0", bundleVersion: "58.0", wantError: "unsupported LWC source API version"},
		{projectVersion: "67.0", bundleVersion: "68.0", wantError: "unsupported LWC source API version"},
		{projectVersion: "67.0", bundleVersion: "", wantError: "missing component API version"},
	} {
		t.Run(test.projectVersion+"_bundle_"+test.bundleVersion, func(t *testing.T) {
			root := t.TempDir()
			bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "probe")
			js := filepath.Join(bundle, "probe.js")
			html := filepath.Join(bundle, "probe.html")
			meta := filepath.Join(bundle, "probe.js-meta.xml")
			writeCompileFixtureFile(t, js, "export default class Probe {}")
			writeCompileFixtureFile(t, html, "<template></template>")
			writeCompileFixtureFile(t, meta, `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+test.bundleVersion+`</apiVersion></LightningComponentBundle>`)
			p := project.Project{Root: root, SourceAPIVersion: test.projectVersion, LWCFiles: []string{js}, LWCHTMLFiles: []string{html}, LWCMetaFiles: []string{meta}}
			cfg, err := buildCompileConfig(p, root, filepath.Join(root, "dist"), "c")
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("buildCompileConfig error = %v", err)
				}
				return
			}
			if err != nil || cfg.LWCAPIVersions["force-app/main/default/lwc/probe"] != test.want {
				t.Fatalf("buildCompileConfig = %#v, %v; want API %d", cfg, err, test.want)
			}
		})
	}
}

func TestLWCModuleAvailabilityFollowsBundleAPIVersion(t *testing.T) {
	requireLWCCompilerToolchain(t)
	for _, test := range []struct {
		version   string
		wantError bool
	}{{"65.0", true}, {"66.0", false}, {"67.0", false}} {
		t.Run(test.version, func(t *testing.T) {
			root := t.TempDir()
			bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "probe")
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js"), `import api from 'experience/blockBuilderApi';
export default class Probe { async load() { return import('experience/blockBuilderApi'); } }`)
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.html"), "<template></template>")
			writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+test.version+`</apiVersion></LightningComponentBundle>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Compile(p, Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
			if test.wantError {
				want := `LWC module "experience/blockBuilderApi" requires API version 66.0 or later; bundle uses 65.0`
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("Compile error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestComplexTemplateExpressionsFollowBundleAPIVersion(t *testing.T) {
	requireLWCCompilerToolchain(t)
	for _, test := range []struct {
		version   string
		wantError bool
	}{{"65.0", true}, {"66.0", false}, {"67.0", false}} {
		t.Run(test.version, func(t *testing.T) {
			err := compileTemplateAtAPIVersion(t, test.version, `<template><p>{items.length > 0 ? 'yes' : 'no'}</p></template>`)
			if (err != nil) != test.wantError {
				t.Fatalf("Compile error = %v, wantError %t", err, test.wantError)
			}
		})
	}
}

func TestHTMLDetailsNameFollowsBundleAPIVersion(t *testing.T) {
	requireLWCCompilerToolchain(t)
	for _, test := range []struct {
		version   string
		wantError bool
	}{{"65.0", true}, {"66.0", true}, {"67.0", false}} {
		t.Run(test.version, func(t *testing.T) {
			err := compileTemplateAtAPIVersion(t, test.version, `<template><details name="group"><summary>One</summary></details></template>`)
			if (err != nil) != test.wantError {
				t.Fatalf("Compile error = %v, wantError %t", err, test.wantError)
			}
		})
	}
}

func TestCompilePreservesDeclaredAPI67(t *testing.T) {
	requireLWCCompilerToolchain(t)
	root := t.TempDir()
	bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "gladeApi67Oracle")
	writeCompileFixtureFile(t, filepath.Join(bundle, "gladeApi67Oracle.js"), `import { LightningElement } from 'lwc';

export default class GladeApi67Oracle extends LightningElement {
    count = 0;

    increment() {
        this.count += 1;
    }
}
`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "gladeApi67Oracle.html"), `<template>
    <section aria-label="Glade API 67 oracle">
        <output data-count>{count}</output>
        <output data-complex>{count + 1}</output>
        <button type="button" onclick={increment}>Increment oracle</button>
        <span class="style-probe" data-style>Styled oracle</span>
        <details name="glade-oracle-group" data-first open>
            <summary>First oracle details</summary>
            <p>First panel</p>
        </details>
        <details name="glade-oracle-group" data-second>
            <summary>Second oracle details</summary>
            <p>Second panel</p>
        </details>
    </section>
</template>
`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "gladeApi67Oracle.css"), `.style-probe {
    color: rgb(17, 34, 51);
    font-weight: 700;
}
`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "gladeApi67Oracle.js-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <apiVersion>67.0</apiVersion>
  <isExposed>true</isExposed>
  <targets><target>lightning__UrlAddressable</target></targets>
</LightningComponentBundle>
`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Compile(p, Options{OutDir: filepath.Join(t.TempDir(), "compiled"), Namespace: "c"})
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := manifest.Modules["c:gladeApi67Oracle"]
	if !ok {
		t.Fatalf("compiled modules missing c:gladeApi67Oracle: %#v", manifest.Modules)
	}
	js, err := os.ReadFile(entry.File)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), "apiVersion: 67") {
		t.Fatalf("component registration did not preserve exact API 67 metadata:\n%s", js)
	}
	if strings.Contains(string(js), "apiVersion: 66") {
		t.Fatalf("component registration silently lowered API 67 to 66:\n%s", js)
	}

	componentDir := filepath.Dir(entry.File)
	templateJS, err := os.ReadFile(filepath.Join(componentDir, "gladeApi67Oracle.html.js"))
	if err != nil {
		t.Fatal(err)
	}
	templateOutput := string(templateJS)
	for _, want := range []string{"count + 1", "Increment oracle", "style-probe", "First oracle details", "Second oracle details"} {
		if !strings.Contains(templateOutput, want) {
			t.Errorf("compiled template output missing %q from the API 67 oracle fixture:\n%s", want, templateOutput)
		}
	}
	styleJS, err := os.ReadFile(filepath.Join(componentDir, "gladeApi67Oracle.css.js"))
	if err != nil {
		t.Fatal(err)
	}
	styleOutput := string(styleJS)
	for _, want := range []string{".style-probe", "color", "17", "34", "51", "font-weight", "700"} {
		if !strings.Contains(styleOutput, want) {
			t.Errorf("compiled style output missing %q from the API 67 oracle fixture:\n%s", want, styleOutput)
		}
	}
}

func compileTemplateAtAPIVersion(t *testing.T, version, template string) error {
	t.Helper()
	requireLWCCompilerToolchain(t)
	root := t.TempDir()
	bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "probe")
	writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js"), `export default class Probe { items = []; }`)
	writeCompileFixtureFile(t, filepath.Join(bundle, "probe.html"), template)
	writeCompileFixtureFile(t, filepath.Join(bundle, "probe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+version+`</apiVersion></LightningComponentBundle>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Compile(p, Options{OutDir: filepath.Join(root, "dist"), Namespace: "c"})
	return err
}

func requireLWCCompilerToolchain(t *testing.T) {
	t.Helper()
	if os.Getenv("GLADE_LWC_COMPILE") != "" {
		return
	}
	root, err := gladehome.Root()
	if err != nil {
		t.Skipf("LWC Go-selected toolchain unavailable: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "third_party", "lwc", "node_modules", "@lwc", "compiler")); err != nil {
		t.Skipf("LWC compiler missing from Go-selected toolchain %s: %v", root, err)
	}
}

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
	writeCompileFixtureFile(t, filepath.Join(bundleDir, "widget.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><isExposed>true</isExposed></LightningComponentBundle>`)

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
	writeCompileFixtureFile(t, filepath.Join(utilityDir, "bUtils.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)

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
	writeCompileFixtureFile(t, filepath.Join(componentDir, "paymentForm.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><isExposed>true</isExposed></LightningComponentBundle>`)

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

func TestCompileTransformsCustomRenderComponentWithoutSameNameTemplate(t *testing.T) {
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
	fixtureDir := filepath.Join(t.TempDir(), "custom-render-fixture")
	componentDir := filepath.Join(fixtureDir, "force-app", "main", "default", "lwc", "tileLike")
	writeCompileFixtureFile(t, filepath.Join(componentDir, "tileLike.js"), `import { LightningElement, api } from 'lwc';
import standardTile from './standardTile.html';
export default class TileLike extends LightningElement {
  @api label = 'Local';
  render() { return standardTile; }
}`)
	writeCompileFixtureFile(t, filepath.Join(componentDir, "standardTile.html"), `<template><section>{label}</section></template>`)
	writeCompileFixtureFile(t, filepath.Join(componentDir, "standardTile.css"), `section { display: block; }`)
	writeCompileFixtureFile(t, filepath.Join(componentDir, "tileLike.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><isExposed>true</isExposed></LightningComponentBundle>`)

	p, err := project.Load(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "dist")
	manifest, err := Compile(p, Options{OutDir: outDir, Namespace: "c", RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := manifest.Modules["c:tileLike"]
	if !ok {
		t.Fatalf("manifest modules = %#v", manifest.Modules)
	}
	if entry.Tag != "c-tile-like" {
		t.Fatalf("tag = %q, want c-tile-like", entry.Tag)
	}
	for _, rel := range []string{
		"c/tileLike/tileLike.js",
		"c/tileLike/standardTile.html.js",
		"c/tileLike/standardTile.css.js",
		"c/tileLike/standardTile.scoped.css.js",
	} {
		if _, err := os.Stat(filepath.Join(outDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("missing compiled custom-render file %s: %v", rel, err)
		}
	}
	entryBytes, err := os.ReadFile(filepath.Join(outDir, "c", "tileLike", "tileLike.js"))
	if err != nil {
		t.Fatal(err)
	}
	entryJS := string(entryBytes)
	if strings.Contains(entryJS, "@api") {
		t.Fatalf("custom-render entry was not transformed:\n%s", entryJS)
	}
	if !strings.Contains(entryJS, `from "./standardTile.html.js"`) {
		t.Fatalf("custom-render template import was not rewritten:\n%s", entryJS)
	}
	if !strings.Contains(entryJS, "registerDecorators") {
		t.Fatalf("custom-render entry missing LWC decorator registration:\n%s", entryJS)
	}
}

func TestCompileEnablesLwcOnDirective(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	fixtureDir := filepath.Join(t.TempDir(), "lwc-on-fixture")
	componentDir := filepath.Join(fixtureDir, "force-app", "main", "default", "lwc", "dynamicOn")
	writeCompileFixtureFile(t, filepath.Join(componentDir, "dynamicOn.js"), `import { LightningElement } from 'lwc';
export default class DynamicOn extends LightningElement {
  handlers = { click: () => {} };
}`)
	writeCompileFixtureFile(t, filepath.Join(componentDir, "dynamicOn.html"), `<template><button lwc:on={handlers}>Dynamic</button></template>`)
	writeCompileFixtureFile(t, filepath.Join(componentDir, "dynamicOn.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><isExposed>true</isExposed></LightningComponentBundle>`)

	p, err := project.Load(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "dist")
	if _, err := Compile(p, Options{OutDir: outDir, Namespace: "c", RepoRoot: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "c", "dynamicOn", "dynamicOn.html.js")); err != nil {
		t.Fatalf("missing compiled lwc:on template: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataHome, "glade")); !os.IsNotExist(err) {
		t.Fatalf("compile wrote user-global toolchain: %v", err)
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
