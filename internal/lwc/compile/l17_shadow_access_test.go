package compile

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestL17ShadowRootReadIsolation guards the JavaScript semantics around the
// private base-button boundary exercised by the native DOM rows. Plain
// registered objects isolate the read transform from browser host setup; the
// Salesforce conformance test separately observes actual base components.
func TestL17ShadowRootReadIsolation(t *testing.T) {
	requireLWCCompilerToolchain(t)
	roots, err := compileToolchainRoots()
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile(filepath.Join(roots.ScriptRoot, "lwcruntime", "src", "shims", "component-dom.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			results := compileConformanceBatch(t, api, 1, func(root string, _ int) {
				bundle := filepath.Join(root, "force-app", "main", "default", "lwc", "shadowReadProbe")
				writeCompileFixtureFile(t, filepath.Join(bundle, "shadowReadProbe.js"), l17ShadowReadSource)
				writeCompileFixtureFile(t, filepath.Join(bundle, "shadowReadProbe.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
			})
			if results[0].Err != nil {
				t.Fatal(results[0].Err)
			}
			entry, ok := results[0].Manifest.Modules["c:shadowReadProbe"]
			if !ok {
				t.Fatalf("missing compiled utility: %#v", results[0].Manifest.Modules)
			}
			compiled, err := os.ReadFile(entry.File)
			if err != nil {
				t.Fatal(err)
			}
			const helperURL = "/lightning/runtime/shims/component-dom.mjs"
			if !strings.Contains(string(compiled), helperURL) {
				t.Fatal("compiled shadow-root reads do not import the production boundary helper")
			}
			root := t.TempDir()
			// Only relocate the production import for Node; execute the emitted
			// utility and production helper without changing their contents.
			writeCompileFixtureFile(t, filepath.Join(root, "probe.mjs"), strings.ReplaceAll(string(compiled), helperURL, "./component-dom.mjs"))
			writeCompileFixtureFile(t, filepath.Join(root, "component-dom.mjs"), string(helper))
			writeCompileFixtureFile(t, filepath.Join(root, "observe.mjs"), l17ShadowReadObserver)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "node", "observe.mjs")
			cmd.Dir = root
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("compiled shadow-root access semantics: %v\n%s", err, output)
			}
		})
	}
}

const l17ShadowReadSource = `
export function read(receiver) { return receiver.shadowRoot; }
export function readComputed(receiver) { return receiver["shadowRoot"]; }
export function readOptional(receiver) { return receiver?.shadowRoot; }
export function readOptionalComputed(receiver) { return receiver?.["shadowRoot"]; }
export function readOptionalNested(receiver) { return receiver?.shadowRoot.child; }
export function readInheritedOptional(receiver) { return receiver?.child.shadowRoot; }
export function readInheritedOptionalCall(receiver) { return receiver?.child().shadowRoot; }
export function readOnce(factory) { return factory().shadowRoot; }
export function readOptionalOnce(factory) { return factory()?.shadowRoot; }
export function call(receiver) { return receiver.shadowRoot(); }
export function callOptionalReceiver(receiver) { return receiver?.shadowRoot(); }
export function callOptionalMethod(receiver) { return receiver.shadowRoot?.(); }
export function write(receiver, value) { return receiver.shadowRoot = value; }
export function writeComputed(receiver, value) { return receiver["shadowRoot"] = value; }
export function writeArrayPattern(receiver, value) { [receiver.shadowRoot] = [value]; }
export function writeObjectPattern(receiver, value) { ({ value: receiver.shadowRoot } = { value }); }
export function add(receiver) { return receiver.shadowRoot += 2; }
export function increment(receiver) { return receiver.shadowRoot++; }
export function remove(receiver) { return delete receiver.shadowRoot; }
export function removeOptional(receiver) { return delete receiver?.shadowRoot; }
export function readOther(receiver) { return receiver.shadowroot; }
export function readDynamic(receiver, key) { return receiver[key]; }
export function readSuper(Base) {
    class Derived extends Base { read() { return super.shadowRoot; } }
    return new Derived().read();
}
`

