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

func TestLightningConfirmPromptContractAPI67(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		const installedNode = "/usr/local/bin/node"
		info, statErr := os.Stat(installedNode)
		if statErr != nil {
			t.Fatalf("Node is required for served dialog modules: PATH lookup: %v; fallback %s: %v", err, installedNode, statErr)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			t.Fatalf("Node fallback %s is not an executable regular file", installedNode)
		}
		node = installedNode
	}

	org := storage.NewOrgState()
	handler := New(&org)
	for _, kind := range []string{"confirm", "prompt"} {
		t.Run(kind, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lightning/shims/lightning/"+kind+".js", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("served %s module status = %d: %s", kind, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
				t.Fatalf("served %s module content type = %q", kind, rec.Header().Get("Content-Type"))
			}
			input := feedbackRuntimeInput(t, handler, kind, rec.Body.String())
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, node, "--experimental-vm-modules", "--input-type=module", "-e", confirmPromptRuntimeHarness)
			cmd.Stdin = bytes.NewReader(input)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			output, err := cmd.Output()
			if err != nil {
				t.Fatalf("served %s runtime failed: %v\nstdout: %s\nstderr: %s", kind, err, output, stderr.String())
			}
			if strings.TrimSpace(string(output)) != kind+"-dismissed" {
				t.Fatalf("unexpected %s runtime output: %q", kind, output)
			}
		})
	}
}

const confirmPromptRuntimeHarness = `
import assert from "node:assert/strict";
import vm from "node:vm";

let input = "";
for await (const chunk of process.stdin) input += chunk;
const { kind, source, overlaySource } = JSON.parse(input);

class Element {
  constructor(tagName) {
    this.tagName = tagName.toUpperCase();
    this.children = [];
    this.attributes = new Map();
    this.listeners = new Map();
    this.style = {};
    this.textContent = "";
    this.value = "";
  }
  appendChild(child) {
    child.parentElement = this;
    this.children.push(child);
    if (child.render) child.render();
    return child;
  }
  append(...children) { for (const child of children) this.appendChild(child); }
  attachShadow() {
    const root = new Element("#shadow-root");
    root.host = this;
    this.shadowRoot = root;
    return root;
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
  get isConnected() {
    let node = this;
    while (node) {
      if (node === document.body) return true;
      node = node.parentElement ?? node.host ?? null;
    }
    return false;
  }
  remove() {
    if (this.parentElement) {
      this.parentElement.children = this.parentElement.children.filter((child) => child !== this);
      this.parentElement = null;
    }
  }
  querySelector(selector) {
    return feedbackElements(this, selector)[0] ?? null;
  }
  querySelectorAll(selector) { return feedbackElements(this, selector); }
}

const document = {
  body: new Element("body"),
  createElement: (tagName) => new Element(tagName),
  querySelector(selector) { return this.body.querySelector(selector); },
};
const window = { dispatchEvent() { return true; } };
class CustomEvent { constructor(type, options = {}) { this.type = type; this.detail = options.detail; } }
const context = vm.createContext({ document, window, CustomEvent, Promise, setTimeout, clearTimeout });
` + feedbackOverlayRuntimeHarness + `
const module = new vm.SourceTextModule(source, { context });
await module.link((specifier) => {
  if (specifier === "lwc") return feedbackLWCModule;
  if (specifier === "/lightning/runtime/shell/overlay.js") return feedbackOverlayModule;
  if (specifier === "@glade/shell/diagnostics") {
    return new vm.SyntheticModule(["reportDiagnostic"], function () {
      this.setExport("reportDiagnostic", () => {});
    }, { context });
  }
  throw new Error("unexpected served " + kind + " import: " + specifier);
});
await module.evaluate();

const Dialog = module.namespace.default;
const opener = document.createElement("button");
document.body.appendChild(opener);
opener.focus();
function activeDialog() {
  return document.querySelector("section");
}
async function openPending(options) {
  const result = Dialog.open(options);
  assert.equal(typeof result?.then, "function", kind + ".open() must return a Promise");
  let settled = false;
  result.then(() => { settled = true; });
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(settled, false, kind + ".open() settled before a choice");
  const dialog = activeDialog();
  assert.ok(dialog, kind + ".open() must present a usable dialog");
  assert.equal(dialog.getAttribute("role"), kind === "prompt" ? "dialog" : "alertdialog");
  const heading = dialog.querySelector("h2");
  assert.ok(heading, kind + " dialog needs a heading");
  assert.equal(heading.getAttribute("tabindex"), "-1");
  assert.equal(feedbackOverlayModule.namespace.focusedControl(), heading, "initial focus must be on the dialog heading");
  return { result, dialog };
}
function button(dialog, label) {
  const choice = dialog.querySelectorAll("button").find((item) => item.textContent === label);
  assert.ok(choice, kind + " dialog needs a " + label + " button");
  return choice;
}
async function settle(result, label) {
  let timer;
  try {
    const value = await Promise.race([
      result,
      new Promise((_, reject) => {
        timer = setTimeout(() => reject(new Error(kind + " " + label + " did not settle")), 1000);
      }),
    ]);
    // Observe removal at the native post-settlement checkpoint.
    await new Promise(resolve => setTimeout(resolve, 800));
    return value;
  } finally {
    clearTimeout(timer);
  }
}

if (kind === "confirm") {
  const yes = await openPending({ label: "Confirm", message: "Proceed?" });
  button(yes.dialog, "OK").click();
  assert.equal(await settle(yes.result, "OK"), true);
  assert.equal(activeDialog(), null, "OK must remove confirm dialog");
  assert.equal(document.activeElement, opener, "OK must restore opener focus");
  const no = await openPending({ label: "Confirm", message: "Proceed?" });
  button(no.dialog, "Cancel").click();
  assert.equal(await settle(no.result, "Cancel"), false);
  assert.equal(activeDialog(), null, "Cancel must remove confirm dialog");
  assert.equal(document.activeElement, opener, "Cancel must restore opener focus");
} else if (kind === "prompt") {
  const yes = await openPending({ label: "Prompt", message: "Enter text", defaultValue: "seed" });
  const input = yes.dialog.querySelector("input");
  assert.ok(input, "prompt dialog needs an editable input");
  assert.equal(input.value, "seed", "defaultValue must prefill the input");
  input.value = "typed";
  input.dispatchEvent({ type: "input", target: input });
  input.dispatchEvent({ type: "change", target: input });
  button(yes.dialog, "OK").click();
  assert.equal(await settle(yes.result, "OK"), "typed");
  assert.equal(activeDialog(), null, "OK must remove prompt dialog");
  assert.equal(document.activeElement, opener, "OK must restore opener focus");
  const no = await openPending({ label: "Prompt", message: "Enter text", defaultValue: "seed" });
  button(no.dialog, "Cancel").click();
  assert.equal(await settle(no.result, "Cancel"), null);
  assert.equal(activeDialog(), null, "Cancel must remove prompt dialog");
  assert.equal(document.activeElement, opener, "Cancel must restore opener focus");
} else {
  throw new Error("unexpected dialog kind: " + kind);
}
process.stdout.write(kind + "-dismissed\n");
`
