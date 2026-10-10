package lwcbrowser

import (
	"context"
	"encoding/json"
	"os/exec"
	"regexp"
	"strconv"
	"testing"
	"time"
)

// Public source pins retained for this behavior question:
// LWC-003-O0, LWC-003-O1, LWC-011-O0, and
// lwc-module-member|lightning%2Frefresh|event|refreshEvent.
// https://developer.salesforce.com/docs/platform/lwc/guide/reference-lightning-refreshevent-data-type
// https://developer.salesforce.com/docs/platform/lwc/guide/reference-lightning-refreshview-registerrefreshcontainer
// https://developer.salesforce.com/docs/platform/lwc/guide/reference-lightning-refreshview-registerrefreshhandler.html
// https://developer.salesforce.com/docs/platform/lwc/guide/reference-lightning-refreshstatus-constants.html
var refreshModuleTerminalStatus = regexp.MustCompile(`(?m)^LWC_REFRESH_ASSERTIONS=([0-9]+) STATUS=(PASS|FAIL|ERROR)$`)

// TestRefreshModuleEventAndRegistrationLifecycle executes the production JS
// module with Node's built-in EventTarget. The fixture covers same-element
// dispatch only; it does not model a browser DOM, shadow tree, or Salesforce's
// full refresh traversal. L16 native r_completion_constants and
// r_handles_container_reregister back the binding and lifecycle assertions.
func TestRefreshModuleEventAndRegistrationLifecycle(t *testing.T) {
	moduleJSON, err := json.Marshal(RefreshModuleJS())
	if err != nil {
		t.Fatalf("encode RefreshModuleJS source: %v", err)
	}

	script := `if (typeof globalThis.CustomEvent !== "function") {
  globalThis.CustomEvent = class CustomEvent extends Event {
    constructor(type, options = {}) {
      super(type, options);
      this.detail = options.detail;
    }
  };
}

const moduleSource = ` + string(moduleJSON) + `;
const failures = [];
let assertionCount = 0;
let harnessError = null;
function check(condition, message) {
  assertionCount++;
  if (!condition) failures.push(message);
}
const nextTurn = () => new Promise((resolve) => setImmediate(resolve));
function settleWithin(promise, milliseconds) {
  return new Promise((resolve) => {
    const timer = setTimeout(() => resolve({ settled: false }), milliseconds);
    Promise.resolve(promise).then(
      (value) => {
        clearTimeout(timer);
        resolve({ settled: true, value });
      },
      (error) => {
        clearTimeout(timer);
        resolve({ settled: true, error });
      }
    );
  });
}

try {
  const refresh = await import(
    "data:text/javascript;base64," + Buffer.from(moduleSource).toString("base64")
  );
  const target = new EventTarget();
  let containerCalls = 0;
  let handlerCalls = 0;
  let unrelatedListenerCalls = 0;
  let statusPromise;
  const event = new refresh.RefreshEvent();

  const original = refresh.registerRefreshContainer(target, (promise) => {
    containerCalls++;
    statusPromise = promise;
    check(event.cancelBubble, "the consuming container stops further propagation");
  });
  refresh.registerRefreshHandler(target, async () => {
    handlerCalls++;
    return true;
  });

  target.addEventListener(event.type, () => { unrelatedListenerCalls++; });
  target.dispatchEvent(event);
  await nextTurn();

  check(containerCalls === 1, "dispatchEvent reaches the registered container callback");
  check(handlerCalls === 1, "dispatchEvent reaches the registered handler");
  check(statusPromise instanceof Promise, "container callback receives a status Promise");
  check(refresh.RefreshComplete === undefined, "the captured named completion binding is undefined");
  check(unrelatedListenerCalls === 1, "other same-element listeners still receive the event");
  if (statusPromise instanceof Promise) {
    const completion = await settleWithin(statusPromise, 1000);
    check(completion.settled, "container status Promise settles");
    if (completion.settled) {
      check(!("error" in completion), "container status Promise fulfills");
      if (!("error" in completion)) {
        check(completion.value === 1,
          "a successful status Promise resolves to the captured numeric status");
      }
    }
  }

  let replacementCalls = 0;
  let replacementPromise;
  refresh.unregisterRefreshContainer(original);
  const replacement = refresh.registerRefreshContainer(target, (promise) => {
    replacementCalls++;
    replacementPromise = promise;
  });
  target.dispatchEvent(new refresh.RefreshEvent());
  await nextTurn();
  check(containerCalls === 1, "replacing a container removes the former listener");
  check(replacementCalls === 1, "replacement container receives one callback");
  check(handlerCalls === 2, "replacement dispatch refreshes the handler once");
  check(replacementPromise instanceof Promise, "replacement callback receives a status Promise");
  if (replacementPromise instanceof Promise) {
    const completion = await settleWithin(replacementPromise, 1000);
    check(completion.settled && !("error" in completion), "replacement status Promise fulfills");
    check(completion.value === 1, "replacement resolves to the captured completion status");
  }
  refresh.unregisterRefreshContainer(replacement);
  target.dispatchEvent(new refresh.RefreshEvent());
  await nextTurn();
  check(replacementCalls === 1, "unregister removes the container callback listener");
  check(handlerCalls === 2, "unregistered container does not start a refresh");
  check(unrelatedListenerCalls === 3, "unregister preserves unrelated event listeners");
} catch (error) {
  harnessError = error;
}

if (assertionCount === 0) {
  check(false, "no behavior assertions were reached");
  harnessError = harnessError || new Error("fixture did not reach behavior assertions");
}
const terminalStatus = harnessError ? "ERROR" : failures.length === 0 ? "PASS" : "FAIL";
console.log("LWC_REFRESH_ASSERTIONS=" + assertionCount + " STATUS=" + terminalStatus);
if (harnessError) console.error("HARNESS_ERROR=" + String(harnessError));
if (failures.length > 0) console.error(failures.join("\n"));
if (terminalStatus !== "PASS") process.exitCode = 1;
`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--input-type=module", "-e", script)
	output, err := cmd.CombinedOutput()

	match := refreshModuleTerminalStatus.FindSubmatch(output)
	if len(match) != 3 {
		t.Fatalf("required Node execution emitted no assertion count and terminal status (err=%v):\n%s", err, output)
	}
	assertionCount, parseErr := strconv.Atoi(string(match[1]))
	if parseErr != nil || assertionCount == 0 {
		t.Fatalf("required Node execution reported an empty/invalid assertion count: %q\n%s", match[1], output)
	}
	if string(match[2]) == "ERROR" {
		t.Fatalf("RefreshModuleJS fixture failed before a conclusive behavior result after %d checks (err=%v):\n%s", assertionCount, err, output)
	}
	if string(match[2]) != "PASS" || err != nil {
		t.Fatalf("RefreshModuleJS behavior failed after %d assertions (err=%v):\n%s", assertionCount, err, output)
	}
	t.Logf("RefreshModuleJS executed %d behavior assertions: %s", assertionCount, match[2])
}

