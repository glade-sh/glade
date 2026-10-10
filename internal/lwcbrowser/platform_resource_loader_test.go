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

// Resource-loader browser contract, initially observed at API 67.
// https://developer.salesforce.com/docs/platform/lightning-component-reference/guide/lightning-platform-resource-loader.html
// native captured rows back pending/settled promise identity and owned-load reuse.
// This module-level regression does not establish universal deduplication or
// version/Salesforce parity.
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
const baseURI = "http://localhost/";
const url = new URL("/resource/fixture/pending." + (method === "loadScript" ? "js" : "css"), baseURI).href;
const elements = [];
const errorListeners = [];
globalThis.window = {
  addEventListener(type, listener, capture) {
    assert.equal(type, "error");
    assert.equal(capture, true);
    errorListeners.push(listener);
  },
};
globalThis.document = {
  baseURI,
  querySelectorAll(selector) {
    assert.ok(selector === "script" || selector === "link", "unexpected resource selector: " + selector);
    return elements.filter(el => el.tagName === selector);
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
  // native dom_failure_{loadScript,loadStyle}_{missing,retry_failure}, API59/67.
  assert.equal(observation.error.message, "lightning/platformResourceLoader encountered an error loading '" + url + "'.");
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
const first = loader[method](self, url);
const second = loader[method](self, url);
assert.equal(first, second, "native pending loads share the promise");
const observations = [observe(first), observe(second)];
await nextTurn();
assert.deepEqual(observations.map(o => o.state), ["pending", "pending"], method + " must not fulfill before load");

const ownedElements = () => elements.filter(el => el.tagName === tag && el !== external);
assert.ok(ownedElements().length > 0, "owned resource was not appended");
// Native syntax/throws scripts hide execution details. The boundary must
// leave neighbouring application errors and DOM resource failures untouched.
assert.equal(errorListeners.length, method === "loadScript" ? 1 : 0);
if (method === "loadScript") {
  const messages = [];
  const previousConsoleError = console.error;
  console.error = (...args) => messages.push(args);
  try {
    const executionError = filename => ({
      filename, prevented: false, stopped: false,
      preventDefault() { this.prevented = true; },
      stopImmediatePropagation() { this.stopped = true; },
    });
    const unrelated = executionError(url + ".unrelated");
    errorListeners[0](unrelated);
    assert.equal(unrelated.prevented, false);
    assert.equal(unrelated.stopped, false);
    const domError = executionError(undefined);
    errorListeners[0](domError);
    assert.equal(domError.prevented, false);
    const owned = executionError(url);
    errorListeners[0](owned);
    assert.equal(owned.prevented, true);
    assert.equal(owned.stopped, true);
    assert.deepEqual(messages, [["Script error.", null]]);
  } finally {
    console.error = previousConsoleError;
  }
}
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
const originalOwnedElements = ownedElements();
for (const el of originalOwnedElements) {
  assert.equal(typeof el[event], "function");
  el[event]();
  if (scenario === "error" && method === "loadScript") {
    // Captured missing/retry_failure and the six failing script-subpath rows
    // request this unavailable blob once, preserving the original error URL.
    assert.equal(el.src, url, "the consumed script retains its original URL");
    const unavailableScripts = ownedElements().filter(el => !originalOwnedElements.includes(el));
    assert.equal(unavailableScripts.length, 1, "the captured blob request needs a fresh script");
    const unavailableScript = unavailableScripts[0];
    assert.ok(unavailableScript.src.startsWith("blob:"));
    const unavailableSource = new URL(unavailableScript.src.slice("blob:".length));
    assert.equal(unavailableSource.protocol, "http:");
    assert.equal(unavailableSource.pathname, "/not-found");
    assert.equal(unavailableSource.host, new URL(baseURI).host);
    await nextTurn();
    // Rejection does not depend on a later event from the unavailable blob.
    assert.deepEqual(observations.map(o => o.state), ["rejected", "rejected"]);
  } else {
    assert.equal(el[attr], url, "successful scripts and styles keep their source");
  }
}
const values = await Promise.all(observations.map(o => o.done));
if (scenario === "error") {
  observations.forEach(assertRejected);
  const previousElements = new Set(elements);
  const laterPromise = loader[method](self, url);
  assert.notEqual(laterPromise, first, "native settled failures return a new promise");
  const later = observe(laterPromise);
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
  assert.deepEqual(new Set(elements), previousElements, "cached failures do not repeat HTTP or blob loads");
} else {
  assert.deepEqual(values, [undefined, undefined]);
  assert.deepEqual(observations.map(o => o.state), ["fulfilled", "fulfilled"]);
  const laterPromise = loader[method](self, url);
  assert.notEqual(laterPromise, first, "native settled successes return a new promise");
  const later = observe(laterPromise);
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