const l17ShadowReadObserver = `
import assert from 'node:assert/strict';
import * as probe from './probe.mjs';
import { registerPrivateComponentShadow } from './component-dom.mjs';

const exposed = { child: 'visible' };
const ordinary = { shadowRoot: exposed, shadowroot: 'other' };
for (const read of [probe.read, probe.readComputed, probe.readOptional, probe.readOptionalComputed]) {
    assert.strictEqual(read(ordinary), exposed);
    assert.strictEqual(read({ tagName: 'LIGHTNING-BUTTON', shadowRoot: exposed }), exposed);
}
const privateHost = { shadowRoot: exposed, shadowroot: 'other' };
registerPrivateComponentShadow(privateHost);
for (const read of [probe.read, probe.readComputed, probe.readOptional, probe.readOptionalComputed]) {
    assert.strictEqual(read(privateHost), null);
}
assert.strictEqual(privateHost.shadowRoot, exposed);
assert.strictEqual(probe.readOther(privateHost), 'other');
assert.strictEqual(probe.readDynamic(privateHost, 'shadowroot'), 'other');
assert.strictEqual(probe.readDynamic(privateHost, 'shadowRoot'), exposed);

let receiverCalls = 0;
const receiver = () => { receiverCalls++; return ordinary; };
assert.strictEqual(probe.readOnce(receiver), exposed);
assert.strictEqual(receiverCalls, 1);
assert.strictEqual(probe.readOptionalOnce(receiver), exposed);
assert.strictEqual(receiverCalls, 2);
assert.strictEqual(probe.readOnce(() => { receiverCalls++; return privateHost; }), null);
assert.strictEqual(receiverCalls, 3);

let getterCalls = 0;
const accessor = {
    get shadowRoot() {
        assert.strictEqual(this, accessor);
        getterCalls++;
        return exposed;
    }
};
assert.strictEqual(probe.read(accessor), exposed);
assert.strictEqual(getterCalls, 1);
assert.strictEqual(probe.readComputed(accessor), exposed);
assert.strictEqual(getterCalls, 2);
const guardedGetter = { get shadowRoot() { throw new Error('private getter must not execute'); } };
registerPrivateComponentShadow(guardedGetter);
assert.strictEqual(probe.read(guardedGetter), null);

const methodReceiver = { shadowRoot() { return this; } };
assert.strictEqual(probe.call(methodReceiver), methodReceiver);
assert.strictEqual(probe.callOptionalReceiver(methodReceiver), methodReceiver);
assert.strictEqual(probe.callOptionalMethod(methodReceiver), methodReceiver);
for (const value of [null, undefined]) {
    assert.strictEqual(probe.readOptional(value), undefined);
    assert.strictEqual(probe.readOptionalComputed(value), undefined);
    assert.strictEqual(probe.readOptionalNested(value), undefined);
    assert.strictEqual(probe.readInheritedOptional(value), undefined);
    assert.strictEqual(probe.readInheritedOptionalCall(value), undefined);
    assert.strictEqual(probe.callOptionalReceiver(value), undefined);
    assert.strictEqual(probe.callOptionalMethod({ shadowRoot: value }), undefined);
    assert.strictEqual(probe.readOptionalOnce(() => { receiverCalls++; return value; }), undefined);
    assert.throws(() => probe.read(value), TypeError);
}
assert.strictEqual(receiverCalls, 5);
assert.strictEqual(probe.readOptionalNested(ordinary), 'visible');
assert.strictEqual(probe.readInheritedOptional({ child: ordinary }), exposed);
const chainReceiver = { child() { assert.strictEqual(this, chainReceiver); return ordinary; } };
assert.strictEqual(probe.readInheritedOptionalCall(chainReceiver), exposed);
assert.throws(() => probe.readInheritedOptional({ child: null }), TypeError);
assert.throws(() => probe.readInheritedOptionalCall({ child() { return null; } }), TypeError);
assert.throws(() => probe.readOptionalNested({ shadowRoot: null }), TypeError);
assert.throws(() => probe.readOptionalNested(privateHost), TypeError);
assert.strictEqual(probe.read('primitive'), undefined);

// Reads are isolated; native JavaScript assignment/update/delete semantics
// still target the original object even if it is registered as private.
const writable = { shadowRoot: 1 };
registerPrivateComponentShadow(writable);
probe.writeArrayPattern(writable, 11);
assert.strictEqual(writable.shadowRoot, 11);
probe.writeObjectPattern(writable, 13);
assert.strictEqual(writable.shadowRoot, 13);
assert.strictEqual(probe.write(writable, 3), 3);
assert.strictEqual(writable.shadowRoot, 3);
assert.strictEqual(probe.writeComputed(writable, 5), 5);
assert.strictEqual(writable.shadowRoot, 5);
assert.strictEqual(probe.add(writable), 7);
assert.strictEqual(writable.shadowRoot, 7);
assert.strictEqual(probe.increment(writable), 7);
assert.strictEqual(writable.shadowRoot, 8);
assert.strictEqual(probe.remove(writable), true);
assert.strictEqual(Object.hasOwn(writable, 'shadowRoot'), false);
writable.shadowRoot = 9;
assert.strictEqual(probe.removeOptional(writable), true);
assert.strictEqual(Object.hasOwn(writable, 'shadowRoot'), false);
assert.strictEqual(probe.removeOptional(null), true);
assert.strictEqual(probe.removeOptional(undefined), true);

class Base { get shadowRoot() { return this; } }
const inherited = probe.readSuper(Base);
assert.ok(inherited instanceof Base);
`
