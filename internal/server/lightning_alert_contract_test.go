package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

func TestLightningAlertOpenWaitsForDismissalAPI67(t *testing.T) {
	org := storage.NewOrgState()
	handler := New(&org)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/lightning/alert.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("served alert module status = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("served alert module content type = %q", rec.Header().Get("Content-Type"))
	}

	node, err := exec.LookPath("node")
	if err != nil {
		const installedNode = "/usr/local/bin/node"
		info, statErr := os.Stat(installedNode)
		if statErr != nil {
			t.Fatalf("Node is required to execute the served alert module: PATH lookup: %v; fallback %s: %v", err, installedNode, statErr)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			t.Fatalf("Node fallback %s is not an executable regular file", installedNode)
		}
		node = installedNode
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "--experimental-vm-modules", "--input-type=module", "-e", alertRuntimeHarness)
	cmd.Stdin = strings.NewReader(rec.Body.String())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("served alert runtime failed: %v\nstdout: %s\nstderr: %s", err, output, stderr.String())
	}
	if strings.TrimSpace(string(output)) != "alert-dismissed" {
		t.Fatalf("unexpected alert runtime output: %q", output)
	}
}

const alertRuntimeHarness = `
import assert from "node:assert/strict";
import vm from "node:vm";

let source = "";
for await (const chunk of process.stdin) source += chunk;

class Element {
  constructor(tagName) {
    this.tagName = tagName.toUpperCase();
    this.children = [];
    this.attributes = new Map();
    this.listeners = new Map();
    this.style = {};
    this.textContent = "";
  }
  appendChild(child) {
    child.parentElement = this;
    this.children.push(child);
    return child;
  }
  append(...children) { for (const child of children) this.appendChild(child); }
  attachShadow() {
    const root = new Element("#shadow-root");
    root.host = this;
    this.shadowRoot = root;
    return root;
  }
  get isConnected() {
    let node = this;
    while (node) {
      if (node === document.body) return true;
      node = node.parentElement ?? node.host ?? null;
    }
    return false;
  }
  setAttribute(name, value) { this.attributes.set(name, String(value)); }
  getAttribute(name) { return this.attributes.get(name) ?? null; }
  addEventListener(name, listener) {
    const listeners = this.listeners.get(name) ?? [];
    listeners.push(listener);
    this.listeners.set(name, listeners);
  }
  removeEventListener(name, listener) {
    this.listeners.set(name, (this.listeners.get(name) ?? []).filter((item) => item !== listener));
  }
  dispatchEvent(event) {
    event.target ??= this;
    for (const listener of this.listeners.get(event.type) ?? []) listener(event);
    if (!event.stopped && this.parentElement) this.parentElement.dispatchEvent(event);
    return !event.prevented;
  }
  click() { this.dispatchEvent({ type: "click", target: this }); }
  focus() {
    this.focusCount = (this.focusCount ?? 0) + 1;
    let focused = this;
    let parent = this.parentElement;
    while (parent) {
      if (parent.host) {
        parent.activeElement = focused;
        focused = parent.host;
        parent = focused.parentElement;
      } else {
        parent = parent.parentElement;
      }
    }
    document.activeElement = focused;
  }
  remove() {
    if (this.parentElement) {
      this.parentElement.children = this.parentElement.children.filter((child) => child !== this);
      this.parentElement = null;
    }
  }
  querySelector(selector) {
    for (const child of this.children) {
      if (selector === "button" && child.tagName === "BUTTON" ||
          selector === '[role="alertdialog"]' && child.getAttribute("role") === "alertdialog") return child;
      const nested = child.querySelector(selector);
      if (nested) return nested;
    }
    return null;
  }
}

const document = {
  body: new Element("body"),
  createElement: (tagName) => new Element(tagName),
  querySelector(selector) { return this.body.querySelector(selector); },
};
const window = { dispatchEvent() { return true; } };
class CustomEvent { constructor(type, options = {}) { this.type = type; this.detail = options.detail; } }
const context = vm.createContext({ document, window, CustomEvent, Promise });
const module = new vm.SourceTextModule(source, { context });
await module.link((specifier) => {
  if (specifier === "lwc") {
    return new vm.SyntheticModule(
      ["LightningElement", "registerDecorators", "registerTemplate", "freezeTemplate", "registerComponent"],
      function () {
        this.setExport("LightningElement", class LightningElement {});
        this.setExport("registerDecorators", () => {});
        this.setExport("registerTemplate", (template) => template);
        this.setExport("freezeTemplate", () => {});
        this.setExport("registerComponent", (component) => component);
      }, { context });
  }
  if (specifier === "@glade/shell/diagnostics") {
    return new vm.SyntheticModule(["reportDiagnostic"], function () {
      this.setExport("reportDiagnostic", () => {});
    }, { context });
  }
  throw new Error("unexpected served alert import: " + specifier);
});
await module.evaluate();

function keydown(target, key, shiftKey = false) {
  const event = {
    type: "keydown", key, shiftKey, prevented: false, stopped: false,
    preventDefault() { this.prevented = true; },
    stopPropagation() { this.stopped = true; },
  };
  target.dispatchEvent(event);
  return event;
}
async function settle(promise, label) {
  let timer;
  try {
    await Promise.race([
      promise,
      new Promise((_, reject) => {
        timer = setTimeout(() => reject(new Error(label + " did not settle the alert promise")), 1000);
      }),
    ]);
  } finally {
    clearTimeout(timer);
  }
}

let resolved = false;
const pending = module.namespace.default.open({ label: "Alert", message: "API 67 alert" })
  .then(() => { resolved = true; });
await Promise.resolve();
await Promise.resolve();
assert.equal(resolved, false, "open() resolved before the user dismissed the alert");
const dialog = document.querySelector('[role="alertdialog"]');
assert.ok(dialog, "open() must present a dismissible alert dialog");
const heading = dialog.children.find((child) => child.tagName === "H2");
assert.ok(heading, "alert dialog needs a heading");
assert.equal(heading.getAttribute("tabindex"), "-1");
assert.equal(document.activeElement, heading, "initial focus must be on the heading");
const ok = dialog.querySelector("button");
assert.ok(ok, "alert dialog needs a usable OK button");
assert.match(ok.textContent || ok.getAttribute("aria-label") || "", /OK/i, "dismissal button must identify OK");
const tab = keydown(heading, "Tab");
assert.equal(tab.prevented, true);
assert.equal(document.activeElement, ok);
heading.focus();
const shiftTab = keydown(heading, "Tab", true);
assert.equal(shiftTab.prevented, true);
assert.equal(document.activeElement, ok);
const wrapTab = keydown(ok, "Tab");
assert.equal(wrapTab.prevented, true);
assert.equal(document.activeElement, ok);
ok.click();
await settle(pending, "OK");
assert.equal(resolved, true);
assert.equal(document.querySelector('[role="alertdialog"]'), null, "dismissal must remove the dialog");

const outerHost = document.createElement("div");
document.body.appendChild(outerHost);
const outerRoot = outerHost.attachShadow();
const innerHost = document.createElement("div");
outerRoot.appendChild(innerHost);
const innerRoot = innerHost.attachShadow();
const opener = document.createElement("button");
innerRoot.appendChild(opener);
opener.focus();
assert.equal(document.activeElement, outerHost);
assert.equal(outerRoot.activeElement, innerHost);
assert.equal(innerRoot.activeElement, opener);
let leakedKeydowns = 0;
document.body.addEventListener("keydown", () => { leakedKeydowns++; });
let escapeSettles = 0;
const escapePending = module.namespace.default.open({ label: "Escape", message: "Dismiss me" })
  .then(() => { escapeSettles++; });
const escapeDialog = document.querySelector('[role="alertdialog"]');
assert.ok(escapeDialog);
const escapeHeading = escapeDialog.children.find((child) => child.tagName === "H2");
assert.equal(document.activeElement, escapeHeading);
const escape = keydown(escapeHeading, "Escape");
assert.equal(escape.prevented, true);
assert.equal(escape.stopped, true);
assert.equal(leakedKeydowns, 0, "Escape must not propagate beyond the dialog");
await settle(escapePending, "Escape");
assert.equal(escapeSettles, 1);
assert.equal(document.querySelector('[role="alertdialog"]'), null);
assert.equal(opener.focusCount, 2, "connected nested shadow opener must regain focus");
assert.equal(innerRoot.activeElement, opener);
assert.equal(outerRoot.activeElement, innerHost);
assert.equal(document.activeElement, outerHost);
keydown(escapeDialog, "Escape");
assert.equal(escapeSettles, 1, "repeat Escape must not settle twice");
assert.equal(opener.focusCount, 2, "repeat Escape must not restore focus twice");

const disconnectedPending = module.namespace.default.open({ label: "Disconnected", message: "Dismiss me" });
const disconnectedDialog = document.querySelector('[role="alertdialog"]');
opener.remove();
keydown(disconnectedDialog, "Escape");
await settle(disconnectedPending, "disconnected Escape");
assert.equal(opener.focusCount, 2, "disconnected opener must not be focused");
assert.equal(document.querySelector('[role="alertdialog"]'), null);
process.stdout.write("alert-dismissed\n");
`