// TestRefreshEventCancellation checks the documented default event flags and
// cancellation contract with Node's built-in event classes and the production
// RefreshModuleJS source. It does not model browser DOM or Salesforce dispatch.
func TestRefreshEventCancellation(t *testing.T) {
	moduleJSON, err := json.Marshal(RefreshModuleJS())
	if err != nil {
		t.Fatalf("encode RefreshModuleJS source: %v", err)
	}

	script := `if (typeof globalThis.EventTarget !== "function" ||
    typeof globalThis.Event !== "function" ||
    typeof globalThis.CustomEvent !== "function") {
  throw new Error("Node built-in EventTarget, Event, or CustomEvent is unavailable");
}

const moduleSource = ` + string(moduleJSON) + `;
const failures = [];
let assertionCount = 0;
let harnessError = null;
function check(condition, message) {
  assertionCount++;
  if (!condition) failures.push(message);
}

try {
  const refresh = await import(
    "data:text/javascript;base64," + Buffer.from(moduleSource).toString("base64")
  );
  const event = new refresh.RefreshEvent();
  check(event.type === "lightning__refresh", "RefreshEvent uses the documented refresh event type");
  check(event.bubbles === true, "RefreshEvent bubbles by default");
  check(event.composed === true, "RefreshEvent is composed by default");
  check(event.cancelable === true, "RefreshEvent is cancelable by default");

  const target = new EventTarget();
  target.addEventListener(event.type, (received) => received.preventDefault(), { once: true });
  const canceledDispatchResult = target.dispatchEvent(event);
  check(event.defaultPrevented === true, "preventDefault cancels RefreshEvent default behavior");
  check(canceledDispatchResult === false, "dispatchEvent reports a canceled RefreshEvent");

  const controlEvent = new refresh.RefreshEvent();
  const controlTarget = new EventTarget();
  const controlDispatchResult = controlTarget.dispatchEvent(controlEvent);
  check(controlEvent.defaultPrevented === false, "unhandled RefreshEvent remains uncanceled");
  check(controlDispatchResult === true, "independent uncanceled dispatch returns true");
} catch (error) {
  harnessError = error;
}

if (assertionCount === 0) {
  check(false, "no cancellation behavior assertions were reached");
  harnessError = harnessError || new Error("fixture did not reach behavior assertions");
}
const terminalStatus = harnessError ? "ERROR" : failures.length === 0 ? "PASS" : "FAIL";
console.log("LWC_REFRESH_ASSERTIONS=" + assertionCount + " STATUS=" + terminalStatus);
if (harnessError) console.error("HARNESS_ERROR=" + String(harnessError));
if (failures.length > 0) console.error(failures.join("\n"));
if (terminalStatus !== "PASS") process.exitCode = 1;
`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--input-type=module", "-e", script)
	output, err := cmd.CombinedOutput()

	match := refreshModuleTerminalStatus.FindSubmatch(output)
	if len(match) != 3 {
		t.Fatalf("required Node execution emitted no assertion count and terminal status (err=%v):\n%s", err, output)
	}
	assertionCount, parseErr := strconv.Atoi(string(match[1]))
	if parseErr != nil || assertionCount == 0 {
		t.Fatalf("required Node execution reported an empty/invalid assertion count: %q\n%s", match[1], output)
	}
	if string(match[2]) == "ERROR" {
		t.Fatalf("RefreshModuleJS cancellation fixture failed before a conclusive behavior result after %d checks (err=%v):\n%s", assertionCount, err, output)
	}
	if string(match[2]) != "PASS" || err != nil {
		t.Fatalf("RefreshModuleJS cancellation behavior failed after %d assertions (err=%v):\n%s", assertionCount, err, output)
	}
	t.Logf("RefreshModuleJS cancellation executed %d behavior assertions: %s", assertionCount, match[2])
}

