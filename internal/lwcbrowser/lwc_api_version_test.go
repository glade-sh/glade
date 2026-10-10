package lwcbrowser

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/project"
)

func TestLWCAPI67RegistrationRunsInBrowser(t *testing.T) {
	if os.Getenv("GLADE_LWC_BROWSER") == "" {
		t.Skip("set GLADE_LWC_BROWSER=1 to run the focused API 67 browser integration")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatalf("GLADE_LWC_BROWSER is enabled but node is unavailable: %v", err)
	}
	serveOnly := os.Getenv("GLADE_LWC_BROWSER_SERVE_ONLY") == "1"
	playwrightPackage := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwrightPackage == "" {
		repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		playwrightPackage = filepath.Join(repoRoot, "lwcruntime", "node_modules", "playwright")
	}
	if !serveOnly {
		if !filepath.IsAbs(playwrightPackage) {
			t.Fatalf("GLADE_LWC_PLAYWRIGHT_MODULE must be an absolute package path, got %q", playwrightPackage)
		}
		playwrightInfo, err := os.Stat(playwrightPackage)
		if err != nil {
			t.Fatalf("GLADE_LWC_BROWSER is enabled but Playwright module %q is unavailable: %v", playwrightPackage, err)
		}
		if !playwrightInfo.IsDir() {
			t.Fatalf("GLADE_LWC_PLAYWRIGHT_MODULE must name the Playwright package directory, got %q", playwrightPackage)
		}
	}
	browserExecutable := os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")
	if !serveOnly && browserExecutable != "" {
		if !filepath.IsAbs(browserExecutable) {
			t.Fatalf("GLADE_LWC_BROWSER_EXECUTABLE must be an absolute path, got %q", browserExecutable)
		}
		info, err := os.Stat(browserExecutable)
		if err != nil {
			t.Fatalf("configured browser executable %q is unavailable: %v", browserExecutable, err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("configured browser executable is not an executable file: %q", browserExecutable)
		}
	}
	browserAddr := os.Getenv("GLADE_LWC_BROWSER_ADDR")
	if browserAddr != "" && browserAddr != "127.0.0.1:8942" {
		t.Fatalf("GLADE_LWC_BROWSER_ADDR only permits 127.0.0.1:8942, got %q", browserAddr)
	}
	toolchainRoot, err := gladehome.Root()
	if err != nil {
		t.Fatalf("GLADE_LWC_BROWSER is enabled but no Go-selected LWC toolchain is available: %v", err)
	}
	t.Setenv("GLADE_HOME", toolchainRoot)
	enginePath := filepath.Join(toolchainRoot, "third_party", "lwc", "node_modules", "@lwc", "engine-dom", "dist", "index.js")
	syntheticShadowPath := filepath.Join(toolchainRoot, "third_party", "lwc", "node_modules", "@lwc", "synthetic-shadow", "dist", "index.js")
	for _, file := range []string{enginePath, syntheticShadowPath} {
		if _, err := os.Stat(file); err != nil {
			t.Fatalf("GLADE_LWC_BROWSER is enabled but selected Go LWC runtime asset is missing at %s: %v", file, err)
		}
	}

	projectRoot := t.TempDir()
	api67Dir := filepath.Join(projectRoot, "force-app", "main", "default", "lwc", "gladeApi67Oracle")
	writeLWCAPI67FixtureFile(t, filepath.Join(api67Dir, "gladeApi67Oracle.js"), `import { LightningElement } from 'lwc';

export default class GladeApi67Oracle extends LightningElement {
    count = 0;

    increment() {
        this.count += 1;
    }
}
`)
	writeLWCAPI67FixtureFile(t, filepath.Join(api67Dir, "gladeApi67Oracle.html"), `<template>
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
	writeLWCAPI67FixtureFile(t, filepath.Join(api67Dir, "gladeApi67Oracle.css"), `.style-probe {
    color: rgb(17, 34, 51);
    font-weight: 700;
}
`)
	writeLWCAPI67FixtureFile(t, filepath.Join(api67Dir, "gladeApi67Oracle.js-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <apiVersion>67.0</apiVersion>
  <isExposed>true</isExposed>
  <targets><target>lightning__UrlAddressable</target></targets>
</LightningComponentBundle>
`)

	controlDir := filepath.Join(projectRoot, "force-app", "main", "default", "lwc", "api66Control")
	writeLWCAPI67FixtureFile(t, filepath.Join(controlDir, "api66Control.js"), `import { LightningElement } from 'lwc';
export default class Api66Control extends LightningElement {}`)
	writeLWCAPI67FixtureFile(t, filepath.Join(controlDir, "api66Control.html"), `<template><p>API 66 lower control</p></template>`)
	writeLWCAPI67FixtureFile(t, filepath.Join(controlDir, "api66Control.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata">
  <apiVersion>66.0</apiVersion>
  <isExposed>false</isExposed>
</LightningComponentBundle>`)

	p, err := project.Load(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "compiled")
	compiled, err := compile.Compile(p, compile.Options{OutDir: outDir, Namespace: "c"})
	if err != nil {
		t.Fatal(err)
	}
	api67Entry, ok := compiled.Modules["c:gladeApi67Oracle"]
	if !ok {
		t.Fatalf("compiled modules missing c:gladeApi67Oracle: %#v", compiled.Modules)
	}
	controlEntry, ok := compiled.Modules["c:api66Control"]
	if !ok {
		t.Fatalf("compiled modules missing c:api66Control: %#v", compiled.Modules)
	}
	moduleURL := func(entry compile.ModuleEntry) string {
		rel, err := filepath.Rel(outDir, entry.File)
		if err != nil {
			t.Fatalf("module %s escapes compile output: %v", entry.File, err)
		}
		return "/lightning/modules/" + filepath.ToSlash(rel)
	}
	imports, err := json.Marshal(map[string]string{
		"lwc":                   "/lightning/vendor/lwc.js",
		"@lwc/synthetic-shadow": "/lightning/vendor/synthetic-shadow.js",
		"c/gladeApi67Oracle":    moduleURL(api67Entry),
		"c/api66Control":        moduleURL(controlEntry),
	})
	if err != nil {
		t.Fatal(err)
	}
	var browserListener net.Listener
	if browserAddr != "" {
		browserListener, err = net.Listen("tcp", browserAddr)
		if err != nil {
			t.Fatalf("listen on admitted browser address %s: %v", browserAddr, err)
		}
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><html><head>
<script>window.process = { env: { NODE_ENV: "production" } };</script>
<script type="importmap">{"imports":%s}</script>
</head><body><div id="host"></div><script type="module" src="/entry.js"></script></body></html>`, imports)
		case r.URL.Path == "/entry.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprint(w, `import "@lwc/synthetic-shadow";
import { createElement } from "lwc";
import Api67Oracle from "c/gladeApi67Oracle";
import Api66Control from "c/api66Control";
const host = document.getElementById("host");
host.appendChild(createElement("c-glade-api67-oracle", { is: Api67Oracle }));
host.appendChild(createElement("c-api66-control", { is: Api66Control }));
`)
		case r.URL.Path == "/lightning/vendor/lwc.js":
			serveTestFile(t, w, enginePath)
		case r.URL.Path == "/lightning/vendor/synthetic-shadow.js":
			serveTestFile(t, w, syntheticShadowPath)
		case strings.HasPrefix(r.URL.Path, "/lightning/modules/"):
			rel := strings.TrimPrefix(r.URL.Path, "/lightning/modules/")
			clean := filepath.Clean(filepath.FromSlash(rel))
			moduleFile := filepath.Join(outDir, clean)
			checked, err := filepath.Rel(outDir, moduleFile)
			if err != nil || checked == ".." || strings.HasPrefix(checked, ".."+string(filepath.Separator)) {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			// Match vendor delivery: read this tiny owned fixture rather than
			// relying on filesystem-backed HTTP transfer under the job sandbox.
			serveTestFile(t, w, moduleFile)
		case r.URL.Path == "/favicon.ico":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})
	// NewUnstartedServer creates an ephemeral listener immediately. Avoid that
	// constructor when the test was admitted for one fixed loopback address.
	var server *httptest.Server
	if browserListener != nil {
		server = &httptest.Server{Listener: browserListener, Config: &http.Server{Handler: handler}}
	} else {
		server = httptest.NewUnstartedServer(handler)
	}
	server.Start()
	defer server.Close()
	if serveOnly {
		if browserAddr != "127.0.0.1:8942" {
			t.Fatal("manual serve-only mode requires admitted fixed loopback address")
		}
		readyPath := os.Getenv("GLADE_LWC_BROWSER_READY_FILE")
		if readyPath == "" || !filepath.IsAbs(readyPath) {
			t.Fatal("manual serve-only mode requires an explicit absolute ready file")
		}
		registration, err := os.ReadFile(api67Entry.File)
		if err != nil || !strings.Contains(string(registration), "apiVersion: 67") {
			t.Fatalf("manual serve-only fixture must preserve exact API67 registration: %v", err)
		}
		ready, err := json.Marshal(map[string]string{
			"mode": "manual-only-not-automated-PASS", "url": server.URL,
			"listener": server.Listener.Addr().String(), "toolchainRoot": toolchainRoot,
			"enginePath": enginePath, "shadowPath": syntheticShadowPath,
			"moduleURL": moduleURL(api67Entry), "registrationPath": api67Entry.File,
		})
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(readyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := file.Write(ready)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatalf("manual ready write failed: %v / %v", writeErr, closeErr)
		}
		t.Log("manual-only server ready; coordinator stops after observation, no automated PASS")
		select {}
	}

	script := fmt.Sprintf(`
import assert from "node:assert/strict";
import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { chromium } = require(%q);
const launchOptions = { headless: true };
const browserExecutable = %q;
if (browserExecutable !== "") launchOptions.executablePath = browserExecutable;
const browser = await chromium.launch(launchOptions);
try {
  const page = await browser.newPage();
  const pageErrors = [];
  const consoleErrors = [];
  page.on("pageerror", (err) => pageErrors.push(err.message));
  page.on("console", (msg) => { if (msg.type() === "error") consoleErrors.push(msg.text()); });
  await page.goto(%q, { waitUntil: "networkidle" });
  const oracle = page.locator("c-glade-api67-oracle");
  await oracle.locator("[data-count]").waitFor();
  await page.getByText("API 66 lower control").waitFor();
  assert.equal((await oracle.locator("[data-count]").textContent()).trim(), "0");
  assert.equal((await oracle.locator("[data-complex]").textContent()).trim(), "1");
  const styles = await oracle.locator(".style-probe").evaluate((node) => {
    const style = getComputedStyle(node);
    return { color: style.color, fontWeight: style.fontWeight };
  });
  assert.deepEqual(styles, { color: "rgb(17, 34, 51)", fontWeight: "700" });
  const details = oracle.locator("details");
  assert.deepEqual(await details.evaluateAll((nodes) => nodes.map((node) => node.open)), [true, false]);
  await oracle.locator("[data-second] summary").click();
  assert.deepEqual(await details.evaluateAll((nodes) => nodes.map((node) => node.open)), [false, true]);
  await oracle.getByRole("button", { name: "Increment oracle" }).click();
  assert.equal((await oracle.locator("[data-count]").textContent()).trim(), "1");
  assert.equal((await oracle.locator("[data-complex]").textContent()).trim(), "2");
  assert.deepEqual(pageErrors, []);
  assert.deepEqual(consoleErrors, []);
} finally {
  await browser.close();
}
`, playwrightPackage, browserExecutable, server.URL)
	testDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(testDir, "test.mjs"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "test.mjs")
	cmd.Dir = testDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("API 67 component browser test failed: %v\n%s", err, output)
	}
}

func writeLWCAPI67FixtureFile(t *testing.T, file, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
