const assert = require("assert");
const Module = require("module");

let messageHandler;
let disposeHandler;
const panel = {
  htmlWrites: 0,
  revealCount: 0,
  disposeCount: 0,
  webview: {
    cspSource: "vscode-resource:",
    set html(value) {
      this.latestHtml = value;
      panel.htmlWrites += 1;
    },
    onDidReceiveMessage(handler) {
      messageHandler = handler;
      return { dispose() {} };
    },
  },
  onDidDispose(handler) {
    disposeHandler = handler;
    return { dispose() {} };
  },
  reveal() {
    this.revealCount += 1;
  },
  dispose() {
    this.disposeCount += 1;
  },
};

const originalLoad = Module._load;
Module._load = function patchedLoad(request, parent, isMain) {
  if (request === "vscode") {
    return {
      ViewColumn: { One: 1 },
      window: {
        createWebviewPanel() {
          return panel;
        },
      },
    };
  }
  return originalLoad.call(this, request, parent, isMain);
};

const { GladeHomeController } = require("../out/hub/controller");

const executed = [];
const controller = new GladeHomeController({ subscriptions: [] }, {
  snapshot: () => ({ changedSince: "origin/main" }),
  executeCommand(command) {
    executed.push(command);
    return Promise.resolve();
  },
});

(async () => {
  controller.open();
  assert.strictEqual(panel.htmlWrites, 1);
  assert(panel.webview.latestHtml.includes("Glade Home"));

  await messageHandler({ type: "ready" });
  assert.strictEqual(panel.htmlWrites, 1);

  await messageHandler({ type: "selectTab", tab: "state" });
  assert.strictEqual(panel.htmlWrites, 1);
  controller.update();
  assert(panel.webview.latestHtml.includes('data-tab="state" aria-selected="true"'));
  assert(panel.webview.latestHtml.includes('data-panel="state">'));

  await messageHandler({ type: "runCommand", command: "glade.runLocalProof" });
  assert.deepStrictEqual(executed, ["glade.runLocalProof"]);
  assert.strictEqual(panel.htmlWrites, 3);

  await messageHandler({ type: "runCommand", command: "workbench.action.files.delete" });
  assert.deepStrictEqual(executed, ["glade.runLocalProof"]);

  disposeHandler();
  controller.open();
  assert(panel.webview.latestHtml.includes('data-tab="home" aria-selected="true"'));
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