// TestRefreshModuleRegisteredTreeTraversalAPI67 covers the LWS public module
// tree contract with a synthetic plain DOM. The official registerRefreshHandler
// and data-refreshview-api guides require parent completion before children
// and false descendant pruning. L16 r_tree_parent_{true,false}_preorder,
// r_tree_root_delayed and r_tree_child_first back the order and status edits.
// API67 is the source target; this is not browser, shadow, Locker, artifact
// compilation, or native Salesforce evidence.
func TestRefreshModuleRegisteredTreeTraversalAPI67(t *testing.T) {
	moduleJSON, err := json.Marshal(RefreshModuleJS())
	if err != nil {
		t.Fatalf("encode RefreshModuleJS source: %v", err)
	}
	script := `const moduleSource = ` + string(moduleJSON) + `;
const failures = [];
let assertionCount = 0;
let harnessError = null;
function check(condition, message) {
  assertionCount++;
  if (!condition) failures.push(message);
}
const nextTurn = () => new Promise((resolve) => setImmediate(resolve));
function settleWithin(promise, milliseconds) {
  return new Promise((resolve) => {
    const timer = setTimeout(() => resolve({ settled: false }), milliseconds);
    Promise.resolve(promise).then(
      (value) => { clearTimeout(timer); resolve({ settled: true, value }); },
      (error) => { clearTimeout(timer); resolve({ settled: true, error }); }
    );
  });
}

try {
  if (typeof EventTarget !== "function" || typeof CustomEvent !== "function") {
    throw new Error("Node built-in EventTarget or CustomEvent is unavailable");
  }
  class TreeElement extends EventTarget {
    constructor(name, parent = null) {
      super();
      this.name = name;
      this.parentElement = parent;
      this.children = [];
      if (parent) parent.children.push(this);
    }
    contains(element) {
      for (let current = element; current; current = current.parentElement) {
        if (current === this) return true;
      }
      return false;
    }
    compareDocumentPosition(other) {
      if (other === this) return 0;
      let root = this;
      while (root.parentElement) root = root.parentElement;
      if (!root.contains(other)) return 1;
      const order = [];
      function visit(element) {
        order.push(element);
        for (const child of element.children) visit(child);
      }
      visit(root);
      return order.indexOf(this) < order.indexOf(other) ? 4 : 2;
    }
  }
  const refresh = await import(
    "data:text/javascript;base64," + Buffer.from(moduleSource).toString("base64")
  );
  const root = new TreeElement("root");
  const a = new TreeElement("A", root);
  const b = new TreeElement("B", root);
  const a1 = new TreeElement("A1", a);
  const b1 = new TreeElement("B1", b);
  const outside = new TreeElement("outside");
  let calls = [];
  let allowA = false;
  let parentResolved = false;
  let releaseParent;
  let parentGate = new Promise((resolve) => { releaseParent = resolve; });
  let statusPromise;
  const container = refresh.registerRefreshContainer(root, (promise) => {
    statusPromise = promise;
  });
  const registrations = [];
  function register(element, handler) {
    const handle = refresh.registerRefreshHandler(element, handler);
    registrations.push(handle);
    return handle;
  }
  // Use the captured preorder registration shape, including the root handler.
  register(root, async () => {
    calls.push("root");
    await parentGate;
    parentResolved = true;
    return true;
  });
  register(a, async () => { calls.push("A"); return allowA; });
  register(a1, async () => { calls.push("A1"); return true; });
  register(b, async function() {
    check(this === undefined, "registered callback retains its plain-call undefined receiver");
    calls.push("B");
    return true;
  });
  const leafHandle = register(b1, async () => { calls.push("B1"); return true; });
  register(outside, async () => { calls.push("outside"); return true; });
  async function completion(expectedStatus = 1) {
    check(statusPromise instanceof Promise, "public container receives the tree status Promise");
    const result = await settleWithin(statusPromise, 1000);
    check(result.settled && !("error" in result), "tree status Promise fulfills");
    check(result.value === expectedStatus, "tree completion preserves the captured numeric status");
  }

  root.dispatchEvent(new refresh.RefreshEvent());
  await nextTurn();
  check(JSON.stringify(calls) === JSON.stringify(["root"]),
    "children wait for the pending parent handler: " + JSON.stringify(calls));
  check(parentResolved === false, "deferred parent is still unresolved");
  releaseParent();
  await completion(2);
  check(parentResolved === true, "parent resolves before the completed tree");
  check(JSON.stringify(calls) === JSON.stringify(["root", "B", "A", "B1"]),
    "false prunes its subtree while its sibling continues in registration order: " + JSON.stringify(calls));
  check(!calls.includes("A1"), "false handler prevents invocation of its descendant");
  check(!calls.includes("outside"), "container excludes handlers outside its tree");

  calls = [];
  allowA = true;
  parentGate = Promise.resolve();
  root.dispatchEvent(new refresh.RefreshEvent());
  await completion();
  check(JSON.stringify(calls) === JSON.stringify(["root", "B", "A", "B1", "A1"]),
    "true visits ready sibling branches in captured registration order: " + JSON.stringify(calls));

  refresh.unregisterRefreshHandler(leafHandle);
  calls = [];
  root.dispatchEvent(new refresh.RefreshEvent());
  await completion();
  check(JSON.stringify(calls) === JSON.stringify(["root", "B", "A", "A1"]),
    "opaque leaf handle unregister removes only its handler: " + JSON.stringify(calls));
  for (const handle of registrations) refresh.unregisterRefreshHandler(handle);
  refresh.unregisterRefreshContainer(container);

  // Unregistered DOM intermediaries do not create refresh-tree levels.
  const secondRoot = new TreeElement("secondRoot");
  const intermediary = new TreeElement("unregistered", secondRoot);
  const early = new TreeElement("early", intermediary);
  const peer = new TreeElement("peer", secondRoot);
  const grandchild = new TreeElement("grandchild", early);
  calls = [];
  refresh.registerRefreshContainer(secondRoot, (promise) => { statusPromise = promise; });
  for (const element of [grandchild, peer, early, secondRoot]) {
    refresh.registerRefreshHandler(element, async () => {
      calls.push(element.name);
      return true;
    });
  }
  secondRoot.dispatchEvent(new refresh.RefreshEvent());
  await completion();
  check(JSON.stringify(calls) === JSON.stringify(["secondRoot", "peer", "early", "grandchild"]),
    "later ancestor registration adopts its descendants in captured insertion order: " + JSON.stringify(calls));
} catch (error) {
  harnessError = error;
}
if (assertionCount === 0) {
  check(false, "no tree behavior assertions were reached");
  harnessError = harnessError || new Error("fixture did not reach behavior assertions");
}
const terminalStatus = harnessError ? "ERROR" : failures.length === 0 ? "PASS" : "FAIL";
console.log("LWC_REFRESH_ASSERTIONS=" + assertionCount + " STATUS=" + terminalStatus);
if (harnessError) console.error("HARNESS_ERROR=" + String(harnessError));
if (failures.length > 0) console.error(failures.join("\n"));
if (terminalStatus !== "PASS") process.exitCode = 1;
`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--input-type=module", "-e", script)
	output, err := cmd.CombinedOutput()
	match := refreshModuleTerminalStatus.FindSubmatch(output)
	if len(match) != 3 {
		t.Fatalf("required Node execution emitted no assertion count and terminal status (err=%v):\n%s", err, output)
	}
	assertionCount, parseErr := strconv.Atoi(string(match[1]))
	if parseErr != nil || assertionCount == 0 {
		t.Fatalf("required Node execution reported an empty/invalid assertion count: %q\n%s", match[1], output)
	}
	if string(match[2]) == "ERROR" {
		t.Fatalf("RefreshModuleJS tree fixture failed before a conclusive behavior result after %d checks (err=%v):\n%s", assertionCount, err, output)
	}
	if string(match[2]) != "PASS" || err != nil {
		t.Fatalf("RefreshModuleJS tree behavior failed after %d assertions (err=%v):\n%s", assertionCount, err, output)
	}
	t.Logf("RefreshModuleJS tree executed %d behavior assertions: %s", assertionCount, match[2])
}

