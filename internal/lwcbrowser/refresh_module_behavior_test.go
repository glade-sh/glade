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
// full refresh traversal.
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

  refresh.registerRefreshContainer(target, (promise) => {
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
  check(refresh.RefreshComplete !== undefined, "RefreshComplete is exported");
  check(unrelatedListenerCalls === 1, "other same-element listeners still receive the event");
  if (statusPromise instanceof Promise) {
    const completion = await settleWithin(statusPromise, 1000);
    check(completion.settled, "container status Promise settles");
    if (completion.settled) {
      check(!("error" in completion), "container status Promise fulfills");
      if (!("error" in completion)) {
        check(completion.value === refresh.RefreshComplete,
          "status Promise resolves to the exported RefreshComplete constant");
      }
    }
  }

  let replacementCalls = 0;
  let replacementPromise;
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
    check(completion.value === refresh.RefreshComplete, "replacement resolves to exported completion identity");
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
	cmd := exec.CommandContext(ctx, "/usr/local/bin/node", "--input-type=module", "-e", script)
	output, err := cmd.CombinedOutput()

	match := refreshModuleTerminalStatus.FindSubmatch(output)
	if len(match) != 3 {
		t.Fatalf("required /usr/local/bin/node execution emitted no assertion count and terminal status (err=%v):\n%s", err, output)
	}
	assertionCount, parseErr := strconv.Atoi(string(match[1]))
	if parseErr != nil || assertionCount == 0 {
		t.Fatalf("required /usr/local/bin/node execution reported an empty/invalid assertion count: %q\n%s", match[1], output)
	}
	if string(match[2]) == "ERROR" {
		t.Fatalf("RefreshModuleJS fixture failed before a conclusive behavior result after %d checks (err=%v):\n%s", assertionCount, err, output)
	}
	if string(match[2]) != "PASS" || err != nil {
		t.Fatalf("RefreshModuleJS behavior failed after %d assertions (err=%v):\n%s", assertionCount, err, output)
	}
	t.Logf("RefreshModuleJS executed %d behavior assertions: %s", assertionCount, match[2])
}
