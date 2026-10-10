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

// TestL18TextareaPreservedEvents preserves the earlier textareaTemplateJS
// input/change bindings and generic handleChange contract. This is a regression
// check of prior local behavior, not an uncaptured Salesforce textarea answer.
// The adjacent input commit order is backed by native r_event_text_commit.
func TestL18TextareaPreservedEvents(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range []string{"input", "textarea"} {
		if err := os.WriteFile(filepath.Join(dir, name+".mjs"), []byte(LightningBaseComponentModuleJS(name)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "--input-type=module")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(l18TextareaPreservedEventsJS)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("L18 preserved textarea events: %v\n%s", err, output)
	}
}

// Execute the generated component's handlers with real EventTarget/CustomEvent
// objects. LWC registration/rendering and i18n imports are inert for this check.
const l18TextareaPreservedEventsJS = `import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const moduleURL = source => "data:text/javascript;base64," + Buffer.from(source).toString("base64");
const lwcURL = moduleURL(` + "`" + `
export class LightningElement extends EventTarget {}
export const registerDecorators = Ctor => Ctor;
export const registerTemplate = template => template;
export const freezeTemplate = () => {};
export const registerComponent = Ctor => Ctor;
` + "`" + `);
async function component(name) {
  const source = readFileSync(name + ".mjs", "utf8")
    .replaceAll('"lwc"', JSON.stringify(lwcURL))
    .replaceAll('"@salesforce/i18n/locale"', JSON.stringify(moduleURL('export default "en-US";')))
    .replaceAll('"@salesforce/i18n/timeZone"', JSON.stringify(moduleURL('export default "UTC";')));
  return (await import(moduleURL(source))).default;
}
const Input = await component("input");
const Textarea = await component("textarea");

function edit(Ctor, initial, value, types) {
  const control = new EventTarget();
  control.value = initial;
  const instance = new Ctor();
  instance.value = initial;
  const events = [];
  for (const type of ["change", "commit"]) {
    instance.addEventListener(type, event => events.push({
      type: event.type, detail: event.detail, bubbles: event.bubbles,
      composed: event.composed, cancelable: event.cancelable
    }));
  }
  control.addEventListener("focus", event => instance.handleFocus(event));
  control.addEventListener("input", event => instance.handleInput(event));
  control.addEventListener("change", event => instance.handleChange(event));
  control.dispatchEvent(new Event("focus"));
  control.value = value;
  for (const type of types) control.dispatchEvent(new Event(type));
  return { instance, events };
}
const change = detail => ({ type: "change", detail, bubbles: true, composed: true, cancelable: false });
for (const [name, initial, value, types] of [
  ["edited", "before", "after", ["input", "change"]],
  ["unchanged", "same", "same", ["input", "change"]],
  ["change only", "before", "after", ["change"]]
]) {
  const { instance, events } = edit(Textarea, initial, value, types);
  assert.equal(instance.value, value, name + " textarea value");
  assert.deepEqual(events, types.map(() => change({ value, checked: false })),
    name + " textarea preserves change payloads and emits no commit");
}

// Native r_event_text_commit backs input's change-before-commit ordering.
const input = edit(Input, "before", "after", ["input", "change"]);
assert.equal(input.instance.value, "after");
assert.deepEqual(input.events, [
  change({ value: "after" }),
  { type: "commit", detail: null, bubbles: false, composed: false, cancelable: false }
]);
`