// TestRefreshModuleNumericRegistrationHandlesAPI67 checks the documented
// numeric register/unregister handle facet through the real module. Official
// registerRefreshHandler/registerRefreshContainer and their unregister guides
// specify numeric handles. Values, freshness, cross-registry inequality and
// mid-refresh child retention are not qualified by this local LWS fixture.
// L16 r_handles_{handler,container}_numeric back its numeric completion edit.
func TestRefreshModuleNumericRegistrationHandlesAPI67(t *testing.T) {
	moduleJSON, err := json.Marshal(RefreshModuleJS())
	if err != nil {
		t.Fatalf("encode RefreshModuleJS source: %v", err)
	}
	script := `const moduleSource = ` + string(moduleJSON) + `;
const failures = [];
let assertionCount = 0;
let harnessError = null;
function check(condition, message) {
  assertionCount++;
  if (!condition) failures.push(message);
}
const nextTurn = () => new Promise((resolve) => setImmediate(resolve));
function settleWithin(promise, milliseconds) {
  return new Promise((resolve) => {
    const timer = setTimeout(() => resolve({ settled: false }), milliseconds);
    Promise.resolve(promise).then(
      (value) => { clearTimeout(timer); resolve({ settled: true, value }); },
      (error) => { clearTimeout(timer); resolve({ settled: true, error }); }
    );
  });
}
try {
  if (typeof EventTarget !== "function" || typeof CustomEvent !== "function") {
    throw new Error("Node built-in EventTarget or CustomEvent is unavailable");
  }
  const refresh = await import(
    "data:text/javascript;base64," + Buffer.from(moduleSource).toString("base64")
  );
  const a = new EventTarget();
  const b = new EventTarget();
  let handlerCallsA = 0;
  let handlerCallsB = 0;
  let containerCallsA = 0;
  let containerCallsB = 0;
  const statuses = new Map();
  const handlerA = refresh.registerRefreshHandler(a, async () => {
    handlerCallsA++;
    return true;
  });
  const handlerB = refresh.registerRefreshHandler(b, async () => {
    handlerCallsB++;
    return true;
  });
  const containerA = refresh.registerRefreshContainer(a, (promise) => {
    containerCallsA++;
    statuses.set(a, promise);
  });
  const containerB = refresh.registerRefreshContainer(b, (promise) => {
    containerCallsB++;
    statuses.set(b, promise);
  });
  check(typeof handlerA === "number", "handler A returns a numeric handle");
  check(typeof handlerB === "number", "handler B returns a numeric handle");
  check(typeof containerA === "number", "container A returns a numeric handle");
  check(typeof containerB === "number", "container B returns a numeric handle");
  check(handlerA !== handlerB, "distinct handler registry nodes have distinguishable handles");
  check(containerA !== containerB, "distinct container registry nodes have distinguishable handles");

  async function dispatch(target, expectCompletion = true) {
    statuses.delete(target);
    target.dispatchEvent(new refresh.RefreshEvent());
    await nextTurn();
    if (expectCompletion) {
      const promise = statuses.get(target);
      const completion = await settleWithin(promise, 1000);
      check(promise instanceof Promise && completion.settled &&
        !("error" in completion) && completion.value === 1,
        "numeric-handle dispatch preserves the public Promise and captured numeric status");
    }
  }
  await dispatch(a);
  check(handlerCallsA === 1 && containerCallsA === 1,
    "same-element handler and container registrations both participate");
  check(handlerCallsB === 0 && containerCallsB === 0,
    "dispatch excludes the other independently registered element");
  await dispatch(b);
  check(handlerCallsB === 1 && containerCallsB === 1,
    "second node remains independently dispatchable");

  refresh.unregisterRefreshHandler(handlerA);
  await dispatch(a);
  check(handlerCallsA === 1 && containerCallsA === 2,
    "exact numeric handler return removes only the same-element handler");
  check(handlerCallsB === 1 && containerCallsB === 1,
    "handler unregister preserves the other node's registrations");
  refresh.unregisterRefreshContainer(containerA);
  await dispatch(a, false);
  check(containerCallsA === 2 && handlerCallsA === 1,
    "exact numeric container return removes its refresh listener");
  await dispatch(b);
  check(handlerCallsB === 2 && containerCallsB === 2,
    "other node survives unregister of the first handler and container");
  refresh.unregisterRefreshContainer(containerB);
  await dispatch(b, false);
  check(containerCallsB === 2 && handlerCallsB === 2,
    "container unregister leaves its handler idle without a refresh consumer");
  refresh.unregisterRefreshHandler(handlerB);

  // Preserve the existing element and element-wrapper unregister aliases.
  const aliasTarget = new EventTarget();
  let aliasHandlerCalls = 0;
  let aliasContainerCalls = 0;
  refresh.registerRefreshHandler(aliasTarget, async () => {
    aliasHandlerCalls++;
    return true;
  });
  refresh.registerRefreshContainer(aliasTarget, (promise) => {
    aliasContainerCalls++;
    statuses.set(aliasTarget, promise);
  });
  refresh.unregisterRefreshHandler({ element: aliasTarget });
  await dispatch(aliasTarget);
  check(aliasHandlerCalls === 0 && aliasContainerCalls === 1,
    "legacy element-wrapper handler unregister remains supported");
  refresh.unregisterRefreshContainer(aliasTarget);
  await dispatch(aliasTarget, false);
  check(aliasContainerCalls === 1 && aliasHandlerCalls === 0,
    "legacy element container unregister remains supported");
} catch (error) {
  harnessError = error;
}
if (assertionCount === 0) {
  check(false, "no numeric-handle behavior assertions were reached");
  harnessError = harnessError || new Error("fixture did not reach behavior assertions");
}
const terminalStatus = harnessError ? "ERROR" : failures.length === 0 ? "PASS" : "FAIL";
console.log("LWC_REFRESH_ASSERTIONS=" + assertionCount + " STATUS=" + terminalStatus);
if (harnessError) console.error("HARNESS_ERROR=" + String(harnessError));
if (failures.length > 0) console.error(failures.join("\n"));
if (terminalStatus !== "PASS") process.exitCode = 1;
`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--input-type=module", "-e", script)
	output, err := cmd.CombinedOutput()
	match := refreshModuleTerminalStatus.FindSubmatch(output)
	if len(match) != 3 {
		t.Fatalf("required Node execution emitted no assertion count and terminal status (err=%v):\n%s", err, output)
	}
	assertionCount, parseErr := strconv.Atoi(string(match[1]))
	if parseErr != nil || assertionCount == 0 {
		t.Fatalf("required Node execution reported an empty/invalid assertion count: %q\n%s", match[1], output)
	}
	if string(match[2]) == "ERROR" {
		t.Fatalf("RefreshModuleJS handle fixture failed before a conclusive behavior result after %d checks (err=%v):\n%s", assertionCount, err, output)
	}
	if string(match[2]) != "PASS" || err != nil {
		t.Fatalf("RefreshModuleJS numeric-handle behavior failed after %d assertions (err=%v):\n%s", assertionCount, err, output)
	}
	t.Logf("RefreshModuleJS numeric handles executed %d behavior assertions: %s", assertionCount, match[2])
}

