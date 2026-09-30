package lwcbrowser

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TASK-10.9: required browser-component contract51, initial API67 qualification.
// lwc-contract-sha256-a87c0a2cb19c8685e2ddf612ee181ccf77bee92d5061aa4e55336ec6a234cc6a
// https://developer.salesforce.com/docs/platform/lightning-component-reference/guide/lightning-platform-resource-loader.html
// Each promise resolves after loading. This module-level regression does not
// assert promise identity, universal deduplication, or version/Salesforce parity.
func TestPlatformResourceLoaderWaitsForPendingResource(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	for _, method := range []string{"loadScript", "loadStyle"} {
		for _, scenario := range []string{"load", "error", "external"} {
			t.Run(method+"/"+scenario, func(t *testing.T) {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "platformResourceLoader.mjs"), []byte(PlatformResourceLoaderModuleJS()), 0o644); err != nil {
					t.Fatal(err)
				}
				script := `import assert from "node:assert/strict";
import * as loader from "./platformResourceLoader.mjs";

const method = process.argv[2];
const scenario = process.argv[3];
assert.ok(method === "loadScript" || method === "loadStyle");
assert.ok(["load", "error", "external"].includes(scenario));
const tag = method === "loadScript" ? "script" : "link";
const attr = method === "loadScript" ? "src" : "href";
const url = "/resource/task109/pending." + (method === "loadScript" ? "js" : "css");
const elements = [];
globalThis.document = {
  querySelectorAll(selector) {
    const match = /^(script|link)\[(src|href)="([^"]+)"\]$/.exec(selector);
    assert.ok(match, "unexpected resource selector: " + selector);
    return elements.filter(el => el.tagName === match[1] && el[match[2]] === match[3]);
  },
  querySelector(selector) { return this.querySelectorAll(selector)[0] || null; },
  createElement(tagName) { return { tagName }; },
  head: { appendChild(el) { elements.push(el); } },
};

// A complete turn drains promise chains of arbitrary microtask depth.
const nextTurn = () => new Promise(resolve => setImmediate(resolve));
function observe(promise) {
  const observation = { state: "pending", error: undefined };
  observation.done = promise.then(
    value => { observation.state = "fulfilled"; return value; },
    error => { observation.state = "rejected"; observation.error = error; },
  );
  return observation;
}
function assertRejected(observation) {
  assert.equal(observation.state, "rejected");
  assert.ok(observation.error instanceof Error);
  assert.equal(observation.error.message, "failed to load resource: " + url);
}

let external;
let externalLoads = 0;
const externalOnLoad = () => { externalLoads++; };
const externalOnError = () => {};
if (scenario === "external") {
  external = document.createElement(tag);
  external[attr] = url;
  if (tag === "link") external.rel = "stylesheet";
  external.onload = externalOnLoad;
  external.onerror = externalOnError;
  document.head.appendChild(external);
}

const self = {};
const observations = [observe(loader[method](self, url)), observe(loader[method](self, url))];
await nextTurn();
assert.deepEqual(observations.map(o => o.state), ["pending", "pending"], method + " must not fulfill before load");

const ownedElements = () => elements.filter(el => el[attr] === url && el !== external);
assert.ok(ownedElements().length > 0, "owned resource was not appended");
if (external) {
  assert.equal(external.onload, externalOnLoad);
  assert.equal(external.onerror, externalOnError);
  external.onload();
  await nextTurn();
  assert.equal(externalLoads, 1);
  assert.deepEqual(observations.map(o => o.state), ["pending", "pending"], "external load must not complete the owned load");
}

// Settle every owned resource without requiring a particular element count.
const event = scenario === "error" ? "onerror" : "onload";
for (const el of ownedElements()) {
  assert.equal(typeof el[event], "function");
  el[event]();
}
const values = await Promise.all(observations.map(o => o.done));
if (scenario === "error") {
  observations.forEach(assertRejected);
  const previousElements = new Set(elements);
  const later = observe(loader[method](self, url));
  await nextTurn();
  assert.notEqual(later.state, "fulfilled", "a previously failed resource must not falsely succeed");
  // A fresh retry is also valid, but its own error must reject rather than fulfill.
  if (later.state === "pending") {
    const retries = ownedElements().filter(el => !previousElements.has(el));
    assert.ok(retries.length > 0, "pending retry must start a fresh load");
    for (const el of retries) el.onerror();
  }
  await later.done;
  assertRejected(later);
} else {
  assert.deepEqual(values, [undefined, undefined]);
  assert.deepEqual(observations.map(o => o.state), ["fulfilled", "fulfilled"]);
  const later = observe(loader[method](self, url));
  await nextTurn();
  assert.equal(later.state, "fulfilled", "a successful owned load must remain reusable");
  assert.equal(await later.done, undefined);
}
console.log("resource-loader pending/load: " + method + "/" + scenario + " ok");
`
				if err := os.WriteFile(filepath.Join(dir, "test.mjs"), []byte(script), 0o644); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, node, "test.mjs", method, scenario)
				cmd.Dir = dir
				output, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("node %s resource-loader test failed: %v\n%s", method, err, output)
				}
				if !strings.Contains(string(output), "resource-loader pending/load: "+method+"/"+scenario+" ok") {
					t.Fatalf("node %s resource-loader test did not finish: %s", method, output)
				}
			})
		}
	}
}