// TestRefreshModuleUnregisterMidRefreshRetainsChildrenAPI67 checks ongoing
// child participation under the official handler/container unregister clauses:
// https://developer.salesforce.com/docs/platform/lwc/guide/reference-lightning-refreshview-unregisterrefreshhandler.html
// https://developer.salesforce.com/docs/platform/lwc/guide/reference-lightning-refreshview-unregisterrefreshcontainer.html
// API67 names the public contract target. This plain-DOM Node fixture does not
// establish artifact compilation, browser/native behavior or API interval parity.
// Only child participation is asserted, without exact callback counts, context
// identity/lifetime, new registrations or subsequent-refresh guarantees.
func TestRefreshModuleUnregisterMidRefreshRetainsChildrenAPI67(t *testing.T) {
	moduleJSON, err := json.Marshal(RefreshModuleJS())
	if err != nil {
		t.Fatalf("encode RefreshModuleJS source: %v", err)
	}
	script := `const moduleSource = ` + string(moduleJSON) + `;
const failures = [];
let assertionCount = 0;
let harnessError = null;
function check(condition, message) {
  assertionCount++;
  if (!condition) failures.push(message);
}
function settleWithin(promise, milliseconds) {
  return new Promise((resolve) => {
    const timer = setTimeout(() => resolve({ settled: false }), milliseconds);
    Promise.resolve(promise).then(
      (value) => { clearTimeout(timer); resolve({ settled: true, value }); },
      (error) => { clearTimeout(timer); resolve({ settled: true, error }); }
    );
  });
}

try {
  if (typeof EventTarget !== "function" || typeof CustomEvent !== "function") {
    throw new Error("Node built-in EventTarget or CustomEvent is unavailable");
  }
  class TreeElement extends EventTarget {
    constructor(name, parent = null) {
      super();
      this.name = name;
      this.parentElement = parent;
      this.children = [];
      if (parent) parent.children.push(this);
    }
    contains(element) {
      for (let current = element; current; current = current.parentElement) {
        if (current === this) return true;
      }
      return false;
    }
    compareDocumentPosition(other) {
      if (other === this) return 0;
      let root = this;
      while (root.parentElement) root = root.parentElement;
      if (!root.contains(other)) return 1;
      const order = [];
      function visit(element) {
        order.push(element);
        for (const child of element.children) visit(child);
      }
      visit(root);
      return order.indexOf(this) < order.indexOf(other) ? 4 : 2;
    }
  }
  const refresh = await import(
    "data:text/javascript;base64," + Buffer.from(moduleSource).toString("base64")
  );
  async function runRetentionCase(name, unregister) {
    const root = new TreeElement(name + ":R");
    const parent = new TreeElement(name + ":P", root);
    const child = new TreeElement(name + ":C", parent);
    let markParentStarted;
    const parentStarted = new Promise((resolve) => { markParentStarted = resolve; });
    let releaseParent;
    const parentGate = new Promise((resolve) => { releaseParent = resolve; });
    let childCalls = 0;
    let statusPromise;
    let caughtStatus;
    let containerHandle = null;
    let parentHandle = null;
    let childHandle = null;
    try {
      containerHandle = refresh.registerRefreshContainer(root, (promise) => {
        statusPromise = promise;
        // Attach rejection handling immediately to the captured ongoing refresh.
        caughtStatus = Promise.resolve(promise).then(
          (value) => ({ value }),
          (error) => ({ error })
        );
      });
      parentHandle = refresh.registerRefreshHandler(parent, async () => {
        markParentStarted();
        return await parentGate;
      });
      childHandle = refresh.registerRefreshHandler(child, async () => {
        childCalls++;
        return true;
      });
      root.dispatchEvent(new refresh.RefreshEvent());
      if (!(statusPromise instanceof Promise) || !caughtStatus) {
        throw new Error(name + ": container did not supply the ongoing status Promise");
      }
      const started = await settleWithin(parentStarted, 1000);
      if (!started.settled || "error" in started) {
        throw new Error(name + ": parent-started barrier did not complete");
      }
      if (unregister === "handler") {
        refresh.unregisterRefreshHandler(parentHandle);
        parentHandle = null;
      } else if (unregister === "container") {
        refresh.unregisterRefreshContainer(containerHandle);
        containerHandle = null;
      }
      releaseParent(true);
      const completion = await settleWithin(caughtStatus, 1000);
      if (!completion.settled || "error" in completion) {
        throw new Error(name + ": ongoing refresh did not settle through the caught status");
      }
      if ("error" in completion.value) {
        throw new Error(name + ": ongoing refresh rejected: " + String(completion.value.error));
      }
      check(childCalls > 0, name + ": child participates in the ongoing refresh");
    } finally {
      releaseParent(true);
      if (childHandle !== null) refresh.unregisterRefreshHandler(childHandle);
      if (parentHandle !== null) refresh.unregisterRefreshHandler(parentHandle);
      if (containerHandle !== null) refresh.unregisterRefreshContainer(containerHandle);
    }
  }
  await runRetentionCase("handler-unregister", "handler");
  await runRetentionCase("handler-control", null);
  await runRetentionCase("container-unregister", "container");
  await runRetentionCase("container-control", null);
} catch (error) {
  harnessError = error;
}
if (assertionCount === 0) {
  check(false, "no mid-refresh child-participation assertions were reached");
  harnessError = harnessError || new Error("fixture did not reach behavior assertions");
}
const terminalStatus = harnessError ? "ERROR" : failures.length === 0 ? "PASS" : "FAIL";
console.log("LWC_REFRESH_ASSERTIONS=" + assertionCount + " STATUS=" + terminalStatus);
if (harnessError) console.error("HARNESS_ERROR=" + String(harnessError));
if (failures.length > 0) console.error(failures.join("\n"));
if (terminalStatus !== "PASS") process.exitCode = 1;
`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--input-type=module", "-e", script)
	output, err := cmd.CombinedOutput()
	match := refreshModuleTerminalStatus.FindSubmatch(output)
	if len(match) != 3 {
		t.Fatalf("required Node execution emitted no assertion count and terminal status (err=%v):\n%s", err, output)
	}
	assertionCount, parseErr := strconv.Atoi(string(match[1]))
	if parseErr != nil || assertionCount != 4 {
		t.Fatalf("required Node execution did not report all four child-participation assertions: %q\n%s", match[1], output)
	}
	if string(match[2]) == "ERROR" {
		t.Fatalf("RefreshModuleJS mid-refresh fixture failed before a conclusive behavior result after %d checks (err=%v):\n%s", assertionCount, err, output)
	}
	if string(match[2]) != "PASS" || err != nil {
		t.Fatalf("RefreshModuleJS mid-refresh child participation failed after %d assertions (err=%v):\n%s", assertionCount, err, output)
	}
	t.Logf("RefreshModuleJS mid-refresh executed %d child-participation assertions: %s", assertionCount, match[2])
}
